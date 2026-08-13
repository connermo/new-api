package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/pkg/cachex"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/samber/hot"
	"github.com/tidwall/sjson"
)

const (
	responseCacheNamespace = "new-api:response_cache:v1"

	// ResponseCacheSkipHeader 客户端级绕过开关。
	ResponseCacheSkipHeader = "X-New-API-No-Cache"
	// ResponseCacheStatusHeader 命中时回写的标记头，便于调用方与排障区分来源。
	// 只放在响应头里，不注入 JSON body，避免破坏 SDK 的结构校验。
	ResponseCacheStatusHeader = "X-New-API-Cache"
)

// responseCacheVolatileFields 是从缓存键里剔除的字段：它们不影响模型生成的内容，
// 保留会让本可复用的请求变成 miss。
//
// 剔除采用白名单式的显式删除，其余字段——包括将来新增的、本项目还不认识的
// 厂商私有参数——一律参与哈希。这是刻意选择的失败方向：未知字段只会造成
// miss，永远不会造成错误命中。
var responseCacheVolatileFields = []string{
	"stream",                 // 回放时按当前请求的模式渲染，与缓存内容无关
	"stream_options",         // 同上，仅影响 usage 是否随流返回
	"user",                   // 调用方身份标签
	"safety_identifier",      // 同上
	"metadata",               // 调用方自定义标签
	"store",                  // 上游是否留存，不影响本次输出
	"prompt_cache_key",       // 上游 prompt cache 路由提示
	"prompt_cache_retention", // 同上
}

// CachedResponse 是格式中立的归一化响应。
//
// 存归一化结果而不是上游原始报文，是为了让同一条目既能回放成非流式 JSON，
// 也能回放成 SSE，并且 usage 是结构化的、可以直接进计费。
type CachedResponse struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	FinishReason     string `json:"finish_reason,omitempty"`
	UpstreamModel    string `json:"upstream_model,omitempty"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	TotalTokens      int    `json:"total_tokens"`
	CreatedAt        int64  `json:"created_at"`
}

// ResponseCacheContext 承载单次请求的缓存状态，由 PrepareResponseCache 创建。
// 为 nil 表示本次请求不参与缓存。
type ResponseCacheContext struct {
	key     string
	model   string
	capture *responseCaptureWriter
	setting *operation_setting.ResponseCacheSetting
}

type responseCacheCounters struct {
	hits   atomic.Int64
	misses atomic.Int64
	stores atomic.Int64
}

var (
	responseCacheOnce  sync.Once
	responseCacheStore *cachex.HybridCache[CachedResponse]
	responseCacheStats responseCacheCounters
)

func getResponseCache() *cachex.HybridCache[CachedResponse] {
	responseCacheOnce.Do(func() {
		setting := operation_setting.GetResponseCacheSetting()
		capacity := setting.EffectiveMaxEntries()
		ttl := time.Duration(setting.EffectiveTTLSeconds()) * time.Second

		responseCacheStore = cachex.NewHybridCache[CachedResponse](cachex.HybridCacheConfig[CachedResponse]{
			Namespace: cachex.Namespace(responseCacheNamespace),
			Redis:     common.RDB,
			RedisEnabled: func() bool {
				return common.RedisEnabled && common.RDB != nil
			},
			RedisCodec: cachex.JSONCodec[CachedResponse]{},
			Memory: func() *hot.HotCache[string, CachedResponse] {
				return hot.NewHotCache[string, CachedResponse](hot.LRU, capacity).
					WithTTL(ttl).
					WithJanitor().
					Build()
			},
		})
	})
	return responseCacheStore
}

// PrepareResponseCache 判断本次请求是否参与缓存并构造缓存键。
// 返回 nil 表示不参与——调用方据此跳过后续所有缓存逻辑。
func PrepareResponseCache(c *gin.Context, relayInfo *relaycommon.RelayInfo) *ResponseCacheContext {
	setting := operation_setting.GetResponseCacheSetting()
	if setting == nil || !setting.Enabled {
		return nil
	}
	if !responseCacheFormatSupported(relayInfo) {
		return nil
	}
	if isTruthyHeaderValue(c.GetHeader(ResponseCacheSkipHeader)) {
		return nil
	}
	if !setting.ModelEnabled(relayInfo.OriginModelName) {
		return nil
	}

	request, ok := relayInfo.Request.(*dto.GeneralOpenAIRequest)
	if !ok || !responseCacheRequestEligible(request, setting) {
		return nil
	}

	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil
	}
	if storage.Size() > int64(setting.EffectiveMaxRequestBytes()) {
		return nil
	}
	body, err := storage.Bytes()
	if err != nil {
		return nil
	}

	key, err := buildResponseCacheKey(relayInfo, setting, body)
	if err != nil {
		logger.LogDebug(c, "response cache: build key failed: "+err.Error())
		return nil
	}

	return &ResponseCacheContext{
		key:     key,
		model:   relayInfo.OriginModelName,
		setting: setting,
	}
}

// responseCacheFormatSupported 限定一期只覆盖 OpenAI Chat Completions。
// 其余格式（Claude Messages / Gemini / Responses）的回放渲染尚未实现，
// 在准入处直接挡掉，而不是让它们走到渲染层再失败。
func responseCacheFormatSupported(relayInfo *relaycommon.RelayInfo) bool {
	if relayInfo == nil {
		return false
	}
	if relayInfo.RelayFormat != types.RelayFormatOpenAI {
		return false
	}
	return relayInfo.RelayMode == relayconstant.RelayModeChatCompletions
}

// responseCacheRequestEligible 是准入门槛。误命中大多不是阈值问题而是这里没拦住，
// 所以每一条都从“出错时宁可不缓存”的方向取默认值。
func responseCacheRequestEligible(request *dto.GeneralOpenAIRequest, setting *operation_setting.ResponseCacheSetting) bool {
	if request == nil || len(request.Messages) == 0 {
		return false
	}
	// n > 1 时响应形态无法用单条内容复用。
	if request.N != nil && *request.N != 1 {
		return false
	}
	// logprobs 的取值依赖具体采样过程，回放出来的是假的。
	if request.LogProbs != nil && *request.LogProbs {
		return false
	}
	if request.TopLogProbs != nil {
		return false
	}
	// 调用方主动要多样性时，命中缓存是 bug 而不是优化。
	if request.Temperature != nil && *request.Temperature > setting.MaxTemperature {
		return false
	}
	if len(request.Tools) > 0 && !setting.CacheToolRequests {
		return false
	}
	if len(request.Functions) > 0 {
		return false
	}

	for i := range request.Messages {
		message := &request.Messages[i]
		// tool 结果与 assistant 工具调用说明当前处在 agent 循环中途，
		// 复用历史 tool_call_id 会直接让客户端状态错乱。
		if message.Role == "tool" || message.Role == "function" {
			return false
		}
		if len(message.ToolCalls) > 0 {
			return false
		}
		if message.IsStringContent() {
			continue
		}
		// 多模态部件不参与一期：内容哈希虽然覆盖了它们，但回放层只还原文本。
		for _, part := range message.ParseContent() {
			if part.Type != dto.ContentTypeText {
				return false
			}
		}
	}
	return true
}

// buildResponseCacheKey 用请求体原始字节构造键。
//
// 这里刻意不做 JSON 归一化（不解析成 map 再重新序列化）：归一化会把数字统一
// 转成 float64，两个仅在 2^53 以上相差一位的 seed 会塌成同一个键，那就是一次
// 错误命中。字段顺序或空白差异只会降低命中率，不会返回错答案——在这个取舍上
// 只能选后者。
func buildResponseCacheKey(
	relayInfo *relaycommon.RelayInfo,
	setting *operation_setting.ResponseCacheSetting,
	body []byte,
) (string, error) {
	canonical := body
	var err error
	for _, field := range responseCacheVolatileFields {
		canonical, err = sjson.DeleteBytes(canonical, field)
		if err != nil {
			return "", fmt.Errorf("strip volatile field %s: %w", field, err)
		}
	}

	hasher := sha256.New()
	// 各维度之间用不可出现在取值里的分隔符隔开，避免拼接歧义。
	hasher.Write([]byte(responseCacheScopeID(relayInfo, setting)))
	hasher.Write([]byte{0})
	hasher.Write([]byte(string(relayInfo.RelayFormat)))
	hasher.Write([]byte{0})
	hasher.Write([]byte(strconv.Itoa(relayInfo.RelayMode)))
	hasher.Write([]byte{0})
	hasher.Write(canonical)

	// 128 位足以排除碰撞，同时让 Redis 键短一半。
	return hex.EncodeToString(hasher.Sum(nil)[:16]), nil
}

// responseCacheScopeID 决定缓存条目在谁之间共享。默认按用户隔离。
func responseCacheScopeID(relayInfo *relaycommon.RelayInfo, setting *operation_setting.ResponseCacheSetting) string {
	switch setting.NormalizedShareScope() {
	case operation_setting.ResponseCacheScopeGlobal:
		return "global"
	case operation_setting.ResponseCacheScopeGroup:
		return "g:" + relayInfo.UsingGroup
	default:
		return "u:" + strconv.Itoa(relayInfo.UserId)
	}
}

func isTruthyHeaderValue(value string) bool {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "1", "true", "on", "yes":
		return true
	default:
		return false
	}
}

// Lookup 查询缓存。未命中返回 nil。
func (ctx *ResponseCacheContext) Lookup(c *gin.Context) *CachedResponse {
	if ctx == nil {
		return nil
	}
	cached, found, err := getResponseCache().Get(ctx.key)
	if err != nil {
		// 缓存故障绝不影响主链路，退回上游即可。
		logger.LogDebug(c, "response cache: lookup failed: "+err.Error())
		responseCacheStats.misses.Add(1)
		return nil
	}
	if !found {
		responseCacheStats.misses.Add(1)
		return nil
	}
	responseCacheStats.hits.Add(1)
	return &cached
}

// Store 写入缓存条目。
//
// 不接受 gin.Context：调用方在请求结束后的 goroutine 里执行，而 gin 会把
// Context 放回对象池给下一个请求复用，此时再读它就是竞态。
func (ctx *ResponseCacheContext) Store(cached *CachedResponse) {
	if ctx == nil || cached == nil {
		return
	}
	ttl := time.Duration(ctx.setting.EffectiveTTLSeconds()) * time.Second
	if err := getResponseCache().SetWithTTL(ctx.key, *cached, ttl); err != nil {
		common.SysError("response cache: store failed: " + err.Error())
		return
	}
	responseCacheStats.stores.Add(1)
}

// ResponseCacheStats 是缓存的运行时统计。计数器为进程内累计值，重启归零。
type ResponseCacheStats struct {
	Enabled       bool    `json:"enabled"`
	ShareScope    string  `json:"share_scope"`
	EnabledModels int     `json:"enabled_models"`
	Hits          int64   `json:"hits"`
	Misses        int64   `json:"misses"`
	Stores        int64   `json:"stores"`
	HitRate       float64 `json:"hit_rate"`
	Entries       int     `json:"entries"`
	CacheAlgo     string  `json:"cache_algo"`
}

func GetResponseCacheStats() ResponseCacheStats {
	setting := operation_setting.GetResponseCacheSetting()
	hits := responseCacheStats.hits.Load()
	misses := responseCacheStats.misses.Load()

	stats := ResponseCacheStats{
		Enabled:       setting.Enabled,
		ShareScope:    setting.NormalizedShareScope(),
		EnabledModels: len(setting.EnabledModels),
		Hits:          hits,
		Misses:        misses,
		Stores:        responseCacheStats.stores.Load(),
	}
	if total := hits + misses; total > 0 {
		stats.HitRate = float64(hits) / float64(total)
	}
	if keys, err := getResponseCache().Keys(); err == nil {
		stats.Entries = len(keys)
	}
	stats.CacheAlgo, _ = getResponseCache().Algorithm()
	return stats
}

// ClearResponseCache 清空全部缓存条目。
func ClearResponseCache() error {
	return getResponseCache().Purge()
}

// ResetResponseCacheStats 重置命中率计数器。
func ResetResponseCacheStats() {
	responseCacheStats.hits.Store(0)
	responseCacheStats.misses.Store(0)
	responseCacheStats.stores.Store(0)
}
