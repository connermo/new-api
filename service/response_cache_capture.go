package service

import (
	"bytes"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/bytedance/gopkg/util/gopool"
	"github.com/gin-gonic/gin"
)

// responseCaptureWriter 在把响应写给客户端的同时旁路复制一份，用于事后解析并
// 写入缓存。缓冲区有上限，超出即放弃本次缓存——宁可不缓存也不能为了缓存把
// 大响应整个留在内存里。
//
// 与 middleware/audit.go 的 auditResponseWriter 同一思路，但这里需要区分
// “截断”状态：审计只要能判断成败，缓存必须拿到完整响应才有意义。
type responseCaptureWriter struct {
	gin.ResponseWriter
	body      *bytes.Buffer
	maxSize   int
	truncated bool
}

func (w *responseCaptureWriter) Write(b []byte) (int, error) {
	if !w.truncated {
		if w.body.Len()+len(b) > w.maxSize {
			w.truncated = true
			w.body.Reset()
		} else {
			w.body.Write(b)
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *responseCaptureWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// BeginCapture 安装旁路 writer。必须在任何响应写出之前调用。
func (ctx *ResponseCacheContext) BeginCapture(c *gin.Context) {
	if ctx == nil {
		return
	}
	writer := &responseCaptureWriter{
		ResponseWriter: c.Writer,
		body:           bytes.NewBuffer(nil),
		maxSize:        ctx.setting.EffectiveMaxResponseBytes(),
	}
	c.Writer = writer
	ctx.capture = writer
}

// FinishCapture 在中继结束后解析并写入缓存。解析全部异步，不阻塞响应返回。
// failed 为 true（上游报错或重试耗尽）时直接丢弃。
//
// 是否按流式解析取 relayInfo 的最终状态而非请求初始值：上游可能把非流式请求
// 的响应以 SSE 返回（见 relay/compatible_handler.go 对 Content-Type 的判断），
// 此时 relayInfo.IsStream 已被改写。
func (ctx *ResponseCacheContext) FinishCapture(relayInfo *relaycommon.RelayInfo, failed bool) {
	if ctx == nil || ctx.capture == nil {
		return
	}
	writer := ctx.capture
	ctx.capture = nil

	if failed || writer.truncated || writer.Status() != http.StatusOK {
		return
	}
	payload := writer.body.Bytes()
	if len(payload) == 0 {
		return
	}
	// 复制一份：请求结束后原缓冲区可能被复用。
	snapshot := make([]byte, len(payload))
	copy(snapshot, payload)
	isStream := relayInfo.IsStream
	upstreamModel := ctx.model

	gopool.Go(func() {
		cached := parseCachedResponse(snapshot, isStream)
		if cached == nil {
			return
		}
		cached.UpstreamModel = upstreamModel
		cached.CreatedAt = common.GetTimestamp()
		ctx.Store(cached)
	})
}

// parseCachedResponse 把客户端可见的响应还原成归一化结构。
// 无法安全复用的响应（含工具调用、内容为空、解析失败）一律返回 nil。
func parseCachedResponse(payload []byte, isStream bool) *CachedResponse {
	if isStream {
		return parseCachedStreamResponse(payload)
	}
	return parseCachedJSONResponse(payload)
}

func parseCachedJSONResponse(payload []byte) *CachedResponse {
	var response dto.OpenAITextResponse
	if err := common.Unmarshal(payload, &response); err != nil {
		return nil
	}
	if response.Error != nil || len(response.Choices) == 0 {
		return nil
	}

	choice := response.Choices[0]
	// 工具调用不进缓存：归一化结构只还原文本，存下来等于丢掉 tool_calls。
	if len(choice.Message.ToolCalls) > 0 {
		return nil
	}
	content := choice.Message.StringContent()
	if content == "" {
		return nil
	}

	cached := &CachedResponse{
		Content:          content,
		FinishReason:     choice.FinishReason,
		PromptTokens:     response.Usage.PromptTokens,
		CompletionTokens: response.Usage.CompletionTokens,
		TotalTokens:      response.Usage.TotalTokens,
	}
	if choice.Message.ReasoningContent != nil {
		cached.ReasoningContent = *choice.Message.ReasoningContent
	}
	return cached
}

func parseCachedStreamResponse(payload []byte) *CachedResponse {
	var (
		content   strings.Builder
		reasoning strings.Builder
		cached    = &CachedResponse{}
		sawChunk  bool
	)

	for _, line := range strings.Split(string(payload), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}

		var chunk dto.ChatCompletionsStreamResponse
		if err := common.Unmarshal([]byte(data), &chunk); err != nil {
			// 中途出现无法解析的分片说明流不完整，整条丢弃。
			return nil
		}
		sawChunk = true

		if chunk.Usage != nil {
			cached.PromptTokens = chunk.Usage.PromptTokens
			cached.CompletionTokens = chunk.Usage.CompletionTokens
			cached.TotalTokens = chunk.Usage.TotalTokens
		}
		for _, choice := range chunk.Choices {
			if len(choice.Delta.ToolCalls) > 0 {
				return nil
			}
			if choice.Delta.Content != nil {
				content.WriteString(*choice.Delta.Content)
			}
			if choice.Delta.ReasoningContent != nil {
				reasoning.WriteString(*choice.Delta.ReasoningContent)
			} else if choice.Delta.Reasoning != nil {
				reasoning.WriteString(*choice.Delta.Reasoning)
			}
			if choice.FinishReason != nil && *choice.FinishReason != "" {
				cached.FinishReason = *choice.FinishReason
			}
		}
	}

	if !sawChunk || content.Len() == 0 {
		return nil
	}
	// 没有收到结束标记说明流被中断，半截响应不能进缓存。
	if cached.FinishReason == "" {
		return nil
	}

	cached.Content = content.String()
	cached.ReasoningContent = reasoning.String()
	return cached
}
