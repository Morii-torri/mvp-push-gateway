package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"mvp-push-gateway/backend/internal/monitoring"
	"mvp-push-gateway/backend/internal/statistics"
)

type retentionCleanupRequest struct {
	RetentionDays int `json:"retention_days"`
	BatchSize     int `json:"batch_size"`
}

type notificationStreamPayload struct {
	Queue    monitoring.QueueSnapshot `json:"queue"`
	Overview statistics.Overview      `json:"overview"`
	SentAt   time.Time                `json:"sent_at"`
}

func (h *Handler) queueMonitoringHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireMonitoringService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}

	window, ok := parseWindowQuery(w, r)
	if !ok {
		return
	}
	snapshot, err := h.monitoring.GetQueueMonitoringSnapshot(r.Context(), monitoring.QueryParams{Window: window})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "MGP-MON-001", "读取队列监控失败")
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func (h *Handler) statisticsOverviewHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireStatisticsService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}

	window, ok := parseWindowQuery(w, r)
	if !ok {
		return
	}
	overview, err := h.stats.GetOverview(r.Context(), statistics.QueryParams{Window: window})
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "MGP-STAT-001", "读取总览统计失败")
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h *Handler) notificationStreamHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireMonitoringService(w) || !h.requireStatisticsService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeAPIError(w, http.StatusInternalServerError, "MGP-MON-003", "当前连接不支持实时通知")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	write := func() bool {
		queueSnapshot, err := h.monitoring.GetQueueMonitoringSnapshot(r.Context(), monitoring.QueryParams{Window: 24 * time.Hour})
		if err != nil {
			writeSSEEvent(w, "error", map[string]string{"message": "读取队列监控失败"})
			flusher.Flush()
			return true
		}
		overview, err := h.stats.GetOverview(r.Context(), statistics.QueryParams{Window: 24 * time.Hour})
		if err != nil {
			writeSSEEvent(w, "error", map[string]string{"message": "读取总览统计失败"})
			flusher.Flush()
			return true
		}
		writeSSEEvent(w, "notifications", notificationStreamPayload{
			Queue:    queueSnapshot,
			Overview: overview,
			SentAt:   time.Now().UTC(),
		})
		flusher.Flush()
		return true
	}

	if !write() {
		return
	}
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !write() {
				return
			}
		}
	}
}

func writeSSEEvent(w http.ResponseWriter, event string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		raw = []byte(`{"message":"encode event failed"}`)
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
}

func (h *Handler) retentionCleanupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireMonitoringService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}

	var request retentionCleanupRequest
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}

	status, err := h.monitoring.RunRetentionCleanup(r.Context(), monitoring.RetentionCleanupParams{
		RetentionDays: request.RetentionDays,
		BatchSize:     request.BatchSize,
	})
	if err != nil {
		if errors.Is(err, monitoring.ErrInvalidInput) {
			writeAPIError(w, http.StatusBadRequest, "MGP-MON-002", "保留期清理参数不合法")
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "MGP-MON-002", "执行保留期清理失败")
		return
	}
	h.recordAudit(r, adminUser, "run", "retention_cleanup", "manual", request, map[string]any{
		"last_batch_deleted": status.LastBatchDeleted,
		"total_deleted":      status.TotalDeleted,
		"completed":          status.Completed,
		"has_more":           status.HasMore,
	})
	writeJSON(w, http.StatusOK, status)
}

func parseWindowQuery(w http.ResponseWriter, r *http.Request) (time.Duration, bool) {
	switch r.URL.Query().Get("window") {
	case "", "24h":
		return 24 * time.Hour, true
	case "15m":
		return 15 * time.Minute, true
	case "1h":
		return time.Hour, true
	case "6h":
		return 6 * time.Hour, true
	case "7d":
		return 7 * 24 * time.Hour, true
	default:
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "时间窗口不支持")
		return 0, false
	}
}
