package httpapi

import (
	"errors"
	"net/http"

	"mvp-push-gateway/backend/internal/monitoring"
)

type retentionCleanupRequest struct {
	RetentionDays int `json:"retention_days"`
	BatchSize     int `json:"batch_size"`
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

	snapshot, err := h.monitoring.GetQueueMonitoringSnapshot(r.Context())
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

	overview, err := h.stats.GetOverview(r.Context())
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, "MGP-STAT-001", "读取总览统计失败")
		return
	}
	writeJSON(w, http.StatusOK, overview)
}

func (h *Handler) retentionCleanupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireMonitoringService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
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
	writeJSON(w, http.StatusOK, status)
}
