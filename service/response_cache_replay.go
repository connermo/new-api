package service

import (
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
)

// ServeResponseCacheHit 用缓存条目直接应答，不再选择渠道、不再请求上游。
//
// 返回的 error 只说明回放是否写完整（例如客户端中途断开），调用方不得据此
// 回退到上游：此时字节已经写出去了，再叠一份上游响应只会让客户端收到两段
// 互相矛盾的内容。无论写出成败都会完成结算，避免预扣费悬空。
func ServeResponseCacheHit(c *gin.Context, relayInfo *relaycommon.RelayInfo, cached *CachedResponse) error {
	if cached == nil {
		return fmt.Errorf("nil cached response")
	}
	setting := operation_setting.GetResponseCacheSetting()

	// 命中路径从不选择渠道，ChannelMeta 因此为 nil，而下游的结算与日志链路
	// 假定它存在（例如 GenerateTextOtherInfo 会读 IsModelMapped）。补一个空实例，
	// 渠道 ID 保持 0——这正是“未使用任何渠道”的语义。
	if !relayInfo.HasChannelMeta() {
		relayInfo.ChannelMeta = &relaycommon.ChannelMeta{
			UpstreamModelName: relayInfo.OriginModelName,
		}
	}

	relayInfo.SetFirstResponseTime()
	c.Writer.Header().Set(ResponseCacheStatusHeader, "exact")

	usage := &dto.Usage{
		PromptTokens:     cached.PromptTokens,
		CompletionTokens: cached.CompletionTokens,
		TotalTokens:      cached.TotalTokens,
	}

	var writeErr error
	if relayInfo.IsStream {
		writeErr = writeCachedStreamResponse(c, relayInfo, cached, usage, setting)
	} else {
		writeCachedJSONResponse(c, relayInfo, cached, usage)
	}

	common.SetContextKey(c, constant.ContextKeyResponseCacheInfo, map[string]any{
		"mode":  "exact",
		"age_s": common.GetTimestamp() - cached.CreatedAt,
	})
	settleResponseCacheHit(c, relayInfo, usage, setting)
	return writeErr
}

func writeCachedJSONResponse(c *gin.Context, relayInfo *relaycommon.RelayInfo, cached *CachedResponse, usage *dto.Usage) {
	message := dto.Message{Role: "assistant"}
	message.SetStringContent(cached.Content)
	if cached.ReasoningContent != "" {
		message.ReasoningContent = &cached.ReasoningContent
	}

	c.JSON(http.StatusOK, dto.OpenAITextResponse{
		Id:      cachedResponseID(c),
		Model:   relayInfo.OriginModelName,
		Object:  "chat.completion",
		Created: common.GetTimestamp(),
		Choices: []dto.OpenAITextResponseChoice{{
			Index:        0,
			Message:      message,
			FinishReason: cached.FinishReason,
		}},
		Usage: *usage,
	})
}

func writeCachedStreamResponse(
	c *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	cached *CachedResponse,
	usage *dto.Usage,
	setting *operation_setting.ResponseCacheSetting,
) error {
	id := cachedResponseID(c)
	created := common.GetTimestamp()
	modelName := relayInfo.OriginModelName

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	newChunk := func() dto.ChatCompletionsStreamResponse {
		return dto.ChatCompletionsStreamResponse{
			Id:      id,
			Object:  "chat.completion.chunk",
			Created: created,
			Model:   modelName,
			Choices: []dto.ChatCompletionsStreamResponseChoice{{Index: 0}},
		}
	}

	// 首个分片只带 role，与真实上游的开场一致。
	first := newChunk()
	first.Choices[0].Delta.Role = "assistant"
	first.Choices[0].Delta.Content = common.GetPointer("")
	if err := writeCachedSSEChunk(c, first); err != nil {
		return err
	}

	chunkSize := setting.EffectiveStreamReplayChunkSize()
	// 推理内容先于正文，与多数上游的分片顺序一致。
	for _, piece := range splitIntoRuneChunks(cached.ReasoningContent, chunkSize) {
		chunk := newChunk()
		chunk.Choices[0].Delta.ReasoningContent = common.GetPointer(piece)
		if err := writeCachedSSEChunk(c, chunk); err != nil {
			return err
		}
	}
	for _, piece := range splitIntoRuneChunks(cached.Content, chunkSize) {
		chunk := newChunk()
		chunk.Choices[0].Delta.Content = common.GetPointer(piece)
		if err := writeCachedSSEChunk(c, chunk); err != nil {
			return err
		}
	}

	finishReason := cached.FinishReason
	if finishReason == "" {
		finishReason = "stop"
	}
	stop := newChunk()
	stop.Choices[0].FinishReason = &finishReason
	if err := writeCachedSSEChunk(c, stop); err != nil {
		return err
	}

	// usage 依据本次请求的 stream_options 决定，而不是缓存写入时的那次请求。
	if shouldIncludeUsageInStream(relayInfo) {
		final := newChunk()
		final.Choices = []dto.ChatCompletionsStreamResponseChoice{}
		final.Usage = usage
		if err := writeCachedSSEChunk(c, final); err != nil {
			return err
		}
	}

	return writeCachedSSERaw(c, "[DONE]")
}

func writeCachedSSEChunk(c *gin.Context, chunk dto.ChatCompletionsStreamResponse) error {
	payload, err := common.Marshal(chunk)
	if err != nil {
		return fmt.Errorf("marshal cached stream chunk: %w", err)
	}
	return writeCachedSSERaw(c, string(payload))
}

func writeCachedSSERaw(c *gin.Context, data string) error {
	if c.Request != nil && c.Request.Context().Err() != nil {
		return c.Request.Context().Err()
	}
	c.Render(-1, common.CustomEvent{Data: "data: " + data})
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
	return nil
}

// splitIntoRuneChunks 按字符而非字节切分，避免把多字节字符劈成两个分片。
func splitIntoRuneChunks(s string, size int) []string {
	if s == "" {
		return nil
	}
	runes := []rune(s)
	chunks := make([]string, 0, len(runes)/size+1)
	for start := 0; start < len(runes); start += size {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func shouldIncludeUsageInStream(relayInfo *relaycommon.RelayInfo) bool {
	request, ok := relayInfo.Request.(*dto.GeneralOpenAIRequest)
	if !ok || request.StreamOptions == nil {
		return false
	}
	return request.StreamOptions.IncludeUsage
}

func cachedResponseID(c *gin.Context) string {
	return "chatcmpl-" + c.GetString(common.RequestIdKey)
}

// settleResponseCacheHit 结算命中请求。
//
// free：全额退还预扣费并记一条 0 额度日志——适用于运营者自己就是 token
// 付费方的自用网关。
// ratio：按原价的固定比例计费，走与普通请求完全相同的结算与日志链路——
// 转售场景必须用这个，否则命中等于白送一次请求。
func settleResponseCacheHit(
	c *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	usage *dto.Usage,
	setting *operation_setting.ResponseCacheSetting,
) {
	if setting.ChargeOnHit() {
		relayInfo.PriceData.AddOtherRatio("response_cache", setting.EffectiveHitBillingRatio())
		PostTextConsumeQuota(c, relayInfo, usage, []string{"缓存命中"})
		return
	}

	if err := SettleBilling(c, relayInfo, 0); err != nil {
		logger.LogError(c, "response cache: settle failed: "+err.Error())
	}
	// 额度为 0，但请求次数仍要计入。
	model.UpdateUserUsedQuotaAndRequestCount(relayInfo.UserId, 0)

	other := GenerateTextOtherInfo(c, relayInfo,
		relayInfo.PriceData.ModelRatio,
		relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		relayInfo.PriceData.CompletionRatio,
		0, relayInfo.PriceData.CacheRatio,
		relayInfo.PriceData.ModelPrice,
		relayInfo.PriceData.GroupRatioInfo.GroupSpecialRatio)

	model.RecordConsumeLog(c, relayInfo.UserId, model.RecordConsumeLogParams{
		ChannelId:        0,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		ModelName:        relayInfo.OriginModelName,
		TokenName:        c.GetString("token_name"),
		Quota:            0,
		Content:          "网关缓存命中，未请求上游",
		TokenId:          relayInfo.TokenId,
		UseTimeSeconds:   int(time.Since(relayInfo.StartTime).Seconds()),
		IsStream:         relayInfo.IsStream,
		Group:            relayInfo.UsingGroup,
		Other:            other,
	})
}
