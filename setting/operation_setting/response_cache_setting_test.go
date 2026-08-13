package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 非法的共享范围必须退回 user。任何拼写错误都不能意外把缓存变成全站共享，
// 那等于跨用户泄漏模型输出。
func TestResponseCacheNormalizedShareScope(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"user", ResponseCacheScopeUser},
		{"group", ResponseCacheScopeGroup},
		{"global", ResponseCacheScopeGlobal},
		{"GLOBAL", ResponseCacheScopeGlobal},
		{"  group  ", ResponseCacheScopeGroup},
		{"", ResponseCacheScopeUser},
		{"everyone", ResponseCacheScopeUser},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			setting := ResponseCacheSetting{ShareScope: tc.in}
			assert.Equal(t, tc.want, setting.NormalizedShareScope())
		})
	}
}

// 白名单为空表示功能不生效；不提供“对所有模型生效”的写法。
func TestResponseCacheModelEnabled(t *testing.T) {
	setting := ResponseCacheSetting{EnabledModels: []string{"qwen3", " Qwen3-Max "}}

	assert.True(t, setting.ModelEnabled("qwen3"))
	assert.True(t, setting.ModelEnabled("QWEN3"))
	assert.True(t, setting.ModelEnabled("qwen3-max"))
	assert.False(t, setting.ModelEnabled("gpt-4o"))
	assert.False(t, setting.ModelEnabled(""))

	empty := ResponseCacheSetting{}
	assert.False(t, empty.ModelEnabled("qwen3"))
}

// 命中计费比例配错时必须退回原价而不是零价：多收可以退，白送收不回来。
func TestResponseCacheEffectiveHitBillingRatio(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want float64
	}{
		{"normal discount", 0.2, 0.2},
		{"full price", 1, 1},
		{"zero falls back to full price", 0, 1},
		{"negative falls back to full price", -0.5, 1},
		{"above one falls back to full price", 1.5, 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			setting := ResponseCacheSetting{HitBillingRatio: tc.in}
			assert.Equal(t, tc.want, setting.EffectiveHitBillingRatio())
		})
	}
}

func TestResponseCacheChargeOnHit(t *testing.T) {
	chargeOnHit := func(hitBilling string) bool {
		setting := ResponseCacheSetting{HitBilling: hitBilling}
		return setting.ChargeOnHit()
	}

	assert.True(t, chargeOnHit(ResponseCacheHitBillingRatio))
	assert.True(t, chargeOnHit("RATIO"))
	assert.False(t, chargeOnHit(ResponseCacheHitBillingFree))
	// 未配置时按 free 处理，与默认值一致。
	assert.False(t, chargeOnHit(""))
}

// 默认值本身就是一项契约：功能默认关闭、默认按用户隔离、默认不永久驻留。
func TestResponseCacheDefaultsAreConservative(t *testing.T) {
	setting := GetResponseCacheSetting()

	assert.False(t, setting.Enabled)
	assert.Empty(t, setting.EnabledModels)
	assert.Equal(t, ResponseCacheScopeUser, setting.NormalizedShareScope())
	assert.False(t, setting.CacheToolRequests)
	assert.Greater(t, setting.TTLSeconds, 0)
}
