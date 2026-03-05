package http

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// MCPHandler handles MCP server management HTTP endpoints (managed mode).
type MCPHandler struct {
	store store.MCPServerStore
	token string
}

// NewMCPHandler creates a handler for MCP server management endpoints.
func NewMCPHandler(s store.MCPServerStore, token string) *MCPHandler {
	return &MCPHandler{store: s, token: token}
}

// RegisterRoutes registers all MCP management routes on the given mux.
func (h *MCPHandler) RegisterRoutes(mux *http.ServeMux) {
	// Server CRUD
	mux.HandleFunc("GET /v1/mcp/servers", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleListServers))
	mux.HandleFunc("GET /v1/mcp/servers/{id}", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleGetServer))
	mux.HandleFunc("POST /v1/mcp/servers", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleCreateServer))
	mux.HandleFunc("PUT /v1/mcp/servers/{id}", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleUpdateServer))
	mux.HandleFunc("DELETE /v1/mcp/servers/{id}", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleDeleteServer))

	// Agent grants
	mux.HandleFunc("GET /v1/mcp/grants/agent/{agentID}", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleListAgentGrants))
	mux.HandleFunc("POST /v1/mcp/servers/{id}/grants/agent", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleGrantAgent))
	mux.HandleFunc("DELETE /v1/mcp/servers/{id}/grants/agent/{agentID}", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleRevokeAgent))

	// User grants
	mux.HandleFunc("POST /v1/mcp/servers/{id}/grants/user", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleGrantUser))
	mux.HandleFunc("DELETE /v1/mcp/servers/{id}/grants/user/{userID}", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleRevokeUser))

	// Access requests
	mux.HandleFunc("POST /v1/mcp/requests", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleCreateRequest))
	mux.HandleFunc("GET /v1/mcp/requests", requireRoleHTTP(h.token, permissions.RoleOperator, h.handleListPendingRequests))
	mux.HandleFunc("POST /v1/mcp/requests/{id}/review", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleReviewRequest))
}

// --- Server CRUD ---

func (h *MCPHandler) handleListServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.store.ListServers(r.Context())
	if err != nil {
		slog.Error("mcp.list_servers", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list servers"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"servers": servers})
}

func (h *MCPHandler) handleCreateServer(w http.ResponseWriter, r *http.Request) {
	var srv store.MCPServerData
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&srv); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if srv.Name == "" || srv.Transport == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and transport are required"})
		return
	}
	if !isValidSlug(srv.Name) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name must be a valid slug (lowercase letters, numbers, hyphens only)"})
		return
	}

	userID := store.UserIDFromContext(r.Context())
	if userID != "" {
		srv.CreatedBy = userID
	}

	if err := h.store.CreateServer(r.Context(), &srv); err != nil {
		slog.Error("mcp.create_server", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, srv)
}

func (h *MCPHandler) handleGetServer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server ID"})
		return
	}

	srv, err := h.store.GetServer(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "server not found"})
		return
	}

	writeJSON(w, http.StatusOK, srv)
}

func (h *MCPHandler) handleUpdateServer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server ID"})
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&updates); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if name, ok := updates["name"]; ok {
		if s, _ := name.(string); !isValidSlug(s) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name must be a valid slug (lowercase letters, numbers, hyphens only)"})
			return
		}
	}

	if err := h.store.UpdateServer(r.Context(), id, updates); err != nil {
		slog.Error("mcp.update_server", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *MCPHandler) handleDeleteServer(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server ID"})
		return
	}

	if err := h.store.DeleteServer(r.Context(), id); err != nil {
		slog.Error("mcp.delete_server", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Agent grants ---

func (h *MCPHandler) handleGrantAgent(w http.ResponseWriter, r *http.Request) {
	serverID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server ID"})
		return
	}

	var req struct {
		AgentID   string          `json:"agent_id"`
		ToolAllow json.RawMessage `json:"tool_allow,omitempty"`
		ToolDeny  json.RawMessage `json:"tool_deny,omitempty"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent_id"})
		return
	}

	grant := store.MCPAgentGrant{
		ServerID:  serverID,
		AgentID:   agentID,
		Enabled:   true,
		ToolAllow: req.ToolAllow,
		ToolDeny:  req.ToolDeny,
		GrantedBy: store.UserIDFromContext(r.Context()),
	}

	if err := h.store.GrantToAgent(r.Context(), &grant); err != nil {
		slog.Error("mcp.grant_agent", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "granted"})
}

func (h *MCPHandler) handleRevokeAgent(w http.ResponseWriter, r *http.Request) {
	serverID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server ID"})
		return
	}

	agentID, err := uuid.Parse(r.PathValue("agentID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent ID"})
		return
	}

	if err := h.store.RevokeFromAgent(r.Context(), serverID, agentID); err != nil {
		slog.Error("mcp.revoke_agent", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func (h *MCPHandler) handleListAgentGrants(w http.ResponseWriter, r *http.Request) {
	agentID, err := uuid.Parse(r.PathValue("agentID"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent ID"})
		return
	}

	grants, err := h.store.ListAgentGrants(r.Context(), agentID)
	if err != nil {
		slog.Error("mcp.list_agent_grants", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"grants": grants})
}

// --- User grants ---

func (h *MCPHandler) handleGrantUser(w http.ResponseWriter, r *http.Request) {
	serverID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server ID"})
		return
	}

	var req struct {
		UserID    string          `json:"user_id"`
		ToolAllow json.RawMessage `json:"tool_allow,omitempty"`
		ToolDeny  json.RawMessage `json:"tool_deny,omitempty"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "user_id is required"})
		return
	}
	if err := store.ValidateUserID(req.UserID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	grant := store.MCPUserGrant{
		ServerID:  serverID,
		UserID:    req.UserID,
		Enabled:   true,
		ToolAllow: req.ToolAllow,
		ToolDeny:  req.ToolDeny,
		GrantedBy: store.UserIDFromContext(r.Context()),
	}

	if err := h.store.GrantToUser(r.Context(), &grant); err != nil {
		slog.Error("mcp.grant_user", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{"status": "granted"})
}

func (h *MCPHandler) handleRevokeUser(w http.ResponseWriter, r *http.Request) {
	serverID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server ID"})
		return
	}

	targetUserID := r.PathValue("userID")
	if err := store.ValidateUserID(targetUserID); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if err := h.store.RevokeFromUser(r.Context(), serverID, targetUserID); err != nil {
		slog.Error("mcp.revoke_user", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

// --- Access requests ---

func (h *MCPHandler) handleCreateRequest(w http.ResponseWriter, r *http.Request) {
	var req store.MCPAccessRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.ServerID == uuid.Nil || req.Scope == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "server_id and scope are required"})
		return
	}
	if req.Scope != "agent" && req.Scope != "user" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "scope must be 'agent' or 'user'"})
		return
	}

	req.RequestedBy = store.UserIDFromContext(r.Context())
	req.Status = "pending"

	if err := h.store.CreateRequest(r.Context(), &req); err != nil {
		slog.Error("mcp.create_request", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, req)
}

func (h *MCPHandler) handleListPendingRequests(w http.ResponseWriter, r *http.Request) {
	requests, err := h.store.ListPendingRequests(r.Context())
	if err != nil {
		slog.Error("mcp.list_pending_requests", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"requests": requests})
}

func (h *MCPHandler) handleReviewRequest(w http.ResponseWriter, r *http.Request) {
	requestID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request ID"})
		return
	}

	var req struct {
		Approved bool   `json:"approved"`
		Note     string `json:"note,omitempty"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	reviewedBy := store.UserIDFromContext(r.Context())

	if err := h.store.ReviewRequest(r.Context(), requestID, req.Approved, reviewedBy, req.Note); err != nil {
		slog.Error("mcp.review_request", "error", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	status := "rejected"
	if req.Approved {
		status = "approved"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status})
}
