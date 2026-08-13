package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCacheRelayInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatOpenAI,
		RelayMode:   relayconstant.RelayModeChatCompletions,
		UserId:      7,
		UsingGroup:  "default",
	}
}

func testCacheSetting() *operation_setting.ResponseCacheSetting {
	return &operation_setting.ResponseCacheSetting{
		ShareScope:     operation_setting.ResponseCacheScopeUser,
		MaxTemperature: 0.2,
	}
}

// 缓存键必须对“不影响生成内容”的字段免疫，同时对任何其他差异敏感。
// 前者决定命中率，后者决定不会返回错答案——后者出问题是数据事故，所以
// 这里对每一个可能被误判为无关的字段都单独取证。
func TestBuildResponseCacheKeyFieldSensitivity(t *testing.T) {
	base := `{"model":"qwen3","messages":[{"role":"user","content":"hi"}]}`

	tests := []struct {
		name      string
		body      string
		wantEqual bool
	}{
		{"identical", base, true},
		{"stream toggled", `{"model":"qwen3","messages":[{"role":"user","content":"hi"}],"stream":true}`, true},
		{"stream_options added", `{"model":"qwen3","messages":[{"role":"user","content":"hi"}],"stream_options":{"include_usage":true}}`, true},
		{"user label added", `{"model":"qwen3","messages":[{"role":"user","content":"hi"}],"user":"alice"}`, true},
		{"metadata added", `{"model":"qwen3","messages":[{"role":"user","content":"hi"}],"metadata":{"trace":"x"}}`, true},
		{"prompt_cache_key added", `{"model":"qwen3","messages":[{"role":"user","content":"hi"}],"prompt_cache_key":"k1"}`, true},

		{"different model", `{"model":"qwen3-max","messages":[{"role":"user","content":"hi"}]}`, false},
		{"different content", `{"model":"qwen3","messages":[{"role":"user","content":"hello"}]}`, false},
		{"temperature added", `{"model":"qwen3","messages":[{"role":"user","content":"hi"}],"temperature":0.1}`, false},
		{"max_tokens added", `{"model":"qwen3","messages":[{"role":"user","content":"hi"}],"max_tokens":64}`, false},
		{"system message added", `{"model":"qwen3","messages":[{"role":"system","content":"s"},{"role":"user","content":"hi"}]}`, false},
		// 未知的厂商私有字段必须参与哈希：不认识的参数只能造成 miss，不能造成错误命中。
		{"unknown vendor field", `{"model":"qwen3","messages":[{"role":"user","content":"hi"}],"some_future_param":1}`, false},
	}

	setting := testCacheSetting()
	baseKey, err := buildResponseCacheKey(testCacheRelayInfo(), setting, []byte(base))
	require.NoError(t, err)
	require.NotEmpty(t, baseKey)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			key, err := buildResponseCacheKey(testCacheRelayInfo(), setting, []byte(tc.body))
			require.NoError(t, err)
			if tc.wantEqual {
				assert.Equal(t, baseKey, key)
			} else {
				assert.NotEqual(t, baseKey, key)
			}
		})
	}
}

// 大整数必须逐字节参与哈希。若键构造走 JSON 归一化（map + float64），
// 这两个 seed 会塌成同一个键并互相返回对方的答案。
func TestBuildResponseCacheKeyPreservesLargeIntegerPrecision(t *testing.T) {
	setting := testCacheSetting()
	first, err := buildResponseCacheKey(testCacheRelayInfo(), setting,
		[]byte(`{"model":"qwen3","messages":[{"role":"user","content":"hi"}],"seed":10000000000000001}`))
	require.NoError(t, err)
	second, err := buildResponseCacheKey(testCacheRelayInfo(), setting,
		[]byte(`{"model":"qwen3","messages":[{"role":"user","content":"hi"}],"seed":10000000000000002}`))
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

// 共享范围决定缓存条目在谁之间可见。user 隔离失效等于跨用户泄漏模型输出。
func TestBuildResponseCacheKeyScopeIsolation(t *testing.T) {
	body := []byte(`{"model":"qwen3","messages":[{"role":"user","content":"hi"}]}`)

	keyFor := func(scope string, userID int, group string) string {
		info := testCacheRelayInfo()
		info.UserId = userID
		info.UsingGroup = group
		setting := testCacheSetting()
		setting.ShareScope = scope

		key, err := buildResponseCacheKey(info, setting, body)
		require.NoError(t, err)
		return key
	}

	t.Run("user scope separates users", func(t *testing.T) {
		assert.NotEqual(t,
			keyFor(operation_setting.ResponseCacheScopeUser, 1, "default"),
			keyFor(operation_setting.ResponseCacheScopeUser, 2, "default"))
	})
	t.Run("group scope shares within a group", func(t *testing.T) {
		assert.Equal(t,
			keyFor(operation_setting.ResponseCacheScopeGroup, 1, "default"),
			keyFor(operation_setting.ResponseCacheScopeGroup, 2, "default"))
	})
	t.Run("group scope separates groups", func(t *testing.T) {
		assert.NotEqual(t,
			keyFor(operation_setting.ResponseCacheScopeGroup, 1, "default"),
			keyFor(operation_setting.ResponseCacheScopeGroup, 1, "vip"))
	})
	t.Run("global scope shares across users and groups", func(t *testing.T) {
		assert.Equal(t,
			keyFor(operation_setting.ResponseCacheScopeGlobal, 1, "default"),
			keyFor(operation_setting.ResponseCacheScopeGlobal, 2, "vip"))
	})
	t.Run("unknown scope falls back to user isolation", func(t *testing.T) {
		assert.NotEqual(t, keyFor("bogus", 1, "default"), keyFor("bogus", 2, "default"))
	})
}

func TestBuildResponseCacheKeySeparatesRelayMode(t *testing.T) {
	body := []byte(`{"model":"qwen3","messages":[{"role":"user","content":"hi"}]}`)
	setting := testCacheSetting()

	chat := testCacheRelayInfo()
	chatKey, err := buildResponseCacheKey(chat, setting, body)
	require.NoError(t, err)

	completions := testCacheRelayInfo()
	completions.RelayMode = relayconstant.RelayModeCompletions
	completionsKey, err := buildResponseCacheKey(completions, setting, body)
	require.NoError(t, err)

	assert.NotEqual(t, chatKey, completionsKey)
}

// 准入门槛是误命中的第一道也是最重要的一道防线。
func TestResponseCacheRequestEligible(t *testing.T) {
	userMessage := func() []dto.Message {
		return []dto.Message{{Role: "user", Content: "hi"}}
	}
	floatPtr := func(v float64) *float64 { return &v }
	intPtr := func(v int) *int { return &v }
	boolPtr := func(v bool) *bool { return &v }

	tests := []struct {
		name    string
		mutate  func(*dto.GeneralOpenAIRequest)
		setting func(*operation_setting.ResponseCacheSetting)
		want    bool
	}{
		{name: "plain chat request", want: true},
		{
			name:   "temperature at threshold",
			mutate: func(r *dto.GeneralOpenAIRequest) { r.Temperature = floatPtr(0.2) },
			want:   true,
		},
		{
			name:   "temperature above threshold",
			mutate: func(r *dto.GeneralOpenAIRequest) { r.Temperature = floatPtr(0.7) },
			want:   false,
		},
		{
			name:   "explicit n=1",
			mutate: func(r *dto.GeneralOpenAIRequest) { r.N = intPtr(1) },
			want:   true,
		},
		{
			name:   "n greater than one",
			mutate: func(r *dto.GeneralOpenAIRequest) { r.N = intPtr(2) },
			want:   false,
		},
		{
			name:   "logprobs requested",
			mutate: func(r *dto.GeneralOpenAIRequest) { r.LogProbs = boolPtr(true) },
			want:   false,
		},
		{
			name:   "top_logprobs requested",
			mutate: func(r *dto.GeneralOpenAIRequest) { r.TopLogProbs = intPtr(3) },
			want:   false,
		},
		{
			name:   "tools present, caching tool requests disabled",
			mutate: func(r *dto.GeneralOpenAIRequest) { r.Tools = []dto.ToolCallRequest{{}} },
			want:   false,
		},
		{
			name:    "tools present, caching tool requests enabled",
			mutate:  func(r *dto.GeneralOpenAIRequest) { r.Tools = []dto.ToolCallRequest{{}} },
			setting: func(s *operation_setting.ResponseCacheSetting) { s.CacheToolRequests = true },
			want:    true,
		},
		{
			name: "tool result message in history",
			mutate: func(r *dto.GeneralOpenAIRequest) {
				r.Messages = append(r.Messages, dto.Message{Role: "tool", Content: "result", ToolCallId: "call_1"})
			},
			want: false,
		},
		{
			name: "assistant tool_calls in history",
			mutate: func(r *dto.GeneralOpenAIRequest) {
				r.Messages = append(r.Messages, dto.Message{
					Role:      "assistant",
					ToolCalls: []byte(`[{"id":"call_1"}]`),
				})
			},
			want: false,
		},
		{
			name: "multimodal image part",
			mutate: func(r *dto.GeneralOpenAIRequest) {
				r.Messages = []dto.Message{{
					Role: "user",
					Content: []any{
						dto.MediaContent{Type: dto.ContentTypeText, Text: "what is this"},
						dto.MediaContent{Type: dto.ContentTypeImageURL, ImageUrl: map[string]any{"url": "https://x/y.png"}},
					},
				}}
			},
			want: false,
		},
		{
			name: "multipart text-only content",
			mutate: func(r *dto.GeneralOpenAIRequest) {
				r.Messages = []dto.Message{{
					Role:    "user",
					Content: []any{dto.MediaContent{Type: dto.ContentTypeText, Text: "hi"}},
				}}
			},
			want: true,
		},
		{
			name:   "empty message list",
			mutate: func(r *dto.GeneralOpenAIRequest) { r.Messages = nil },
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := &dto.GeneralOpenAIRequest{Model: "qwen3", Messages: userMessage()}
			if tc.mutate != nil {
				tc.mutate(request)
			}
			setting := testCacheSetting()
			if tc.setting != nil {
				tc.setting(setting)
			}
			assert.Equal(t, tc.want, responseCacheRequestEligible(request, setting))
		})
	}
}

func TestParseCachedJSONResponse(t *testing.T) {
	t.Run("complete response", func(t *testing.T) {
		payload := []byte(`{"id":"chatcmpl-1","model":"qwen3","object":"chat.completion",
			"choices":[{"index":0,"message":{"role":"assistant","content":"四十二"},"finish_reason":"stop"}],
			"usage":{"prompt_tokens":11,"completion_tokens":3,"total_tokens":14}}`)

		cached := parseCachedResponse(payload, false)
		require.NotNil(t, cached)
		assert.Equal(t, "四十二", cached.Content)
		assert.Equal(t, "stop", cached.FinishReason)
		assert.Equal(t, 11, cached.PromptTokens)
		assert.Equal(t, 3, cached.CompletionTokens)
		assert.Equal(t, 14, cached.TotalTokens)
	})

	t.Run("tool call response is not cacheable", func(t *testing.T) {
		payload := []byte(`{"id":"chatcmpl-1","model":"qwen3","object":"chat.completion",
			"choices":[{"index":0,"message":{"role":"assistant","content":"","tool_calls":[{"id":"call_1"}]},"finish_reason":"tool_calls"}],
			"usage":{"prompt_tokens":11,"completion_tokens":3,"total_tokens":14}}`)

		assert.Nil(t, parseCachedResponse(payload, false))
	})

	t.Run("empty content is not cacheable", func(t *testing.T) {
		payload := []byte(`{"id":"chatcmpl-1","choices":[{"index":0,"message":{"role":"assistant","content":""},"finish_reason":"stop"}]}`)
		assert.Nil(t, parseCachedResponse(payload, false))
	})

	t.Run("malformed payload is not cacheable", func(t *testing.T) {
		assert.Nil(t, parseCachedResponse([]byte(`not json`), false))
	})
}

func TestParseCachedStreamResponse(t *testing.T) {
	t.Run("reassembles deltas and usage", func(t *testing.T) {
		payload := []byte(`data: {"id":"1","choices":[{"index":0,"delta":{"role":"assistant","content":""}}]}

data: {"id":"1","choices":[{"index":0,"delta":{"content":"四十"}}]}

data: {"id":"1","choices":[{"index":0,"delta":{"content":"二"}}]}

data: {"id":"1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: {"id":"1","choices":[],"usage":{"prompt_tokens":11,"completion_tokens":3,"total_tokens":14}}

data: [DONE]

`)

		cached := parseCachedResponse(payload, true)
		require.NotNil(t, cached)
		assert.Equal(t, "四十二", cached.Content)
		assert.Equal(t, "stop", cached.FinishReason)
		assert.Equal(t, 11, cached.PromptTokens)
		assert.Equal(t, 3, cached.CompletionTokens)
		assert.Equal(t, 14, cached.TotalTokens)
	})

	t.Run("collects reasoning content separately", func(t *testing.T) {
		payload := []byte(`data: {"id":"1","choices":[{"index":0,"delta":{"reasoning_content":"想一下"}}]}

data: {"id":"1","choices":[{"index":0,"delta":{"content":"答案"}}]}

data: {"id":"1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}

data: [DONE]

`)

		cached := parseCachedResponse(payload, true)
		require.NotNil(t, cached)
		assert.Equal(t, "答案", cached.Content)
		assert.Equal(t, "想一下", cached.ReasoningContent)
	})

	// 没有 finish_reason 说明流被中断，半截响应进了缓存会被后续请求当成完整答案。
	t.Run("truncated stream is not cacheable", func(t *testing.T) {
		payload := []byte(`data: {"id":"1","choices":[{"index":0,"delta":{"content":"半截"}}]}

`)
		assert.Nil(t, parseCachedResponse(payload, true))
	})

	t.Run("tool call deltas are not cacheable", func(t *testing.T) {
		payload := []byte(`data: {"id":"1","choices":[{"index":0,"delta":{"tool_calls":[{"id":"call_1"}]}}]}

data: {"id":"1","choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}

data: [DONE]

`)
		assert.Nil(t, parseCachedResponse(payload, true))
	})
}

func testReplayContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, recorder
}

// 回放出去的字节必须能被捕获层解析回同一条目。这条往返不变量把渲染和解析
// 绑在一起：任何一侧改了格式而另一侧没跟上，测试立刻失败。
func TestCachedStreamReplayRoundTrip(t *testing.T) {
	cached := &CachedResponse{
		Content:          "四十二是终极答案",
		ReasoningContent: "先想一下这个问题",
		FinishReason:     "stop",
		PromptTokens:     11,
		CompletionTokens: 8,
		TotalTokens:      19,
	}
	usage := &dto.Usage{PromptTokens: 11, CompletionTokens: 8, TotalTokens: 19}

	c, recorder := testReplayContext(t)
	relayInfo := testCacheRelayInfo()
	relayInfo.OriginModelName = "qwen3"
	relayInfo.IsStream = true
	relayInfo.Request = &dto.GeneralOpenAIRequest{
		Model:         "qwen3",
		StreamOptions: &dto.StreamOptions{IncludeUsage: true},
	}

	setting := testCacheSetting()
	setting.StreamReplayChunkSize = 3

	require.NoError(t, writeCachedStreamResponse(c, relayInfo, cached, usage, setting))

	body := recorder.Body.String()
	assert.Contains(t, body, "data: ")
	assert.True(t, strings.HasSuffix(strings.TrimSpace(body), "data: [DONE]"),
		"stream must terminate with the SSE done marker, got: %q", body)

	reparsed := parseCachedStreamResponse(recorder.Body.Bytes())
	require.NotNil(t, reparsed)
	assert.Equal(t, cached.Content, reparsed.Content)
	assert.Equal(t, cached.ReasoningContent, reparsed.ReasoningContent)
	assert.Equal(t, cached.FinishReason, reparsed.FinishReason)
	assert.Equal(t, cached.PromptTokens, reparsed.PromptTokens)
	assert.Equal(t, cached.CompletionTokens, reparsed.CompletionTokens)
	assert.Equal(t, cached.TotalTokens, reparsed.TotalTokens)
}

// stream_options 取自当前请求而非写入缓存时的那次请求，否则同一条目服务于
// 两类客户端时会给出对方不期望的分片。
func TestCachedStreamReplayOmitsUsageWhenNotRequested(t *testing.T) {
	cached := &CachedResponse{Content: "hi", FinishReason: "stop", TotalTokens: 5}
	usage := &dto.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}

	c, recorder := testReplayContext(t)
	relayInfo := testCacheRelayInfo()
	relayInfo.OriginModelName = "qwen3"
	relayInfo.IsStream = true
	relayInfo.Request = &dto.GeneralOpenAIRequest{Model: "qwen3"}

	require.NoError(t, writeCachedStreamResponse(c, relayInfo, cached, usage, testCacheSetting()))

	// 分片结构里 usage 字段没有 omitempty，正常分片一律是 "usage":null；
	// 这里要断言的是没有任何分片携带真正的 usage 对象。
	assert.NotContains(t, recorder.Body.String(), `"usage":{`)
}

func TestCachedJSONReplayRoundTrip(t *testing.T) {
	cached := &CachedResponse{
		Content:          "四十二",
		FinishReason:     "stop",
		PromptTokens:     11,
		CompletionTokens: 3,
		TotalTokens:      14,
	}
	usage := &dto.Usage{PromptTokens: 11, CompletionTokens: 3, TotalTokens: 14}

	c, recorder := testReplayContext(t)
	relayInfo := testCacheRelayInfo()
	relayInfo.OriginModelName = "qwen3"
	relayInfo.Request = &dto.GeneralOpenAIRequest{Model: "qwen3"}

	writeCachedJSONResponse(c, relayInfo, cached, usage)

	reparsed := parseCachedJSONResponse(recorder.Body.Bytes())
	require.NotNil(t, reparsed)
	assert.Equal(t, cached.Content, reparsed.Content)
	assert.Equal(t, cached.FinishReason, reparsed.FinishReason)
	assert.Equal(t, cached.PromptTokens, reparsed.PromptTokens)
	assert.Equal(t, cached.CompletionTokens, reparsed.CompletionTokens)
	assert.Equal(t, cached.TotalTokens, reparsed.TotalTokens)

	// 回放必须使用用户请求的模型名，而不是缓存写入时的上游模型名。
	assert.Contains(t, recorder.Body.String(), `"model":"qwen3"`)
}

// 按字节切分会把多字节字符劈开，客户端收到的是乱码。
func TestSplitIntoRuneChunks(t *testing.T) {
	tests := []struct {
		name string
		in   string
		size int
		want []string
	}{
		{"empty", "", 3, nil},
		{"exact multiple", "abcdef", 3, []string{"abc", "def"}},
		{"trailing partial", "abcde", 3, []string{"abc", "de"}},
		{"multibyte stays intact", "四十二是答案", 2, []string{"四十", "二是", "答案"}},
		{"size exceeds length", "ab", 8, []string{"ab"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, splitIntoRuneChunks(tc.in, tc.size))
		})
	}
}
