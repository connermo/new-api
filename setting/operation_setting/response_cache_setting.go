package operation_setting

import (
	"strings"

	"github.com/QuantumNous/new-api/setting/config"
)

const (
	// ResponseCacheScopeUser 缓存条目仅在同一用户内共享（默认）。
	ResponseCacheScopeUser = "user"
	// ResponseCacheScopeGroup 缓存条目在同一分组内共享。
	ResponseCacheScopeGroup = "group"
	// ResponseCacheScopeGlobal 缓存条目全站共享。
	ResponseCacheScopeGlobal = "global"

	// ResponseCacheHitBillingFree 命中不计费，预扣费全额退还。
	ResponseCacheHitBillingFree = "free"
	// ResponseCacheHitBillingRatio 命中按原价的固定比例计费。
	ResponseCacheHitBillingRatio = "ratio"
)

// ResponseCacheSetting 网关响应缓存（精确匹配）。
//
// 命中时由网关直接返回此前记录的响应，不再选择渠道、不再请求上游。
// 精确匹配指请求体在剔除若干不影响输出的字段后逐字节相同，因此不存在语义
// 缓存那类误命中风险；任何未知字段都会参与哈希，只会导致 miss，不会导致错误命中。
type ResponseCacheSetting struct {
	Enabled bool `json:"enabled"`

	// ShareScope 决定缓存条目在谁之间共享：user / group / global。
	// 默认 user——不同用户之间永不共享，避免响应内容跨用户泄漏。
	// 放宽到 group 或 global 会显著提高命中率，但意味着用户能读到彼此的模型输出，
	// 只应在单一应用/可信团队的部署形态下开启。
	ShareScope string `json:"share_scope"`

	// EnabledModels 模型白名单。为空表示不对任何模型生效，即整个功能保持关闭。
	// 这里刻意不提供“对所有模型生效”的写法：缓存是否安全取决于模型用途，
	// 必须由管理员逐个确认。
	EnabledModels []string `json:"enabled_models"`

	// TTLSeconds 缓存条目存活时间。不提供“永不过期”，避免用户对话内容长期驻留。
	TTLSeconds int `json:"ttl_seconds"`

	// MaxEntries 仅在未启用 Redis、退化为进程内缓存时作为 LRU 容量上限。
	MaxEntries int `json:"max_entries"`

	// MaxRequestBytes 请求体超过该字节数则不参与缓存。
	MaxRequestBytes int `json:"max_request_bytes"`
	// MaxResponseBytes 响应体超过该字节数则不写入缓存。
	MaxResponseBytes int `json:"max_response_bytes"`

	// MaxTemperature 请求 temperature 高于该值时不缓存。
	// temperature 缺省的请求视为可缓存（多数上游默认 1.0，但缺省意味着调用方
	// 没有主动要求多样性，实践中重复请求期待稳定结果）。
	MaxTemperature float64 `json:"max_temperature"`

	// CacheToolRequests 是否缓存带 tools 的请求。默认关闭：工具调用通常处在
	// agent 循环中途，复用历史 tool_call_id 会让客户端状态错乱。
	CacheToolRequests bool `json:"cache_tool_requests"`

	// HitBilling 命中后的计费方式：free 或 ratio。
	// 自用网关（运营者即 token 付费方）用 free；对外转售必须用 ratio，
	// 否则命中等于白送一次请求，损失的是毛利而不是成本。
	HitBilling string `json:"hit_billing"`
	// HitBillingRatio HitBilling 为 ratio 时，按原价的该比例计费。
	HitBillingRatio float64 `json:"hit_billing_ratio"`

	// StreamReplayChunkSize 流式回放时每个 delta 携带的字符数。
	// 切成多块而非一次性推完，是因为部分客户端对超大单块 delta 处理有问题。
	StreamReplayChunkSize int `json:"stream_replay_chunk_size"`
}

var responseCacheSetting = ResponseCacheSetting{
	Enabled:               false,
	ShareScope:            ResponseCacheScopeUser,
	EnabledModels:         []string{},
	TTLSeconds:            3600,
	MaxEntries:            10_000,
	MaxRequestBytes:       256 * 1024,
	MaxResponseBytes:      256 * 1024,
	MaxTemperature:        0.2,
	CacheToolRequests:     false,
	HitBilling:            ResponseCacheHitBillingFree,
	HitBillingRatio:       0.2,
	StreamReplayChunkSize: 24,
}

func init() {
	config.GlobalConfig.Register("response_cache_setting", &responseCacheSetting)
}

func GetResponseCacheSetting() *ResponseCacheSetting {
	return &responseCacheSetting
}

// NormalizedShareScope 返回合法的共享范围，非法值一律退回最保守的 user。
func (s *ResponseCacheSetting) NormalizedShareScope() string {
	switch strings.TrimSpace(strings.ToLower(s.ShareScope)) {
	case ResponseCacheScopeGlobal:
		return ResponseCacheScopeGlobal
	case ResponseCacheScopeGroup:
		return ResponseCacheScopeGroup
	default:
		return ResponseCacheScopeUser
	}
}

// ModelEnabled 判断模型是否在白名单内。白名单为空时功能不生效。
func (s *ResponseCacheSetting) ModelEnabled(modelName string) bool {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return false
	}
	for _, m := range s.EnabledModels {
		if strings.EqualFold(strings.TrimSpace(m), modelName) {
			return true
		}
	}
	return false
}

// EffectiveTTLSeconds 返回生效的 TTL，非法值退回 1 小时。
func (s *ResponseCacheSetting) EffectiveTTLSeconds() int {
	if s.TTLSeconds <= 0 {
		return 3600
	}
	return s.TTLSeconds
}

// EffectiveMaxEntries 返回进程内缓存容量上限。
func (s *ResponseCacheSetting) EffectiveMaxEntries() int {
	if s.MaxEntries <= 0 {
		return 10_000
	}
	return s.MaxEntries
}

// EffectiveMaxRequestBytes 返回请求体大小上限。
func (s *ResponseCacheSetting) EffectiveMaxRequestBytes() int {
	if s.MaxRequestBytes <= 0 {
		return 256 * 1024
	}
	return s.MaxRequestBytes
}

// EffectiveMaxResponseBytes 返回响应体大小上限。
func (s *ResponseCacheSetting) EffectiveMaxResponseBytes() int {
	if s.MaxResponseBytes <= 0 {
		return 256 * 1024
	}
	return s.MaxResponseBytes
}

// EffectiveStreamReplayChunkSize 返回流式回放分片大小。
func (s *ResponseCacheSetting) EffectiveStreamReplayChunkSize() int {
	if s.StreamReplayChunkSize <= 0 {
		return 24
	}
	return s.StreamReplayChunkSize
}

// EffectiveHitBillingRatio 返回命中计费比例。仅在 HitBilling 为 ratio 时有意义；
// 比例必须落在 (0, 1] 内，否则退回 1（按原价计费）——宁可多收也不能因为配置
// 笔误变成白送。
func (s *ResponseCacheSetting) EffectiveHitBillingRatio() float64 {
	if s.HitBillingRatio <= 0 || s.HitBillingRatio > 1 {
		return 1
	}
	return s.HitBillingRatio
}

// ChargeOnHit 命中是否需要计费。
func (s *ResponseCacheSetting) ChargeOnHit() bool {
	return strings.TrimSpace(strings.ToLower(s.HitBilling)) == ResponseCacheHitBillingRatio
}
