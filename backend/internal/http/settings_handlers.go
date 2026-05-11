package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"mvp-push-gateway/backend/internal/settings"
)

type settingsResponse struct {
	Settings []settingResponse `json:"settings"`
}

type settingBody struct {
	Setting settingResponse `json:"setting"`
}

type settingResponse struct {
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	CreatedAt   string          `json:"created_at"`
	UpdatedAt   string          `json:"updated_at"`
}

func (h *Handler) settingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireSettingsService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	items, err := h.settings.ListSettings(r.Context())
	if err != nil {
		status, code, message := settingsErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := settingsResponse{Settings: make([]settingResponse, 0, len(items))}
	for _, item := range items {
		response.Settings = append(response.Settings, toSettingResponse(item))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) settingDetailHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		methodNotAllowed(w, http.MethodPut)
		return
	}
	key := singleIDFromPath(r.URL.Path, h.cfg.Server.APIPrefix+"/settings/")
	if key == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-SETTINGS-001", "系统设置不存在")
		return
	}
	if !h.requireSettingsService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	var request settings.UpdateInput
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	item, err := h.settings.UpdateSetting(r.Context(), key, request)
	if err != nil {
		status, code, message := settingsErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := settingBody{Setting: toSettingResponse(item)}
	h.recordAudit(r, adminUser, "update", "system_setting", key, request, response)
	writeJSON(w, http.StatusOK, response)
}

func toSettingResponse(item settings.Setting) settingResponse {
	return settingResponse{
		Key:         item.Key,
		Value:       defaultRawJSON(item.Value),
		Description: item.Description,
		Category:    item.Category,
		CreatedAt:   formatTime(item.CreatedAt),
		UpdatedAt:   formatTime(item.UpdatedAt),
	}
}

func settingsErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, settings.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, settings.ErrNotFound):
		return http.StatusNotFound, "MGP-SETTINGS-001", "系统设置不存在"
	default:
		return http.StatusInternalServerError, "MGP-SETTINGS-999", "系统设置服务内部错误"
	}
}
