package settings

import (
	"context"
	"encoding/json"
	"testing"
)

func TestDefaultSettingsExposePerformanceControls(t *testing.T) {
	defaults := DefaultSettings()

	if settingValue(defaults, "ingest.max_payload_bytes") != "5242880" {
		t.Fatalf("expected 5MiB ingest payload default, got %s", settingValue(defaults, "ingest.max_payload_bytes"))
	}
	concurrency := settingByKey(defaults, "runtime.delivery_global_concurrency")
	if string(concurrency.Value) != "10" {
		t.Fatalf("expected default global delivery concurrency 10, got %s", concurrency.Value)
	}
	if concurrency.Description != "当前系统实例并发上限" {
		t.Fatalf("expected system instance concurrency wording, got %q", concurrency.Description)
	}
}

func TestRunPerformanceTestUpdatesGlobalDeliveryConcurrency(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{MessageCount: 80})
	if err != nil {
		t.Fatalf("run performance test: %v", err)
	}
	if result.RecommendedGlobalConcurrency < 1 {
		t.Fatalf("expected positive recommendation, got %+v", result)
	}
	if string(store.values["runtime.delivery_global_concurrency"]) != jsonNumber(result.RecommendedGlobalConcurrency) {
		t.Fatalf("expected runtime setting to be updated, got %s result=%+v", store.values["runtime.delivery_global_concurrency"], result)
	}
	if result.GeneratedSourceCode == "" || result.GeneratedRouteName == "" || result.GeneratedChannelName == "" {
		t.Fatalf("expected generated test resource names, got %+v", result)
	}
}

func TestRunPerformanceTestUsesObservedBenchmarkMetrics(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{
		MessageCount:          4,
		SourceCount:           2,
		PayloadVariantCount:   3,
		ConcurrencyCandidates: []int{4, 8, 16},
		Observations: []PerformanceTestObservation{
			{InboundDurationMS: 4, RouteDurationMS: 3, ReceiveDurationMS: 1, Success: true},
			{InboundDurationMS: 5, RouteDurationMS: 7, ReceiveDurationMS: 2, Success: true, SlowRuleCount: 1},
			{InboundDurationMS: 6, RouteDurationMS: 9, ReceiveDurationMS: 2, Success: true},
			{InboundDurationMS: 40, RouteDurationMS: 12, ReceiveDurationMS: 3, Success: false},
		},
	})
	if err != nil {
		t.Fatalf("run observed performance test: %v", err)
	}
	if result.SourceCount != 2 || result.PayloadVariantCount != 3 {
		t.Fatalf("expected source and payload variant counts to be preserved, got %+v", result)
	}
	if result.AcceptedCount != 3 || result.FailedCount != 1 {
		t.Fatalf("expected success/failure counts from observations, got %+v", result)
	}
	if result.SuccessRate != 75 {
		t.Fatalf("expected success rate 75, got %.2f", result.SuccessRate)
	}
	if result.RouteP99MS != 12 || result.EndToEndP99MS != 55 {
		t.Fatalf("expected p99 metrics from observations, got route=%.2f e2e=%.2f", result.RouteP99MS, result.EndToEndP99MS)
	}
	if result.SlowRuleCount != 1 {
		t.Fatalf("expected slow rule count from observations, got %+v", result)
	}
	if len(result.StageResults) < 4 {
		t.Fatalf("expected stage metrics, got %+v", result.StageResults)
	}
	if len(result.ConcurrencyResults) != 3 {
		t.Fatalf("expected candidate concurrency results, got %+v", result.ConcurrencyResults)
	}
	if result.RecommendationReason == "" {
		t.Fatalf("expected recommendation reason, got %+v", result)
	}
	if string(store.values["runtime.delivery_global_concurrency"]) != jsonNumber(result.RecommendedGlobalConcurrency) {
		t.Fatalf("expected runtime setting to be updated, got %s result=%+v", store.values["runtime.delivery_global_concurrency"], result)
	}
}

func TestRunPerformanceTestUsesP99TailMetrics(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)
	observations := make([]PerformanceTestObservation, 0, 100)
	for index := 1; index <= 100; index++ {
		observations = append(observations, PerformanceTestObservation{
			Concurrency:                  10,
			InboundDurationMS:            index,
			RouteDurationMS:              index,
			TemplateRenderDurationMS:     index,
			EndToEndDurationMS:           index,
			CompletionEndToEndDurationMS: index + 100,
			ConcurrencyRunDurationMS:     100,
			Success:                      true,
		})
	}

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{
		MessageCount:          100,
		ConcurrencyCandidates: []int{10},
		Observations:          observations,
	})
	if err != nil {
		t.Fatalf("run observed performance test: %v", err)
	}

	if result.RouteP99MS != 99 || result.EndToEndP99MS != 99 || result.CompletionEndToEndP99MS != 199 {
		t.Fatalf("expected tail metrics to use p99, got route=%.2f dispatch=%.2f completion=%.2f", result.RouteP99MS, result.EndToEndP99MS, result.CompletionEndToEndP99MS)
	}
	stages := map[string]PerformanceTestStageResult{}
	for _, item := range result.StageResults {
		stages[item.Key] = item
	}
	if stages["dispatch"].P99MS != 99 || stages["completion"].P99MS != 199 {
		t.Fatalf("expected stage tail metrics to use p99, got dispatch=%+v completion=%+v", stages["dispatch"], stages["completion"])
	}
}

func TestRunPerformanceTestSplitsAcceptedDispatchAndCompletionMetrics(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{
		MessageCount:          2,
		ConcurrencyCandidates: []int{10},
		Observations: []PerformanceTestObservation{
			{
				Concurrency:                  10,
				InboundDurationMS:            5,
				RouteDurationMS:              10,
				DispatchDurationMS:           15,
				ReceiveDurationMS:            40,
				EndToEndDurationMS:           30,
				CompletionEndToEndDurationMS: 70,
				AcceptedRunDurationMS:        20,
				DispatchRunDurationMS:        50,
				ConcurrencyRunDurationMS:     100,
				Success:                      true,
			},
			{
				Concurrency:                  10,
				InboundDurationMS:            6,
				RouteDurationMS:              12,
				DispatchDurationMS:           18,
				ReceiveDurationMS:            50,
				EndToEndDurationMS:           36,
				CompletionEndToEndDurationMS: 80,
				AcceptedRunDurationMS:        20,
				DispatchRunDurationMS:        50,
				ConcurrencyRunDurationMS:     100,
				Success:                      false,
			},
		},
	})
	if err != nil {
		t.Fatalf("run performance test with split metrics: %v", err)
	}

	if result.EstimatedAcceptedQPS != 100 {
		t.Fatalf("expected accepted qps from accepted run duration, got %+v", result)
	}
	if result.EstimatedDispatchQPS != 40 || result.EstimatedSendQPS != result.EstimatedDispatchQPS {
		t.Fatalf("expected send qps to mean dispatch qps, got %+v", result)
	}
	if result.EstimatedCompletionQPS != 20 {
		t.Fatalf("expected completion qps from full run duration, got %+v", result)
	}
	if result.EndToEndP99MS != 36 || result.CompletionEndToEndP99MS != 80 {
		t.Fatalf("expected dispatch and completion p99 split, got dispatch=%.2f completion=%.2f", result.EndToEndP99MS, result.CompletionEndToEndP99MS)
	}
	if len(result.ConcurrencyResults) != 1 {
		t.Fatalf("expected one concurrency result, got %+v", result.ConcurrencyResults)
	}
	row := result.ConcurrencyResults[0]
	if row.AcceptedQPS != 100 || row.DispatchQPS != 40 || row.SendQPS != 40 || row.CompletionQPS != 20 {
		t.Fatalf("expected split qps on concurrency row, got %+v", row)
	}
	if row.DispatchP99MS != 36 || row.CompletionP99MS != 80 {
		t.Fatalf("expected split p99 on concurrency row, got %+v", row)
	}
	rowStages := map[string]PerformanceTestStageResult{}
	for _, item := range row.StageResults {
		rowStages[item.Key] = item
	}
	if rowStages["dispatch"].P99MS != 36 || rowStages["completion"].P99MS != 80 {
		t.Fatalf("expected concurrency row stage metrics to follow its bucket, got %+v", row.StageResults)
	}
	stages := map[string]PerformanceTestStageResult{}
	for _, item := range result.StageResults {
		stages[item.Key] = item
	}
	if stages["dispatch"].Label != "出站链路" || stages["dispatch"].P99MS != 36 {
		t.Fatalf("expected dispatch stage to use outbound end-to-end p99, got %+v", stages["dispatch"])
	}
	if stages["completion"].Label != "完整链路" || stages["completion"].P99MS != 80 {
		t.Fatalf("expected completion stage to keep full p99, got %+v", stages["completion"])
	}
	if _, ok := stages["route"]; ok {
		t.Fatalf("expected duplicate route wait stage to be removed, got %+v", result.StageResults)
	}
	if _, ok := stages["receive"]; ok {
		t.Fatalf("expected duplicate receive wait stage to be removed, got %+v", result.StageResults)
	}
}

func TestRunPerformanceTestUsesConcurrencyRangeAndActualBuckets(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{
		MessageCount:        6,
		ConcurrencyRange:    "2-4",
		SourceCount:         1,
		PayloadVariantCount: 2,
		Observations: []PerformanceTestObservation{
			{Concurrency: 2, InboundDurationMS: 3, RouteDurationMS: 2, TemplateRenderDurationMS: 4, ReceiveDurationMS: 1, EndToEndDurationMS: 10, ConcurrencyRunDurationMS: 20, DBPoolWaitDurationMS: 1, Success: true},
			{Concurrency: 2, InboundDurationMS: 5, RouteDurationMS: 3, TemplateRenderDurationMS: 6, ReceiveDurationMS: 1, EndToEndDurationMS: 15, ConcurrencyRunDurationMS: 20, DBPoolWaitDurationMS: 2, Success: true},
			{Concurrency: 3, InboundDurationMS: 4, RouteDurationMS: 4, TemplateRenderDurationMS: 5, ReceiveDurationMS: 1, EndToEndDurationMS: 14, ConcurrencyRunDurationMS: 30, DBPoolWaitDurationMS: 2, Success: true},
			{Concurrency: 3, InboundDurationMS: 4, RouteDurationMS: 5, TemplateRenderDurationMS: 5, ReceiveDurationMS: 1, EndToEndDurationMS: 15, ConcurrencyRunDurationMS: 30, DBPoolWaitDurationMS: 2, Success: true},
			{Concurrency: 3, InboundDurationMS: 6, RouteDurationMS: 6, TemplateRenderDurationMS: 7, ReceiveDurationMS: 1, EndToEndDurationMS: 20, ConcurrencyRunDurationMS: 30, DBPoolWaitDurationMS: 3, Success: true},
			{Concurrency: 4, InboundDurationMS: 9, RouteDurationMS: 8, TemplateRenderDurationMS: 10, ReceiveDurationMS: 1, EndToEndDurationMS: 28, ConcurrencyRunDurationMS: 40, DBPoolWaitDurationMS: 5, Success: false},
		},
		Diagnostics: PerformanceRuntimeDiagnostics{
			DBPoolWaitCountDelta:      2,
			DBPoolWaitDurationDeltaMS: 6,
			QueueBacklogBefore:        3,
			QueueBacklogAfter:         5,
			GoroutinesBefore:          20,
			GoroutinesAfter:           22,
			GoroutineGrowthWarning:    false,
		},
	})
	if err != nil {
		t.Fatalf("run bucketed performance test: %v", err)
	}
	if result.ConcurrencyRange != "2-4" {
		t.Fatalf("expected normalized concurrency range, got %+v", result)
	}
	if len(result.ConcurrencyResults) != 3 {
		t.Fatalf("expected one real result per requested concurrency, got %+v", result.ConcurrencyResults)
	}
	if result.ConcurrencyResults[0].Concurrency != 2 || result.ConcurrencyResults[0].MessageCount != 2 {
		t.Fatalf("expected first bucket to contain concurrency 2 samples, got %+v", result.ConcurrencyResults)
	}
	if result.ConcurrencyResults[0].SendQPS != 100 {
		t.Fatalf("expected QPS from concurrency bucket wall-clock duration, got %+v", result.ConcurrencyResults[0])
	}
	if result.ConcurrencyResults[2].Concurrency != 4 || result.ConcurrencyResults[2].SuccessRate != 0 {
		t.Fatalf("expected failing concurrency bucket to be preserved, got %+v", result.ConcurrencyResults[2])
	}
	if result.Diagnostics.DBPoolWaitCountDelta != 2 ||
		result.Diagnostics.QueueBacklogDelta != 2 ||
		result.Diagnostics.GoroutinesDelta != 2 {
		t.Fatalf("expected runtime diagnostics to be summarized, got %+v", result.Diagnostics)
	}
	if !stageExists(result.StageResults, "template", "请求前模板预览") {
		t.Fatalf("expected template render stage metrics, got %+v", result.StageResults)
	}
}

func TestRunPerformanceTestUsesConcurrencyStartAndEnd(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{
		MessageCount:     1,
		ConcurrencyStart: 500,
		ConcurrencyEnd:   510,
		Observations: []PerformanceTestObservation{
			{Concurrency: 500, InboundDurationMS: 1, RouteDurationMS: 1, TemplateRenderDurationMS: 1, ReceiveDurationMS: 1, ConcurrencyRunDurationMS: 10, Success: true},
			{Concurrency: 510, InboundDurationMS: 1, RouteDurationMS: 1, TemplateRenderDurationMS: 1, ReceiveDurationMS: 1, ConcurrencyRunDurationMS: 10, Success: true},
		},
	})
	if err != nil {
		t.Fatalf("run start/end performance test: %v", err)
	}
	if result.ConcurrencyRange != "500-510" {
		t.Fatalf("expected start/end range label, got %+v", result)
	}
	if len(result.ConcurrencyResults) != 11 {
		t.Fatalf("expected 500-510 candidates, got %d %+v", len(result.ConcurrencyResults), result.ConcurrencyResults)
	}
	if result.ConcurrencyResults[0].Concurrency != 500 || result.ConcurrencyResults[10].Concurrency != 510 {
		t.Fatalf("expected candidate endpoints 500 and 510, got %+v", result.ConcurrencyResults)
	}
}

func TestRunPerformanceTestIncludesInboundStageBreakdown(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{
		MessageCount:        1,
		ConcurrencyStart:    1,
		ConcurrencyEnd:      1,
		SourceCount:         1,
		PayloadVariantCount: 1,
		Observations: []PerformanceTestObservation{{
			Concurrency:                        1,
			InboundDurationMS:                  24,
			RouteDurationMS:                    2,
			TemplateRenderDurationMS:           1,
			ReceiveDurationMS:                  1,
			EndToEndDurationMS:                 28,
			ConcurrencyRunDurationMS:           28,
			SourceLookupDurationMS:             3,
			LatestPayloadUpdateDurationMS:      4,
			EnqueueInboundDurationMS:           17,
			InsertMessageRecordDurationMS:      5,
			InsertRoutePlanJobDurationMS:       6,
			CommitInboundTransactionDurationMS: 7,
			Success:                            true,
		}},
	})
	if err != nil {
		t.Fatalf("run performance test with inbound breakdown: %v", err)
	}

	stages := map[string]PerformanceTestStageResult{}
	for _, item := range result.StageResults {
		stages[item.Key] = item
	}
	for _, expected := range []struct {
		key   string
		label string
		p99   float64
	}{
		{key: "source_lookup", label: "来源配置查询", p99: 3},
		{key: "latest_payload", label: "最近 Payload 更新", p99: 4},
		{key: "enqueue_inbound", label: "入站队列发布", p99: 17},
		{key: "insert_message_record", label: "写入消息主记录", p99: 5},
		{key: "insert_route_plan_job", label: "写入路由规划任务", p99: 6},
		{key: "commit_inbound", label: "提交入站事务", p99: 7},
	} {
		stage, ok := stages[expected.key]
		if !ok {
			t.Fatalf("expected stage %q in %+v", expected.key, result.StageResults)
		}
		if stage.Label != expected.label || stage.P99MS != expected.p99 {
			t.Fatalf("unexpected stage %q: %+v", expected.key, stage)
		}
	}
}

func TestRunPerformanceTestIncludesWorkerStageBreakdown(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{
		MessageCount:        1,
		ConcurrencyStart:    1,
		ConcurrencyEnd:      1,
		SourceCount:         1,
		PayloadVariantCount: 1,
		Observations: []PerformanceTestObservation{{
			Concurrency:                      1,
			InboundDurationMS:                10,
			RouteDurationMS:                  20,
			TemplateRenderDurationMS:         1,
			ReceiveDurationMS:                30,
			EndToEndDurationMS:               60,
			ConcurrencyRunDurationMS:         60,
			PlanningClaimDurationMS:          2,
			RoutePlanLookupDurationMS:        3,
			RouteConditionDurationMS:         4,
			PlanningTemplateRenderDurationMS: 5,
			PlanningCompleteDurationMS:       6,
			DeliveryClaimDurationMS:          7,
			DeliveryDispatchDurationMS:       8,
			DeliverySendDurationMS:           9,
			DeliveryCompleteDurationMS:       10,
			Success:                          true,
		}},
	})
	if err != nil {
		t.Fatalf("run performance test with worker breakdown: %v", err)
	}

	stages := map[string]PerformanceTestStageResult{}
	for _, item := range result.StageResults {
		stages[item.Key] = item
	}
	for _, expected := range []struct {
		key   string
		label string
		p99   float64
	}{
		{key: "planning_claim", label: "路由任务领取", p99: 2},
		{key: "route_plan_lookup", label: "路由缓存/加载", p99: 3},
		{key: "route_condition", label: "条件判断", p99: 4},
		{key: "planning_template_render", label: "规划模板渲染", p99: 5},
		{key: "planning_complete", label: "写入投递任务", p99: 6},
		{key: "delivery_claim", label: "发送任务领取", p99: 7},
		{key: "delivery_send", label: "上级请求往返", p99: 9},
		{key: "delivery_complete", label: "发送结果落库", p99: 10},
	} {
		stage, ok := stages[expected.key]
		if !ok {
			t.Fatalf("expected stage %q in %+v", expected.key, result.StageResults)
		}
		if stage.Label != expected.label || stage.P99MS != expected.p99 {
			t.Fatalf("unexpected stage %q: %+v", expected.key, stage)
		}
	}
	if _, ok := stages["delivery_dispatch"]; ok {
		t.Fatalf("expected delivery_dispatch stage to be hidden, got %+v", result.StageResults)
	}
}

func TestRunPerformanceTestIncludesDBStageBreakdown(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{
		MessageCount:     1,
		ConcurrencyStart: 1,
		ConcurrencyEnd:   1,
		Observations: []PerformanceTestObservation{{
			Concurrency:              1,
			InboundDurationMS:        10,
			RouteDurationMS:          20,
			ReceiveDurationMS:        30,
			EndToEndDurationMS:       60,
			ConcurrencyRunDurationMS: 60,
			DBTimings: map[string][]int{
				"db.acquire.claim_send_jobs":       {12},
				"db.query.claim_send_jobs":         {8},
				"db.acquire.complete_delivery":     {7},
				"db.query.complete_delivery_batch": {5},
			},
			Success: true,
		}},
	})
	if err != nil {
		t.Fatalf("run performance test with db breakdown: %v", err)
	}

	stages := map[string]PerformanceTestStageResult{}
	for _, item := range result.StageResults {
		stages[item.Key] = item
	}
	for _, expected := range []struct {
		key   string
		label string
		p99   float64
	}{
		{key: "db.acquire.claim_send_jobs", label: "DB 等待：发送任务领取", p99: 12},
		{key: "db.query.claim_send_jobs", label: "SQL 执行：发送任务领取", p99: 8},
		{key: "db.acquire.complete_delivery", label: "DB 等待：发送结果落库", p99: 7},
		{key: "db.query.complete_delivery_batch", label: "SQL 执行：批量发送结果落库", p99: 5},
	} {
		stage, ok := stages[expected.key]
		if !ok {
			t.Fatalf("expected db stage %q in %+v", expected.key, result.StageResults)
		}
		if stage.Label != expected.label || stage.P99MS != expected.p99 {
			t.Fatalf("unexpected db stage %q: %+v", expected.key, stage)
		}
	}
}

func TestRunPerformanceTestExpandsMaxConcurrencyRange(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{
		MessageCount:   5,
		MaxConcurrency: 5,
		Observations: []PerformanceTestObservation{
			{InboundDurationMS: 2, RouteDurationMS: 2, ReceiveDurationMS: 1, Success: true},
			{InboundDurationMS: 2, RouteDurationMS: 2, ReceiveDurationMS: 1, Success: true},
			{InboundDurationMS: 3, RouteDurationMS: 2, ReceiveDurationMS: 1, Success: true},
			{InboundDurationMS: 3, RouteDurationMS: 3, ReceiveDurationMS: 1, Success: true},
			{InboundDurationMS: 4, RouteDurationMS: 3, ReceiveDurationMS: 1, Success: true},
		},
	})
	if err != nil {
		t.Fatalf("run ranged performance test: %v", err)
	}
	if len(result.ConcurrencyResults) != 5 {
		t.Fatalf("expected one result per concurrency from 1 to 5, got %+v", result.ConcurrencyResults)
	}
	for index, item := range result.ConcurrencyResults {
		expected := index + 1
		if item.Concurrency != expected {
			t.Fatalf("expected concurrency result %d to be %d, got %+v", index, expected, result.ConcurrencyResults)
		}
	}
	if result.RecommendedGlobalConcurrency < 1 || result.RecommendedGlobalConcurrency > 5 {
		t.Fatalf("expected recommendation inside requested range, got %+v", result)
	}
}

func TestPerformanceMessageCountUsesRequestedConcurrencyAsSampleCount(t *testing.T) {
	count := PerformanceMessageCount(PerformanceTestInput{
		SourceCount:         1,
		PayloadVariantCount: 1,
		ConcurrencyRange:    "1-80",
	})
	if count != 80 {
		t.Fatalf("expected message count to match requested upper concurrency, got %d", count)
	}
}

func TestPerformanceMessageCountForConcurrencyEqualsConcurrency(t *testing.T) {
	count := PerformanceMessageCountForConcurrency(PerformanceTestInput{
		SourceCount:         1,
		PayloadVariantCount: 3,
	}, 1000)
	if count != 1000 {
		t.Fatalf("expected per-concurrency message count to equal requested concurrency, got %d", count)
	}
}

func TestRunPerformanceTestAutoSizesMessageCountFromBenchmarkShape(t *testing.T) {
	store := newMemorySettingsStore()
	service := NewService(store)

	result, err := service.RunPerformanceTest(context.Background(), PerformanceTestInput{
		SourceCount:         2,
		PayloadVariantCount: 3,
		MaxConcurrency:      8,
	})
	if err != nil {
		t.Fatalf("run auto-sized performance test: %v", err)
	}
	if result.MessageCount != 8 {
		t.Fatalf("expected message count to be derived from benchmark shape, got %+v", result)
	}
	if result.AcceptedCount != 8 {
		t.Fatalf("expected synthetic observations to follow derived message count, got %+v", result)
	}
}

func TestBuildPerformanceTestResultReportsActualWorkerCount(t *testing.T) {
	service := NewService(newMemorySettingsStore())

	result, err := service.BuildPerformanceTestResult(PerformanceTestInput{
		ConcurrencyStart: 8,
		ConcurrencyEnd:   8,
		Observations: []PerformanceTestObservation{
			{Concurrency: 8, Success: true, EndToEndDurationMS: 10, ConcurrencyRunDurationMS: 10},
			{Concurrency: 8, Success: true, EndToEndDurationMS: 12, ConcurrencyRunDurationMS: 10},
			{Concurrency: 8, Success: true, EndToEndDurationMS: 13, ConcurrencyRunDurationMS: 10},
		},
	})
	if err != nil {
		t.Fatalf("build performance result: %v", err)
	}
	if got := result.ConcurrencyResults[0].ActualWorkerCount; got != 3 {
		t.Fatalf("expected actual worker count to reflect available samples, got %d", got)
	}
}

func TestBuildPerformanceTestResultUsesObservedWorkerCount(t *testing.T) {
	service := NewService(newMemorySettingsStore())

	result, err := service.BuildPerformanceTestResult(PerformanceTestInput{
		ConcurrencyStart: 50,
		ConcurrencyEnd:   50,
		Observations: []PerformanceTestObservation{
			{Concurrency: 50, WorkerCount: 20, Success: true, EndToEndDurationMS: 10, ConcurrencyRunDurationMS: 10},
			{Concurrency: 50, WorkerCount: 20, Success: true, EndToEndDurationMS: 12, ConcurrencyRunDurationMS: 10},
		},
	})
	if err != nil {
		t.Fatalf("build performance result: %v", err)
	}
	if got := result.ConcurrencyResults[0].ActualWorkerCount; got != 20 {
		t.Fatalf("expected actual worker count to prefer observed worker count, got %d", got)
	}
}

func TestBuildPerformanceTestResultCarriesPostgresAndRuntimeDiagnostics(t *testing.T) {
	service := NewService(newMemorySettingsStore())

	result, err := service.BuildPerformanceTestResult(PerformanceTestInput{
		ConcurrencyStart: 1,
		ConcurrencyEnd:   1,
		Diagnostics: PerformanceRuntimeDiagnostics{
			PostgresMaxConnections: 100,
			PostgresBlocksRead:     900,
			PostgresBlocksHit:      1200,
			PostgresTempBytes:      4096,
			CPUCount:               8,
			GoMaxProcs:             4,
		},
	})
	if err != nil {
		t.Fatalf("build performance result: %v", err)
	}
	if result.Diagnostics.PostgresMaxConnections != 100 ||
		result.Diagnostics.PostgresBlocksRead != 900 ||
		result.Diagnostics.PostgresBlocksHit != 1200 ||
		result.Diagnostics.PostgresTempBytes != 4096 ||
		result.Diagnostics.CPUCount != 8 ||
		result.Diagnostics.GoMaxProcs != 4 {
		t.Fatalf("expected PostgreSQL and runtime diagnostics to survive normalization, got %+v", result.Diagnostics)
	}
}

func TestBuildPerformanceTestResultAttachesDiagnosticsToConcurrencyBuckets(t *testing.T) {
	service := NewService(newMemorySettingsStore())

	result, err := service.BuildPerformanceTestResult(PerformanceTestInput{
		ConcurrencyStart: 2,
		ConcurrencyEnd:   3,
		Observations: []PerformanceTestObservation{
			{Concurrency: 2, Success: true, InboundDurationMS: 12, RouteDurationMS: 4, TemplateRenderDurationMS: 2, ReceiveDurationMS: 1, EndToEndDurationMS: 19, ConcurrencyRunDurationMS: 20},
			{Concurrency: 3, Success: true, InboundDurationMS: 18, RouteDurationMS: 5, TemplateRenderDurationMS: 3, ReceiveDurationMS: 1, EndToEndDurationMS: 27, ConcurrencyRunDurationMS: 20},
		},
		ConcurrencyDiagnostics: []PerformanceConcurrencyDiagnostics{
			{
				Concurrency: 2,
				Diagnostics: PerformanceRuntimeDiagnostics{
					DBPoolWaitCountDelta:      4,
					DBPoolWaitDurationDeltaMS: 20,
					QueueRoutePlanBefore:      1,
					QueueRoutePlanAfter:       3,
				},
			},
			{
				Concurrency: 3,
				Diagnostics: PerformanceRuntimeDiagnostics{
					DBPoolWaitCountDelta:      9,
					DBPoolWaitDurationDeltaMS: 90,
					QueueRoutePlanBefore:      3,
					QueueRoutePlanAfter:       12,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build performance result: %v", err)
	}
	if len(result.ConcurrencyResults) != 2 {
		t.Fatalf("expected two concurrency buckets, got %+v", result.ConcurrencyResults)
	}
	if result.ConcurrencyResults[0].Diagnostics.DBPoolWaitCountDelta != 4 ||
		result.ConcurrencyResults[0].Diagnostics.QueueRoutePlanDelta != 2 ||
		result.ConcurrencyResults[1].Diagnostics.DBPoolWaitCountDelta != 9 ||
		result.ConcurrencyResults[1].Diagnostics.QueueRoutePlanDelta != 9 {
		t.Fatalf("expected per-concurrency diagnostics to be attached, got %+v", result.ConcurrencyResults)
	}
}

func TestUpdateSettingValidatesTypedValuesByKey(t *testing.T) {
	service := NewService(newMemorySettingsStore())

	cases := []struct {
		name  string
		key   string
		value json.RawMessage
	}{
		{name: "retention string", key: KeyLogsRetentionDays, value: json.RawMessage(`"30"`)},
		{name: "retention too small", key: KeyLogsRetentionDays, value: json.RawMessage(`0`)},
		{name: "payload negative", key: KeyIngestMaxPayloadBytes, value: json.RawMessage(`-1`)},
		{name: "concurrency zero", key: KeyRuntimeDeliveryConcurrency, value: json.RawMessage(`0`)},
		{name: "dead letter invalid mode", key: KeyDeadLetterProcessingMode, value: json.RawMessage(`"random"`)},
		{name: "single account non bool", key: "admin.single_account_mode", value: json.RawMessage(`"true"`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.UpdateSetting(context.Background(), tc.key, UpdateInput{Value: tc.value})
			if err == nil {
				t.Fatalf("expected invalid %s setting value to fail", tc.key)
			}
		})
	}

	if _, err := service.UpdateSetting(context.Background(), KeyRuntimeDeliveryConcurrency, UpdateInput{Value: json.RawMessage(`64`)}); err != nil {
		t.Fatalf("expected valid concurrency to be accepted: %v", err)
	}
	if _, err := service.UpdateSetting(context.Background(), KeyDeadLetterProcessingMode, UpdateInput{Value: json.RawMessage(`"auto"`)}); err != nil {
		t.Fatalf("expected valid dead letter mode to be accepted: %v", err)
	}
}

func settingValue(settings []Setting, key string) string {
	return string(settingByKey(settings, key).Value)
}

func settingByKey(settings []Setting, key string) Setting {
	for _, item := range settings {
		if item.Key == key {
			return item
		}
	}
	return Setting{}
}

func jsonNumber(value int) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

func stageExists(stages []PerformanceTestStageResult, key string, label string) bool {
	for _, item := range stages {
		if item.Key == key && item.Label == label {
			return true
		}
	}
	return false
}

type memorySettingsStore struct {
	values map[string]json.RawMessage
}

func newMemorySettingsStore() *memorySettingsStore {
	values := map[string]json.RawMessage{}
	for _, item := range DefaultSettings() {
		values[item.Key] = item.Value
	}
	return &memorySettingsStore{values: values}
}

func (m *memorySettingsStore) ListSettings(context.Context) ([]Setting, error) {
	items := make([]Setting, 0, len(m.values))
	for key, value := range m.values {
		items = append(items, Setting{Key: key, Value: value})
	}
	return items, nil
}

func (m *memorySettingsStore) GetSetting(_ context.Context, key string) (Setting, error) {
	value, ok := m.values[key]
	if !ok {
		return Setting{}, ErrNotFound
	}
	return Setting{Key: key, Value: value}, nil
}

func (m *memorySettingsStore) UpdateSetting(_ context.Context, key string, input UpdateInput) (Setting, error) {
	m.values[key] = input.Value
	return Setting{Key: key, Value: input.Value}, nil
}

func (m *memorySettingsStore) EnsureDefaultSettings(_ context.Context, defaults []Setting) error {
	for _, item := range defaults {
		m.values[item.Key] = item.Value
	}
	return nil
}
