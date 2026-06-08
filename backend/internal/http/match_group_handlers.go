package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"mvp-push-gateway/backend/internal/matchgroup"
)

type matchGroupsResponse struct {
	MatchGroups []matchGroupResponse `json:"match_groups"`
}

type matchGroupBody struct {
	MatchGroup matchGroupResponse `json:"match_group"`
}

type matchGroupItemsResponse struct {
	Items []matchGroupItemResponse `json:"items"`
}

type matchGroupItemBody struct {
	Item matchGroupItemResponse `json:"item"`
}

type matchGroupRequest struct {
	Name        string `json:"name"`
	GroupType   string `json:"group_type"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled"`
}

type matchGroupResponse struct {
	ID             string                   `json:"id"`
	Name           string                   `json:"name"`
	GroupType      string                   `json:"group_type"`
	Description    string                   `json:"description"`
	Enabled        bool                     `json:"enabled"`
	ItemCount      int                      `json:"item_count"`
	ReferenceCount int                      `json:"reference_count"`
	Items          []matchGroupItemResponse `json:"items,omitempty"`
	CreatedAt      string                   `json:"created_at"`
	UpdatedAt      string                   `json:"updated_at"`
}

type matchGroupItemResponse struct {
	ID        string          `json:"id"`
	GroupID   string          `json:"group_id"`
	Value     string          `json:"value"`
	ValueType string          `json:"value_type"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt string          `json:"created_at"`
}

func (h *Handler) matchGroupsHandler(w http.ResponseWriter, r *http.Request) {
	if !h.requireMatchGroupService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		groups, err := h.matchGroups.ListGroups(r.Context())
		if err != nil {
			status, code, message := matchGroupErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := matchGroupsResponse{MatchGroups: make([]matchGroupResponse, 0, len(groups))}
		for _, item := range groups {
			response.MatchGroups = append(response.MatchGroups, toMatchGroupResponse(item))
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPost:
		var request matchGroupRequest
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		input := request.toInput()
		created, err := h.matchGroups.CreateGroup(r.Context(), input)
		if err != nil {
			status, code, message := matchGroupErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := matchGroupBody{MatchGroup: toMatchGroupResponse(created)}
		h.recordAudit(r, adminUser, "create", "match_group", created.ID, input, response)
		writeJSON(w, http.StatusCreated, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
	}
}

func (h *Handler) matchGroupDetailHandler(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, h.cfg.Server.APIPrefix+"/match-groups/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-MATCH-001", "匹配组不存在")
		return
	}
	groupID := parts[0]
	if len(parts) >= 2 && parts[1] == "items" {
		h.matchGroupItemsHandler(w, r, groupID, parts[2:])
		return
	}
	if len(parts) != 1 {
		writeAPIError(w, http.StatusNotFound, "MGP-MATCH-001", "匹配组不存在")
		return
	}
	if !h.requireMatchGroupService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := h.matchGroups.GetGroup(r.Context(), groupID)
		if err != nil {
			status, code, message := matchGroupErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, matchGroupBody{MatchGroup: toMatchGroupResponse(item)})
	case http.MethodPut:
		var request matchGroupRequest
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		input := request.toInput()
		updated, err := h.matchGroups.UpdateGroup(r.Context(), groupID, input)
		if err != nil {
			status, code, message := matchGroupErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := matchGroupBody{MatchGroup: toMatchGroupResponse(updated)}
		h.recordAudit(r, adminUser, "update", "match_group", groupID, input, response)
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := h.matchGroups.DeleteGroup(r.Context(), groupID); err != nil {
			status, code, message := matchGroupErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := okResponse{OK: true}
		h.recordAudit(r, adminUser, "delete", "match_group", groupID, nil, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (h *Handler) matchGroupItemsHandler(w http.ResponseWriter, r *http.Request, groupID string, parts []string) {
	if !h.requireMatchGroupService(w) {
		return
	}
	adminUser, ok := h.authenticateRequest(w, r)
	if !ok {
		return
	}
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := h.matchGroups.ListItems(r.Context(), groupID)
			if err != nil {
				status, code, message := matchGroupErrorStatus(err)
				writeAPIError(w, status, code, message)
				return
			}
			response := matchGroupItemsResponse{Items: make([]matchGroupItemResponse, 0, len(items))}
			for _, item := range items {
				response.Items = append(response.Items, toMatchGroupItemResponse(item))
			}
			writeJSON(w, http.StatusOK, response)
		case http.MethodPost:
			var request matchgroup.ItemInput
			if err := decodeJSON(r, &request); err != nil {
				writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
				return
			}
			created, err := h.matchGroups.CreateItem(r.Context(), groupID, request)
			if err != nil {
				status, code, message := matchGroupErrorStatus(err)
				writeAPIError(w, status, code, message)
				return
			}
			response := matchGroupItemBody{Item: toMatchGroupItemResponse(created)}
			h.recordAudit(r, adminUser, "create", "match_group_item", created.ID, request, response)
			writeJSON(w, http.StatusCreated, response)
		default:
			methodNotAllowed(w, http.MethodGet+", "+http.MethodPost)
		}
		return
	}
	if len(parts) != 1 || parts[0] == "" {
		writeAPIError(w, http.StatusNotFound, "MGP-MATCH-001", "匹配组条目不存在")
		return
	}
	itemID := parts[0]
	switch r.Method {
	case http.MethodGet:
		item, err := h.matchGroups.GetItem(r.Context(), groupID, itemID)
		if err != nil {
			status, code, message := matchGroupErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		writeJSON(w, http.StatusOK, matchGroupItemBody{Item: toMatchGroupItemResponse(item)})
	case http.MethodPut:
		var request matchgroup.ItemInput
		if err := decodeJSON(r, &request); err != nil {
			writeAPIError(w, http.StatusBadRequest, "MGP-REQ-001", "请求 JSON 不合法")
			return
		}
		item, err := h.matchGroups.UpdateItem(r.Context(), groupID, itemID, request)
		if err != nil {
			status, code, message := matchGroupErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := matchGroupItemBody{Item: toMatchGroupItemResponse(item)}
		h.recordAudit(r, adminUser, "update", "match_group_item", itemID, request, response)
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := h.matchGroups.DeleteItem(r.Context(), groupID, itemID); err != nil {
			status, code, message := matchGroupErrorStatus(err)
			writeAPIError(w, status, code, message)
			return
		}
		response := okResponse{OK: true}
		h.recordAudit(r, adminUser, "delete", "match_group_item", itemID, nil, response)
		writeJSON(w, http.StatusOK, response)
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (r matchGroupRequest) toInput() matchgroup.GroupInput {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return matchgroup.GroupInput{
		Name:        r.Name,
		GroupType:   r.GroupType,
		Description: r.Description,
		Enabled:     enabled,
	}
}

func toMatchGroupResponse(item matchgroup.Group) matchGroupResponse {
	items := make([]matchGroupItemResponse, 0, len(item.Items))
	for _, groupItem := range item.Items {
		items = append(items, toMatchGroupItemResponse(groupItem))
	}
	return matchGroupResponse{
		ID:             item.ID,
		Name:           item.Name,
		GroupType:      item.GroupType,
		Description:    item.Description,
		Enabled:        item.Enabled,
		ItemCount:      item.ItemCount,
		ReferenceCount: item.ReferenceCount,
		Items:          items,
		CreatedAt:      formatTime(item.CreatedAt),
		UpdatedAt:      formatTime(item.UpdatedAt),
	}
}

func toMatchGroupItemResponse(item matchgroup.Item) matchGroupItemResponse {
	return matchGroupItemResponse{
		ID:        item.ID,
		GroupID:   item.GroupID,
		Value:     item.Value,
		ValueType: item.ValueType,
		Metadata:  defaultRawJSON(item.Metadata),
		CreatedAt: formatTime(item.CreatedAt),
	}
}

func matchGroupErrorStatus(err error) (int, string, string) {
	switch {
	case errors.Is(err, matchgroup.ErrInvalidInput):
		return http.StatusBadRequest, "MGP-REQ-001", "请求参数不合法"
	case errors.Is(err, matchgroup.ErrAlreadyExists):
		return http.StatusConflict, "MGP-MATCH-001", "匹配组资源已存在"
	case errors.Is(err, matchgroup.ErrInUse):
		return http.StatusConflict, "MGP-MATCH-002", "匹配组正在被路由条件引用，不能删除"
	case errors.Is(err, matchgroup.ErrNotFound):
		return http.StatusNotFound, "MGP-MATCH-001", "匹配组资源不存在"
	default:
		return http.StatusInternalServerError, "MGP-MATCH-999", "匹配组服务内部错误"
	}
}
