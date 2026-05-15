package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	msgtemplate "mvp-push-gateway/backend/internal/template"
)

type templatesResponse struct {
	Templates []templateResponse `json:"templates"`
}

type templateResponse struct {
	ID                 string                   `json:"id"`
	Name               string                   `json:"name"`
	Description        string                   `json:"description"`
	SourceID           string                   `json:"source_id"`
	Enabled            bool                     `json:"enabled"`
	CurrentVersionID   string                   `json:"current_version_id"`
	MessageType        string                   `json:"message_type,omitempty"`
	TargetProviderType string                   `json:"target_provider_type,omitempty"`
	TemplateBody       string                   `json:"template_body,omitempty"`
	MessageBodySchema  json.RawMessage          `json:"message_body_schema,omitempty"`
	SamplePayload      json.RawMessage          `json:"sample_payload,omitempty"`
	CompiledPreview    json.RawMessage          `json:"compiled_preview,omitempty"`
	UsedVariables      []string                 `json:"used_variables,omitempty"`
	ValidationStatus   string                   `json:"validation_status,omitempty"`
	ValidationErrors   json.RawMessage          `json:"validation_errors,omitempty"`
	CurrentVersion     *templateVersionResponse `json:"current_version,omitempty"`
	CreatedAt          string                   `json:"created_at"`
	UpdatedAt          string                   `json:"updated_at"`
}

type templateBody struct {
	Template templateResponse `json:"template"`
}

type templateVersionBody struct {
	Version templateVersionResponse `json:"version"`
}

type templateVersionsResponse struct {
	Versions []templateVersionResponse `json:"versions"`
}

type templateVersionResponse struct {
	ID                    string          `json:"id"`
	TemplateID            string          `json:"template_id"`
	VersionNo             int             `json:"version_no"`
	MessageType           string          `json:"message_type"`
	TargetProviderType    string          `json:"target_provider_type"`
	TemplateEngine        string          `json:"template_engine"`
	TemplateSyntaxVersion string          `json:"template_syntax_version"`
	TemplateBody          string          `json:"template_body"`
	MessageBodySchema     json.RawMessage `json:"message_body_schema"`
	SamplePayload         json.RawMessage `json:"sample_payload"`
	CompiledPreview       json.RawMessage `json:"compiled_preview"`
	UsedVariables         []string        `json:"used_variables"`
	AllowedFilters        []string        `json:"allowed_filters"`
	ValidationStatus      string          `json:"validation_status"`
	ValidationErrors      json.RawMessage `json:"validation_errors"`
	PublishedAt           *string         `json:"published_at"`
	CreatedAt             string          `json:"created_at"`
	UpdatedAt             string          `json:"updated_at"`
}

type validationResponse struct {
	Result msgtemplate.ValidationResult `json:"result"`
}

func (h *Handler) templateParseHandler(w http.ResponseWriter, r *http.Request) {
	h.templateValidationAction(w, r, "parse")
}

func (h *Handler) templatePreviewHandler(w http.ResponseWriter, r *http.Request) {
	h.templateValidationAction(w, r, "preview")
}

func (h *Handler) templateValidateHandler(w http.ResponseWriter, r *http.Request) {
	h.templateValidationAction(w, r, "validate")
}

func (h *Handler) templateValidationAction(w http.ResponseWriter, r *http.Request, action string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireTemplateService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}

	var request msgtemplate.VersionInput
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}

	switch action {
	case "parse":
		result, err := h.templates.Parse(request)
		if err != nil && !errors.Is(err, msgtemplate.ErrInvalidTemplate) {
			status, code, message := templateErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		status := http.StatusOK
		if result.Status == "invalid" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, validationResponse{Result: result})
	case "preview":
		result, err := h.templates.Preview(request)
		if err != nil && !errors.Is(err, msgtemplate.ErrInvalidTemplate) {
			status, code, message := templateErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		status := http.StatusOK
		if result.Status == "invalid" {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, validationResponse{Result: result})
	default:
		result := h.templates.Validate(request)
		writeJSON(w, http.StatusOK, validationResponse{Result: result})
	}
}

func (h *Handler) templatesHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireTemplateService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.templates.ListTemplates(r.Context())
		if err != nil {
			status, code, message := templateErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := templatesResponse{Templates: make([]templateResponse, 0, len(items))}
		for _, item := range items {
			response.Templates = append(response.Templates, toTemplateResponse(item))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var request msgtemplate.TemplateInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		item, err := h.templates.CreateTemplate(r.Context(), request)
		if err != nil {
			status, code, message := templateErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := templateBody{Template: toTemplateResponse(item)}
		h.recordAudit(r, adminUser, "create", "template", item.ID, request, response)
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) templateDetailHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, h.cfg.Server.APIPrefix+"/templates/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 2 && parts[1] == "publish" {
		h.templatePublishHandler(w, r, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "versions" {
		h.templateVersionsHandler(w, r, parts[0])
		return
	}
	if len(parts) == 4 && parts[1] == "versions" && parts[3] == "restore" {
		h.templateRestoreVersionHandler(w, r, parts[0], parts[2])
		return
	}
	if len(parts) != 1 || parts[0] == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-TPL-001", "模板不存在")
		return
	}
	id := parts[0]
	if !h.requireTemplateService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := h.templates.GetTemplate(r.Context(), id)
		if err != nil {
			status, code, message := templateErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, templateBody{Template: toTemplateResponse(item)})
	case http.MethodPut:
		var request msgtemplate.TemplateInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		item, err := h.templates.UpdateTemplate(r.Context(), id, request)
		if err != nil {
			status, code, message := templateErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := templateBody{Template: toTemplateResponse(item)}
		h.recordAudit(r, adminUser, "update", "template", id, request, response)
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := h.templates.DeleteTemplate(r.Context(), id); err != nil {
			status, code, message := templateErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := okResponse{OK: true}
		h.recordAudit(r, adminUser, "delete", "template", id, nil, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (h *Handler) templatePublishHandler(w http.ResponseWriter, r *http.Request, templateID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireTemplateService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	var request msgtemplate.VersionInput
	if err := decodeJSON(r, &request); err != nil {
		writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
		return
	}
	version, err := h.templates.Publish(r.Context(), templateID, request)
	if err != nil {
		status, code, message := templateErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := templateVersionBody{Version: toTemplateVersionResponse(version)}
	h.recordAudit(r, adminUser, "publish", "template", templateID, request, response)
	writeJSON(w, http.StatusCreated, response)
}

func (h *Handler) templateVersionsHandler(w http.ResponseWriter, r *http.Request, templateID string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireTemplateService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	versions, err := h.templates.ListTemplateVersions(r.Context(), templateID)
	if err != nil {
		status, code, message := templateErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := templateVersionsResponse{Versions: make([]templateVersionResponse, 0, len(versions))}
	for _, version := range versions {
		response.Versions = append(response.Versions, toTemplateVersionResponse(version))
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) templateRestoreVersionHandler(w http.ResponseWriter, r *http.Request, templateID string, versionID string) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !h.requireTemplateService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	version, err := h.templates.RestoreTemplateVersion(r.Context(), templateID, versionID)
	if err != nil {
		status, code, message := templateErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	response := templateVersionBody{Version: toTemplateVersionResponse(version)}
	h.recordAudit(r, adminUser, "restore", "template", templateID, map[string]string{"version_id": versionID}, response)
	writeJSON(w, http.StatusCreated, response)
}

func toTemplateResponse(item msgtemplate.Template) templateResponse {
	response := templateResponse{ID: item.ID, Name: item.Name, Description: item.Description, SourceID: item.SourceID, Enabled: item.Enabled, CurrentVersionID: item.CurrentVersionID, CreatedAt: formatTime(item.CreatedAt), UpdatedAt: formatTime(item.UpdatedAt)}
	if item.CurrentVersion != nil {
		version := toTemplateVersionResponse(*item.CurrentVersion)
		response.MessageType = version.MessageType
		response.TargetProviderType = version.TargetProviderType
		response.TemplateBody = version.TemplateBody
		response.MessageBodySchema = version.MessageBodySchema
		response.SamplePayload = version.SamplePayload
		response.CompiledPreview = version.CompiledPreview
		response.UsedVariables = version.UsedVariables
		response.ValidationStatus = version.ValidationStatus
		response.ValidationErrors = version.ValidationErrors
		response.CurrentVersion = &version
	}
	return response
}

func toTemplateVersionResponse(item msgtemplate.TemplateVersion) templateVersionResponse {
	return templateVersionResponse{
		ID:                    item.ID,
		TemplateID:            item.TemplateID,
		VersionNo:             item.VersionNo,
		MessageType:           item.MessageType,
		TargetProviderType:    item.TargetProviderType,
		TemplateEngine:        item.TemplateEngine,
		TemplateSyntaxVersion: item.TemplateSyntaxVersion,
		TemplateBody:          item.TemplateBody,
		MessageBodySchema:     defaultRawJSON(item.MessageBodySchema),
		SamplePayload:         defaultRawJSON(item.SamplePayload),
		CompiledPreview:       defaultRawJSON(item.CompiledPreview),
		UsedVariables:         item.UsedVariables,
		AllowedFilters:        item.AllowedFilters,
		ValidationStatus:      item.ValidationStatus,
		ValidationErrors:      defaultRawJSON(item.ValidationErrors),
		PublishedAt:           formatOptionalTime(item.PublishedAt),
		CreatedAt:             formatTime(item.CreatedAt),
		UpdatedAt:             formatTime(item.UpdatedAt),
	}
}

func templateErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, msgtemplate.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, msgtemplate.ErrInvalidTemplate):
		return http.StatusBadRequest, "MGP-TPL-001", "模板校验失败"
	case errors.Is(err, msgtemplate.ErrNotFound):
		return http.StatusNotFound, "MGP-TPL-001", "模板不存在"
	default:
		return http.StatusInternalServerError, "MGP-TPL-999", "模板服务内部错误"
	}
}
