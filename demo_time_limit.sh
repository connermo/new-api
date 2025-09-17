#!/bin/bash

# New API 时段限制功能演示脚本
# 此脚本演示如何创建和配置带时段限制的令牌

echo "=== New API 时段限制功能演示 ==="
echo

# 演示1: 创建工作时间限制的令牌
echo "1. 创建工作时间令牌（周一到周五，9:00-17:00）"
echo "POST /api/token"
echo '{
  "name": "工作时间令牌",
  "time_limit_enabled": true,
  "time_limit_config": {
    "rules": [
      {"day_of_week": 1, "start_time": "09:00", "end_time": "17:00"},
      {"day_of_week": 2, "start_time": "09:00", "end_time": "17:00"},
      {"day_of_week": 3, "start_time": "09:00", "end_time": "17:00"},
      {"day_of_week": 4, "start_time": "09:00", "end_time": "17:00"},
      {"day_of_week": 5, "start_time": "09:00", "end_time": "17:00"}
    ]
  }
}'
echo

# 演示2: 创建学习时间限制的令牌
echo "2. 创建学习时间令牌（每天晚上7:00-10:00）"
echo "POST /api/token"
echo '{
  "name": "学习时间令牌",
  "time_limit_enabled": true,
  "time_limit_config": {
    "rules": [
      {"day_of_week": -1, "start_time": "19:00", "end_time": "22:00"}
    ]
  }
}'
echo

# 演示3: 创建周末娱乐时间限制的令牌
echo "3. 创建周末娱乐令牌（仅周末使用）"
echo "POST /api/token"
echo '{
  "name": "周末娱乐令牌",
  "time_limit_enabled": true,
  "time_limit_config": {
    "rules": [
      {"day_of_week": 0, "start_time": "00:00", "end_time": "23:59"},
      {"day_of_week": 6, "start_time": "00:00", "end_time": "23:59"}
    ]
  }
}'
echo

# 演示4: 创建维护窗口限制的令牌
echo "4. 创建维护窗口令牌（仅在非维护时间使用）"
echo "POST /api/token"
echo '{
  "name": "维护窗口令牌",
  "time_limit_enabled": true,
  "time_limit_config": {
    "rules": [
      {"day_of_week": -1, "start_time": "06:00", "end_time": "02:00"}
    ]
  }
}'
echo

# 演示5: 更新令牌的时段限制
echo "5. 更新令牌时段限制为全天可用"
echo "PUT /api/token"
echo '{
  "id": 123,
  "time_limit_enabled": false,
  "time_limit_config": ""
}'
echo

echo "=== 使用说明 ==="
echo "1. time_limit_enabled: 是否启用时段限制"
echo "2. time_limit_config: JSON格式的时段配置"
echo "3. day_of_week: -1=每天, 0-6=周日到周六"
echo "4. start_time/end_time: HH:MM格式的时间"
echo "5. 多个规则之间是OR关系，满足任一规则即可使用"
echo

echo "=== 错误示例 ==="
echo "当令牌在限制时间外使用时，会返回："
echo '{
  "error": {
    "message": "该令牌当前时段不可用，请检查令牌的使用时间限制",
    "type": "time_limit_violation"
  }
}'
echo

echo "=== 管理命令 ==="
echo "# 查看令牌列表"
echo "GET /api/token"
echo
echo "# 查看特定令牌"
echo "GET /api/token/{id}"
echo
echo "# 更新令牌时段限制"
echo "PUT /api/token"
echo
echo "# 删除令牌"
echo "DELETE /api/token/{id}"
echo

echo "演示完成！请根据实际需要配置令牌的时段限制。"
