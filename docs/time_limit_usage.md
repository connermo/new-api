# 令牌时段限制功能使用指南

## 功能概述

New API 的令牌时段限制功能允许管理员为用户令牌设置使用时间限制，只允许在指定的时间段内使用令牌。这对于控制访问时间、确保安全合规性等方面非常有用。

## 功能特性

- **灵活的时间配置**: 支持按星期几和具体时间段进行限制
- **多规则支持**: 一个令牌可以配置多个时间规则
- **实时验证**: API 请求时实时检查当前时间是否在允许范围内
- **向后兼容**: 不影响现有令牌的正常使用

## 配置说明

### 时段规则结构

每个时段规则包含以下字段：

```json
{
  "day_of_week": -1,
  "start_time": "09:00",
  "end_time": "17:00"
}
```

字段说明：
- `day_of_week`: 星期几设置
  - `-1`: 每天
  - `0`: 周日
  - `1`: 周一
  - `2`: 周二
  - `3`: 周三
  - `4`: 周四
  - `5`: 周五
  - `6`: 周六
- `start_time`: 开始时间，格式为 "HH:MM"
- `end_time`: 结束时间，格式为 "HH:MM"

### 完整配置示例

```json
{
  "rules": [
    {
      "day_of_week": 1,
      "start_time": "09:00",
      "end_time": "17:00"
    },
    {
      "day_of_week": 2,
      "start_time": "09:00",
      "end_time": "17:00"
    },
    {
      "day_of_week": 3,
      "start_time": "09:00",
      "end_time": "17:00"
    },
    {
      "day_of_week": 4,
      "start_time": "09:00",
      "end_time": "17:00"
    },
    {
      "day_of_week": 5,
      "start_time": "09:00",
      "end_time": "17:00"
    }
  ]
}
```

这个配置表示令牌只允许在周一到周五的上午9点到下午5点使用。

## API 使用方法

### 创建带时段限制的令牌

```bash
POST /api/token
Content-Type: application/json

{
  "name": "工作时间令牌",
  "time_limit_enabled": true,
  "time_limit_config": "{\"rules\":[{\"day_of_week\":1,\"start_time\":\"09:00\",\"end_time\":\"17:00\"},{\"day_of_week\":2,\"start_time\":\"09:00\",\"end_time\":\"17:00\"},{\"day_of_week\":3,\"start_time\":\"09:00\",\"end_time\":\"17:00\"},{\"day_of_week\":4,\"start_time\":\"09:00\",\"end_time\":\"17:00\"},{\"day_of_week\":5,\"start_time\":\"09:00\",\"end_time\":\"17:00\"}]}"
}
```

### 更新令牌的时段限制

```bash
PUT /api/token
Content-Type: application/json

{
  "id": 123,
  "name": "工作时间令牌",
  "time_limit_enabled": true,
  "time_limit_config": "{\"rules\":[{\"day_of_week\":-1,\"start_time\":\"08:00\",\"end_time\":\"20:00\"}]}"
}
```

### 禁用时段限制

```bash
PUT /api/token
Content-Type: application/json

{
  "id": 123,
  "time_limit_enabled": false,
  "time_limit_config": ""
}
```

## 使用场景示例

### 1. 工作时间限制

只允许在工作时间内使用：

```json
{
  "rules": [
    {"day_of_week": 1, "start_time": "09:00", "end_time": "18:00"},
    {"day_of_week": 2, "start_time": "09:00", "end_time": "18:00"},
    {"day_of_week": 3, "start_time": "09:00", "end_time": "18:00"},
    {"day_of_week": 4, "start_time": "09:00", "end_time": "18:00"},
    {"day_of_week": 5, "start_time": "09:00", "end_time": "18:00"}
  ]
}
```

### 2. 学习时间限制

只允许在学习时间内使用：

```json
{
  "rules": [
    {"day_of_week": -1, "start_time": "19:00", "end_time": "22:00"}
  ]
}
```

### 3. 周末娱乐限制

只允许周末使用：

```json
{
  "rules": [
    {"day_of_week": 0, "start_time": "00:00", "end_time": "23:59"},
    {"day_of_week": 6, "start_time": "00:00", "end_time": "23:59"}
  ]
}
```

### 4. 夜间维护窗口

只允许在维护时间外使用：

```json
{
  "rules": [
    {"day_of_week": -1, "start_time": "06:00", "end_time": "02:00"}
  ]
}
```

## 错误处理

当令牌在限制时间外使用时，API 会返回以下错误：

```json
{
  "error": {
    "message": "该令牌当前时段不可用，请检查令牌的使用时间限制",
    "type": "time_limit_violation"
  }
}
```

## 注意事项

1. **时区**: 时间限制基于服务器时区，请确保服务器时区设置正确
2. **性能**: 时段检查对性能影响很小，通常在微秒级别
3. **缓存**: Redis缓存会缓存时段限制配置，配置变更后会自动更新
4. **日志**: 所有时段限制相关的操作都会记录在系统日志中
5. **兼容性**: 现有令牌不受影响，新功能向后兼容

## 故障排除

### 常见问题

1. **令牌明明在允许时间内却被拒绝**
   - 检查服务器时区设置
   - 确认令牌配置是否正确保存
   - 查看系统日志中的错误信息

2. **配置无法保存**
   - 检查JSON格式是否正确
   - 确认时间格式为HH:MM
   - 验证开始时间是否小于结束时间

3. **时段限制不生效**
   - 确认 `time_limit_enabled` 设置为 `true`
   - 检查 `time_limit_config` 是否为空
   - 重启服务以确保配置生效

### 调试方法

启用详细日志来调试时段限制：

```bash
# 在环境变量中设置
ERROR_LOG_ENABLED=true
```

查看系统日志：
```bash
tail -f logs/oneapi-*.log | grep time_limit
```

## 扩展功能

未来版本可能会增加以下功能：

- 基于IP地址和时间的复合限制
- 自定义错误消息
- 临时覆盖功能（管理员手动允许）
- 批量配置管理
- 图形化时间选择器

## 技术实现

该功能基于以下技术实现：

- **数据存储**: JSON格式存储在数据库中
- **缓存策略**: Redis缓存配置，减少数据库查询
- **验证逻辑**: 实时时间检查，支持多规则组合
- **错误处理**: 统一的错误响应格式
- **性能优化**: 高效的时间比较算法
