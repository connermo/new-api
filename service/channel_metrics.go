package service

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/bytedance/gopkg/util/gopool"
)

const (
	defaultMetricsInterval = 15 * time.Second
	metricsFetchTimeout    = 5 * time.Second
)

var channelMetricsOnce sync.Once

// metricsResult holds parsed Prometheus metrics from a vLLM/SGLang endpoint.
type metricsResult struct {
	GPUCacheUsage    float64 // vllm:gpu_cache_usage_perc (0~1)
	RequestsWaiting  float64 // vllm:num_requests_waiting
	RequestsRunning  float64 // vllm:num_requests_running
	Reachable        bool
}

// channelMetricsEntry groups channels sharing the same metrics URL.
type channelMetricsEntry struct {
	MetricsURL  string
	MetricsType string
	ChannelIDs  []int
}

// StartChannelMetricsTask starts the background goroutine that periodically
// polls configured vLLM/SGLang Prometheus endpoints and adjusts channel weights.
func StartChannelMetricsTask() {
	channelMetricsOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}
		if !common.MemoryCacheEnabled {
			return
		}
		gopool.Go(func() {
			ctx := context.Background()
			logger.LogInfo(ctx, "channel metrics task started")
			for {
				pollChannelMetrics(ctx)
				interval := getMetricsInterval()
				time.Sleep(interval)
			}
		})
	})
}

// pollChannelMetrics collects all metrics-enabled channels, groups by URL,
// fetches metrics once per URL, and updates weights.
func pollChannelMetrics(ctx context.Context) {
	channels := model.CacheGetAllChannels()
	if len(channels) == 0 {
		return
	}

	// Group channels by metrics_url to avoid duplicate fetches.
	urlMap := make(map[string]*channelMetricsEntry)
	for _, ch := range channels {
		if ch.Status != common.ChannelStatusEnabled {
			continue
		}
		settings := ch.GetOtherSettings()
		if !settings.MetricsEnabled || settings.MetricsURL == "" {
			continue
		}
		metricsType := settings.MetricsType
		if metricsType == "" {
			metricsType = "vllm"
		}
		key := settings.MetricsURL
		if entry, ok := urlMap[key]; ok {
			entry.ChannelIDs = append(entry.ChannelIDs, ch.Id)
		} else {
			urlMap[key] = &channelMetricsEntry{
				MetricsURL:  settings.MetricsURL,
				MetricsType: metricsType,
				ChannelIDs:  []int{ch.Id},
			}
		}
	}

	if len(urlMap) == 0 {
		return
	}

	allUnreachable := true
	totalChannels := 0
	for _, entry := range urlMap {
		result := fetchAndParseMetrics(entry.MetricsURL, entry.MetricsType)
		weight := calculateWeight(result)

		for _, chID := range entry.ChannelIDs {
			model.CacheUpdateChannelWeight(chID, uint(weight))
		}
		totalChannels += len(entry.ChannelIDs)

		if !result.Reachable {
			logger.LogError(ctx, fmt.Sprintf("channel metrics: %s unreachable, weight set to %d for channels %v",
				entry.MetricsURL, weight, entry.ChannelIDs))
		} else {
			allUnreachable = false
			logger.LogInfo(ctx, fmt.Sprintf("channel metrics: %s gpu_cache=%.2f waiting=%.0f running=%.0f -> weight=%d channels=%v",
				entry.MetricsURL, result.GPUCacheUsage, result.RequestsWaiting, result.RequestsRunning, weight, entry.ChannelIDs))
		}
	}

	if allUnreachable {
		logger.LogError(ctx, fmt.Sprintf("channel metrics: WARNING all %d metrics endpoints unreachable, %d channels have weight 0 (will fall back to even distribution)",
			len(urlMap), totalChannels))
	}
}

// fetchAndParseMetrics fetches Prometheus text from the given URL and extracts key metrics.
func fetchAndParseMetrics(url, metricsType string) metricsResult {
	result := metricsResult{}

	client := &http.Client{Timeout: metricsFetchTimeout}
	resp, err := client.Get(url)
	if err != nil {
		return result // Reachable is false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return result
	}

	result.Reachable = true

	// Determine metric name prefixes based on type.
	// vLLM uses "vllm:" prefix, SGLang uses "sglang:" prefix.
	prefix := "vllm:"
	if metricsType == "sglang" {
		prefix = "sglang:"
	}

	gpuCacheMetric := prefix + "gpu_cache_usage_perc"
	waitingMetric := prefix + "num_requests_waiting"
	runningMetric := prefix + "num_requests_running"

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		if val, ok := extractMetricValue(line, gpuCacheMetric); ok {
			result.GPUCacheUsage = val
		} else if val, ok := extractMetricValue(line, waitingMetric); ok {
			result.RequestsWaiting = val
		} else if val, ok := extractMetricValue(line, runningMetric); ok {
			result.RequestsRunning = val
		}
	}

	return result
}

// extractMetricValue extracts a float64 value from a Prometheus text line
// that starts with the given metric name. Handles both bare metrics and
// those with labels: "metric_name{...} value" or "metric_name value".
func extractMetricValue(line, metricName string) (float64, bool) {
	if !strings.HasPrefix(line, metricName) {
		return 0, false
	}

	rest := line[len(metricName):]
	// After metric name, expect either '{' (labels) or ' ' (value).
	if len(rest) == 0 {
		return 0, false
	}

	var valueStr string
	if rest[0] == '{' {
		// Skip labels: find closing '}'
		idx := strings.IndexByte(rest, '}')
		if idx < 0 || idx+1 >= len(rest) {
			return 0, false
		}
		valueStr = strings.TrimSpace(rest[idx+1:])
	} else if rest[0] == ' ' || rest[0] == '\t' {
		valueStr = strings.TrimSpace(rest)
	} else {
		return 0, false
	}

	// Handle possible timestamp suffix: "value timestamp"
	if spaceIdx := strings.IndexByte(valueStr, ' '); spaceIdx >= 0 {
		valueStr = valueStr[:spaceIdx]
	}

	val, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		return 0, false
	}
	return val, true
}

// getMetricsInterval returns the polling interval. Environment variable
// CHANNEL_METRICS_INTERVAL (seconds) takes priority, otherwise the system
// setting monitor_setting.channel_metrics_interval_seconds is used.
func getMetricsInterval() time.Duration {
	if envVal := os.Getenv("CHANNEL_METRICS_INTERVAL"); envVal != "" {
		if seconds, err := strconv.Atoi(envVal); err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	ms := operation_setting.GetMonitorSetting()
	if ms.ChannelMetricsIntervalSeconds > 0 {
		return time.Duration(ms.ChannelMetricsIntervalSeconds) * time.Second
	}
	return defaultMetricsInterval
}

// calculateWeight converts metrics into a channel weight value.
func calculateWeight(m metricsResult) int {
	if !m.Reachable {
		return 0
	}

	usage := m.GPUCacheUsage

	switch {
	case usage < 0.30:
		return 100
	case usage < 0.60:
		return 75
	case usage < 0.80:
		return 50
	case usage < 0.95:
		return 20
	default:
		return 5
	}
}
