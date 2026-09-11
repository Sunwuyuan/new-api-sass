package perfmetrics

import context "context"

import (
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

func flushLoop(tenantCtx context.Context) {
	for {
		interval := perf_metrics_setting.GetFlushIntervalMinutes(tenantCtx)
		time.Sleep(time.Duration(interval) * time.Minute)
		setting := perf_metrics_setting.GetSetting(tenantCtx)
		if !setting.Enabled {
			continue
		}
		flushCompletedBuckets(tenantCtx)
		cleanupExpiredMetrics(tenantCtx, setting.RetentionDays)
	}
}

func flushCompletedBuckets(tenantCtx context.Context) {
	currentBucket := bucketStart(tenantCtx, time.Now().Unix())
	TenantRuntime(tenantCtx).hotBuckets.Range(func(key, value any) bool {
		k := key.(bucketKey)
		if k.bucketTs >= currentBucket {
			return true
		}

		bucket := value.(*atomicBucket)
		drained := bucket.drain()
		if drained.requestCount == 0 {
			deleteOldEmptyBucket(tenantCtx, k, key)
			return true
		}

		err := model.UpsertPerfMetric(tenantCtx, &model.PerfMetric{
			ModelName:      k.model,
			Group:          k.group,
			BucketTs:       k.bucketTs,
			RequestCount:   drained.requestCount,
			SuccessCount:   drained.successCount,
			TotalLatencyMs: drained.totalLatencyMs,
			TtftSumMs:      drained.ttftSumMs,
			TtftCount:      drained.ttftCount,
			OutputTokens:   drained.outputTokens,
			GenerationMs:   drained.generationMs,
		})
		if err != nil {
			bucket.addCounters(drained)
			common.SysError(fmt.Sprintf("failed to flush perf metric bucket model=%s group=%s bucket=%d: %s", k.model, k.group, k.bucketTs, err.Error()))
			return true
		}

		deleteOldEmptyBucket(tenantCtx, k, key)
		return true
	})
}

func deleteOldEmptyBucket(tenantCtx context.Context, k bucketKey, rawKey any) {
	if k.bucketTs < bucketStart(tenantCtx, time.Now().Add(-24*time.Hour).Unix()) {
		TenantRuntime(tenantCtx).hotBuckets.Delete(rawKey)
	}
}

func cleanupExpiredMetrics(tenantCtx context.Context, retentionDays int) {
	if retentionDays <= 0 {
		return
	}
	cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour).Unix()
	if err := model.DeletePerfMetricsBefore(tenantCtx, cutoff); err != nil {
		common.SysError("failed to cleanup expired perf metrics: " + err.Error())
	}
}

func redisCounters(values map[string]string) counters {
	return counters{
		requestCount:   parseRedisInt(values["req"]),
		successCount:   parseRedisInt(values["ok"]),
		totalLatencyMs: parseRedisInt(values["lat"]),
		ttftSumMs:      parseRedisInt(values["ttft"]),
		ttftCount:      parseRedisInt(values["ttft_n"]),
		outputTokens:   parseRedisInt(values["out"]),
		generationMs:   parseRedisInt(values["gen_ms"]),
	}
}

func parseRedisInt(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

func FlushTenant(ctx context.Context) { flushCompletedBuckets(ctx) }
