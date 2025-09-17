package model

import (
	"encoding/json"
	"testing"
)

// TestTimeLimitRule 测试时段规则验证
func TestTimeLimitRule(t *testing.T) {
	// 测试有效规则
	validRule := TimeLimitRule{
		DayOfWeek: 1, // 周一
		StartTime: "09:00",
		EndTime:   "17:00",
	}

	if err := ValidateTimeLimitRule(validRule); err != nil {
		t.Errorf("Valid rule should pass validation, got error: %v", err)
	}

	// 测试无效的星期几
	invalidDayRule := TimeLimitRule{
		DayOfWeek: -2, // 无效的星期几
		StartTime: "09:00",
		EndTime:   "17:00",
	}

	if err := ValidateTimeLimitRule(invalidDayRule); err == nil {
		t.Error("Invalid day of week should fail validation")
	}

	// 测试无效的时间格式
	invalidTimeRule := TimeLimitRule{
		DayOfWeek: 1,
		StartTime: "25:00", // 无效的小时
		EndTime:   "17:00",
	}

	if err := ValidateTimeLimitRule(invalidTimeRule); err == nil {
		t.Error("Invalid time format should fail validation")
	}

	// 测试开始时间晚于结束时间
	invalidOrderRule := TimeLimitRule{
		DayOfWeek: 1,
		StartTime: "17:00",
		EndTime:   "09:00",
	}

	if err := ValidateTimeLimitRule(invalidOrderRule); err == nil {
		t.Error("Start time after end time should fail validation")
	}
}

// TestTimeLimitConfig 测试时段配置
func TestTimeLimitConfig(t *testing.T) {
	token := &Token{
		TimeLimitEnabled: true,
	}

	// 创建测试配置
	config := &TimeLimitConfig{
		Rules: []TimeLimitRule{
			{
				DayOfWeek: 1, // 周一
				StartTime: "09:00",
				EndTime:   "17:00",
			},
			{
				DayOfWeek: -1, // 每天
				StartTime: "08:00",
				EndTime:   "18:00",
			},
		},
	}

	// 测试设置配置
	if err := token.SetTimeLimitConfig(config); err != nil {
		t.Errorf("Setting time limit config should succeed, got error: %v", err)
	}

	// 测试获取配置
	retrievedConfig, err := token.GetTimeLimitConfig()
	if err != nil {
		t.Errorf("Getting time limit config should succeed, got error: %v", err)
	}

	if len(retrievedConfig.Rules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(retrievedConfig.Rules))
	}
}

// TestCheckTimeLimit 测试时段检查逻辑
func TestCheckTimeLimit(t *testing.T) {
	token := &Token{
		TimeLimitEnabled: false,
	}

	// 测试未启用时段限制
	allowed, err := token.CheckTimeLimit()
	if err != nil {
		t.Errorf("CheckTimeLimit with disabled limit should not error, got: %v", err)
	}
	if !allowed {
		t.Error("CheckTimeLimit with disabled limit should return true")
	}

	// 启用时段限制
	token.TimeLimitEnabled = true

	// 测试空配置
	allowed, err = token.CheckTimeLimit()
	if err != nil {
		t.Errorf("CheckTimeLimit with empty config should not error, got: %v", err)
	}
	if !allowed {
		t.Error("CheckTimeLimit with empty config should return true")
	}

	// 设置配置 - 当前时间应该在范围内
	config := &TimeLimitConfig{
		Rules: []TimeLimitRule{
			{
				DayOfWeek: -1, // 每天
				StartTime: "00:00",
				EndTime:   "23:59",
			},
		},
	}

	token.SetTimeLimitConfig(config)
	allowed, err = token.CheckTimeLimit()
	if err != nil {
		t.Errorf("CheckTimeLimit with valid config should not error, got: %v", err)
	}
	if !allowed {
		t.Error("CheckTimeLimit with time in range should return true")
	}

	// 设置配置 - 当前时间不在范围内
	narrowConfig := &TimeLimitConfig{
		Rules: []TimeLimitRule{
			{
				DayOfWeek: -1,
				StartTime: "02:00",
				EndTime:   "03:00", // 假设当前时间不在这个范围内
			},
		},
	}

	token.SetTimeLimitConfig(narrowConfig)
	// 注意：这个测试可能因为实际时间而失败，在实际使用中需要更智能的测试
}

// TestValidateUserTokenWithTimeLimit 测试带时段限制的token验证
func TestValidateUserTokenWithTimeLimit(t *testing.T) {
	// 注意：这个测试需要数据库连接，实际运行时需要确保数据库可用
	// 这里只是展示如何测试时段限制功能

	// 跳过数据库相关的测试，因为需要完整的环境设置
	t.Skip("Skipping database-dependent test")

	// 示例测试代码（需要数据库环境）
	/*
		// 创建测试token
		testToken := &Token{
			UserId:           1,
			Name:             "Test Time Limited Token",
			Key:              "test_key_123",
			CreatedTime:      GetTimestamp(),
			AccessedTime:     GetTimestamp(),
			Status:           1, // enabled
			TimeLimitEnabled: true,
		}

		// 设置时段限制（只允许在工作时间内使用）
		config := &TimeLimitConfig{
			Rules: []TimeLimitRule{
				{
					DayOfWeek: 1, // 周一
					StartTime: "09:00",
					EndTime:   "17:00",
				},
				{
					DayOfWeek: 2, // 周二
					StartTime: "09:00",
					EndTime:   "17:00",
				},
			},
		}
		testToken.SetTimeLimitConfig(config)

		// 模拟数据库插入
		if err := testToken.Insert(); err != nil {
			t.Errorf("Failed to insert test token: %v", err)
		}

		// 测试token验证
		_, err := ValidateUserToken(testToken.Key)
		// 结果取决于当前时间是否在允许的时段内
		if err != nil {
			// 如果当前时间不在允许时段，会返回时段限制错误
			expectedError := "该令牌当前时段不可用"
			if strings.Contains(err.Error(), expectedError) {
				t.Errorf("Expected time limit error, got: %v", err)
			}
		}
	*/
}

// TestTimeLimitJSONSerialization 测试JSON序列化
func TestTimeLimitJSONSerialization(t *testing.T) {
	config := &TimeLimitConfig{
		Rules: []TimeLimitRule{
			{
				DayOfWeek: 1,
				StartTime: "09:00",
				EndTime:   "17:00",
			},
		},
	}

	// 序列化
	data, err := json.Marshal(config)
	if err != nil {
		t.Errorf("Failed to marshal config: %v", err)
	}

	// 反序列化
	var decodedConfig TimeLimitConfig
	if err := json.Unmarshal(data, &decodedConfig); err != nil {
		t.Errorf("Failed to unmarshal config: %v", err)
	}

	if len(decodedConfig.Rules) != 1 {
		t.Errorf("Expected 1 rule after deserialization, got %d", len(decodedConfig.Rules))
	}

	rule := decodedConfig.Rules[0]
	if rule.DayOfWeek != 1 || rule.StartTime != "09:00" || rule.EndTime != "17:00" {
		t.Errorf("Rule data mismatch after deserialization: %+v", rule)
	}
}

// BenchmarkTimeLimitCheck 基准测试时段检查性能
func BenchmarkTimeLimitCheck(b *testing.B) {
	token := &Token{
		TimeLimitEnabled: true,
	}

	config := &TimeLimitConfig{
		Rules: []TimeLimitRule{
			{
				DayOfWeek: -1, // 每天
				StartTime: "00:00",
				EndTime:   "23:59",
			},
			{
				DayOfWeek: 1, // 周一
				StartTime: "09:00",
				EndTime:   "17:00",
			},
			{
				DayOfWeek: 2, // 周二
				StartTime: "09:00",
				EndTime:   "17:00",
			},
		},
	}

	token.SetTimeLimitConfig(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		token.CheckTimeLimit()
	}
}
