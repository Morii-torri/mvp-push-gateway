package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"mvp-push-gateway/backend/internal/recipient"
)

type orgUnitsResponse struct {
	OrgUnits []orgUnitResponse `json:"org_units"`
}

type orgUnitResponse struct {
	ID        string `json:"id"`
	ParentID  string `json:"parent_id"`
	Code      string `json:"code"`
	Name      string `json:"name"`
	SortOrder int    `json:"sort_order"`
	Path      string `json:"path"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type orgUnitBody struct {
	OrgUnit orgUnitResponse `json:"org_unit"`
}

type usersResponse struct {
	Users []userResponse `json:"users"`
}

type userResponse struct {
	ID           string          `json:"id"`
	DisplayName  string          `json:"display_name"`
	PrimaryOrgID string          `json:"primary_org_id"`
	Enabled      bool            `json:"enabled"`
	Attributes   json.RawMessage `json:"attributes"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

type userBody struct {
	User userResponse `json:"user"`
}

type userIdentitiesResponse struct {
	Identities []userIdentityResponse `json:"identities"`
}

type userIdentityResponse struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	ProviderType  string `json:"provider_type"`
	IdentityKind  string `json:"identity_kind"`
	IdentityValue string `json:"identity_value"`
	Verified      bool   `json:"verified"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type userIdentityBody struct {
	Identity userIdentityResponse `json:"identity"`
}

type recipientGroupsResponse struct {
	Groups []recipientGroupResponse `json:"groups"`
}

type recipientGroupResponse struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	UserIDs         []string `json:"user_ids"`
	OrgIDs          []string `json:"org_ids"`
	ExcludedUserIDs []string `json:"excluded_user_ids"`
	ExcludedOrgIDs  []string `json:"excluded_org_ids"`
	Enabled         bool     `json:"enabled"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

type recipientGroupBody struct {
	Group recipientGroupResponse `json:"group"`
}

func (h *Handler) orgUnitsHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireRecipientService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.recipients.ListOrgUnits(r.Context())
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := orgUnitsResponse{OrgUnits: make([]orgUnitResponse, 0, len(items))}
		for _, item := range items {
			response.OrgUnits = append(response.OrgUnits, toOrgUnitResponse(item))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var request recipient.OrgUnitInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		item, err := h.recipients.CreateOrgUnit(r.Context(), request)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := orgUnitBody{OrgUnit: toOrgUnitResponse(item)}
		h.recordAudit(r, adminUser, "create", "org_unit", item.ID, request, response)
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) orgUnitDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := singleIDFromPath(r.URL.Path, h.cfg.Server.APIPrefix+"/org-units/")
	if id == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-RCP-001", "组织不存在")
		return
	}
	if !h.requireRecipientService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := h.recipients.GetOrgUnit(r.Context(), id)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, orgUnitBody{OrgUnit: toOrgUnitResponse(item)})
	case http.MethodPut:
		var request recipient.OrgUnitInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		item, err := h.recipients.UpdateOrgUnit(r.Context(), id, request)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := orgUnitBody{OrgUnit: toOrgUnitResponse(item)}
		h.recordAudit(r, adminUser, "update", "org_unit", id, request, response)
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := h.recipients.DeleteOrgUnit(r.Context(), id); err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := okResponse{OK: true}
		h.recordAudit(r, adminUser, "delete", "org_unit", id, nil, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (h *Handler) usersHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireRecipientService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		users, err := h.recipients.ListUsers(r.Context())
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := usersResponse{Users: make([]userResponse, 0, len(users))}
		for _, item := range users {
			response.Users = append(response.Users, toUserResponse(item))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var request recipient.UserInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		user, err := h.recipients.CreateUser(r.Context(), request)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := userBody{User: toUserResponse(user)}
		h.recordAudit(r, adminUser, "create", "user", user.ID, request, response)
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) userDetailHandler(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, h.cfg.Server.APIPrefix+"/users/"), "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 2 && parts[1] == "identities" {
		h.userIdentitiesHandler(w, r, parts[0])
		return
	}
	if len(parts) != 1 || parts[0] == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-RCP-001", "用户不存在")
		return
	}
	id := parts[0]
	if !h.requireRecipientService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		user, err := h.recipients.GetUser(r.Context(), id)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, userBody{User: toUserResponse(user)})
	case http.MethodPut:
		var request recipient.UserInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		user, err := h.recipients.UpdateUser(r.Context(), id, request)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := userBody{User: toUserResponse(user)}
		h.recordAudit(r, adminUser, "update", "user", id, request, response)
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := h.recipients.DeleteUser(r.Context(), id); err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := okResponse{OK: true}
		h.recordAudit(r, adminUser, "delete", "user", id, nil, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (h *Handler) userIdentitiesHandler(w http.ResponseWriter, r *http.Request, userID string) {
	if !h.requireRecipientService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.recipients.ListUserIdentities(r.Context(), userID)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := userIdentitiesResponse{Identities: make([]userIdentityResponse, 0, len(items))}
		for _, item := range items {
			response.Identities = append(response.Identities, toUserIdentityResponse(item))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var request recipient.UserIdentityInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		request.UserID = userID
		item, err := h.recipients.CreateUserIdentity(r.Context(), request)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := userIdentityBody{Identity: toUserIdentityResponse(item)}
		h.recordAudit(r, adminUser, "create", "user_identity", item.ID, request, response)
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) userIdentityLookupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !h.requireRecipientService(w) {
		return
	}
	if _, ok := h.authenticateRequest(w, r); !ok {
		return
	}
	item, err := h.recipients.FindUserIdentity(
		r.Context(),
		r.URL.Query().Get("provider_type"),
		r.URL.Query().Get("identity_kind"),
		r.URL.Query().Get("identity_value"),
	)
	if err != nil {
		status, code, message := recipientErrorStatus(err)
		writeAPIError(w, status, code, message)
		return
	}
	writeJSON(w, http.StatusOK, userIdentityBody{Identity: toUserIdentityResponse(item)})
}

func (h *Handler) userIdentityDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := singleIDFromPath(r.URL.Path, h.cfg.Server.APIPrefix+"/user-identities/")
	if id == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-RCP-001", "身份不存在")
		return
	}
	if !h.requireRecipientService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodPut:
		var request recipient.UserIdentityInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		item, err := h.recipients.UpdateUserIdentity(r.Context(), id, request)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := userIdentityBody{Identity: toUserIdentityResponse(item)}
		h.recordAudit(r, adminUser, "update", "user_identity", id, request, response)
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := h.recipients.DeleteUserIdentity(r.Context(), id); err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := okResponse{OK: true}
		h.recordAudit(r, adminUser, "delete", "user_identity", id, nil, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodPut+", "+http.MethodDelete)
	}
}

func (h *Handler) recipientGroupsHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireRecipientService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		items, err := h.recipients.ListRecipientGroups(r.Context())
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := recipientGroupsResponse{Groups: make([]recipientGroupResponse, 0, len(items))}
		for _, item := range items {
			response.Groups = append(response.Groups, toRecipientGroupResponse(item))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var request recipient.RecipientGroupInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		item, err := h.recipients.CreateRecipientGroup(r.Context(), request)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := recipientGroupBody{Group: toRecipientGroupResponse(item)}
		h.recordAudit(r, adminUser, "create", "recipient_group", item.ID, request, response)
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) recipientGroupDetailHandler(w http.ResponseWriter, r *http.Request) {
	id := singleIDFromPath(r.URL.Path, h.cfg.Server.APIPrefix+"/recipient-groups/")
	if id == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-RCP-001", "接收人组不存在")
		return
	}
	if !h.requireRecipientService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := h.recipients.GetRecipientGroup(r.Context(), id)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, recipientGroupBody{Group: toRecipientGroupResponse(item)})
	case http.MethodPut:
		var request recipient.RecipientGroupInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		item, err := h.recipients.UpdateRecipientGroup(r.Context(), id, request)
		if err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := recipientGroupBody{Group: toRecipientGroupResponse(item)}
		h.recordAudit(r, adminUser, "update", "recipient_group", id, request, response)
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := h.recipients.DeleteRecipientGroup(r.Context(), id); err != nil {
			status, code, message := recipientErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := okResponse{OK: true}
		h.recordAudit(r, adminUser, "delete", "recipient_group", id, nil, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func toOrgUnitResponse(item recipient.OrgUnit) orgUnitResponse {
	return orgUnitResponse{ID: item.ID, ParentID: item.ParentID, Code: item.Code, Name: item.Name, SortOrder: item.SortOrder, Path: item.Path, CreatedAt: formatTime(item.CreatedAt), UpdatedAt: formatTime(item.UpdatedAt)}
}

func toUserResponse(item recipient.User) userResponse {
	return userResponse{ID: item.ID, DisplayName: item.DisplayName, PrimaryOrgID: item.PrimaryOrgID, Enabled: item.Enabled, Attributes: defaultRawJSON(item.Attributes), CreatedAt: formatTime(item.CreatedAt), UpdatedAt: formatTime(item.UpdatedAt)}
}

func toUserIdentityResponse(item recipient.UserIdentity) userIdentityResponse {
	return userIdentityResponse{ID: item.ID, UserID: item.UserID, ProviderType: item.ProviderType, IdentityKind: item.IdentityKind, IdentityValue: item.IdentityValue, Verified: item.Verified, CreatedAt: formatTime(item.CreatedAt), UpdatedAt: formatTime(item.UpdatedAt)}
}

func toRecipientGroupResponse(item recipient.RecipientGroup) recipientGroupResponse {
	return recipientGroupResponse{ID: item.ID, Name: item.Name, UserIDs: item.UserIDs, OrgIDs: item.OrgIDs, ExcludedUserIDs: item.ExcludedUserIDs, ExcludedOrgIDs: item.ExcludedOrgIDs, Enabled: item.Enabled, CreatedAt: formatTime(item.CreatedAt), UpdatedAt: formatTime(item.UpdatedAt)}
}

func recipientErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, recipient.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, recipient.ErrNotFound):
		return http.StatusNotFound, "MGP-RCP-001", "接收人资源不存在"
	case errors.Is(err, recipient.ErrAlreadyExists):
		return http.StatusConflict, "MGP-RCP-001", "接收人资源已存在"
	default:
		return http.StatusInternalServerError, "MGP-RCP-999", "接收人服务内部错误"
	}
}

func singleIDFromPath(path string, prefix string) string {
	id := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if id == "" || strings.Contains(id, "/") {
		return ""
	}
	return id
}
