package http

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/tools"
)

// ToolsInvokeHandler handles POST /v1/tools/invoke (direct tool invocation).
type ToolsInvokeHandler struct {
	registry   *tools.Registry
	token      string
	agentStore store.AgentStore // nil in standalone mode
}

// NewToolsInvokeHandler creates a handler for the tools invoke endpoint.
func NewToolsInvokeHandler(registry *tools.Registry, token string, agentStore store.AgentStore) *ToolsInvokeHandler {
	return &ToolsInvokeHandler{
		registry:   registry,
		token:      token,
		agentStore: agentStore,
	}
}

type toolsInvokeRequest struct {
	Tool       string                 `json:"tool"`
	Action     string                 `json:"action,omitempty"`
	Args       map[string]interface{} `json:"args"`
	SessionKey string                 `json:"sessionKey,omitempty"`
	AgentID    string                 `json:"agentId,omitempty"`
	DryRun     bool                   `json:"dryRun,omitempty"`
}

type toolsInvokeErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type toolsInvokeDryRunResponse struct {
	Tool        string                 `json:"tool"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	DryRun      bool                   `json:"dryRun"`
}

type toolsInvokeResultPayload struct {
	Output   string      `json:"output"`
	ForUser  string      `json:"forUser,omitempty"`
	Metadata interface{} `json:"metadata"`
}

type toolsInvokeResultResponse struct {
	Result toolsInvokeResultPayload `json:"result"`
}

var emptyObject = struct{}{}

func (h *ToolsInvokeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.token != "" {
		if extractBearerToken(r) != h.token {
			writeToolError(w, http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token")
			return
		}
	}

	// Prevent oversized payload allocations from untrusted callers.
	const maxRequestBodySize = 1 << 20 // 1MB
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var req toolsInvokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeToolError(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	if req.Tool == "" {
		writeToolError(w, http.StatusBadRequest, "BAD_REQUEST", "tool is required")
		return
	}

	slog.Info("tools invoke request", "tool", req.Tool, "dry_run", req.DryRun)

	if req.DryRun {
		// Just check if tool exists and return its schema
		tool, ok := h.registry.Get(req.Tool)
		if !ok {
			writeToolError(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("Tool '%s' not found", req.Tool))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(toolsInvokeDryRunResponse{
			Tool:        req.Tool,
			Description: tool.Description(),
			Parameters:  tool.Parameters(),
			DryRun:      true,
		})
		return
	}

	// Inject userID and agentID into context for interceptors (bootstrap, memory).
	ctx := r.Context()

	if userID := extractUserID(r); userID != "" {
		ctx = store.WithUserID(ctx, userID)
	}

	agentIDStr := req.AgentID
	if agentIDStr == "" {
		agentIDStr = extractAgentID(r, "")
	}
	if agentIDStr != "" && h.agentStore != nil {
		ag, err := h.agentStore.GetByKey(ctx, agentIDStr)
		if err == nil {
			ctx = store.WithAgentID(ctx, ag.ID)
		}
	}

	// Execute the tool
	args := req.Args
	if args == nil {
		args = make(map[string]interface{})
	}

	// If action is specified, add it to args
	if req.Action != "" {
		args["action"] = req.Action
	}

	result := h.registry.ExecuteWithContext(ctx, req.Tool, args, "http", "api", "direct", "", nil)

	if result.IsError {
		writeToolError(w, http.StatusBadRequest, "TOOL_ERROR", result.ForLLM)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toolsInvokeResultResponse{
		Result: toolsInvokeResultPayload{
			Output:   result.ForLLM,
			ForUser:  result.ForUser,
			Metadata: emptyObject,
		},
	})
}

func writeToolError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	var resp toolsInvokeErrorResponse
	resp.Error.Code = code
	resp.Error.Message = message
	_ = json.NewEncoder(w).Encode(resp)
}
