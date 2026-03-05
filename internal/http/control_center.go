package http

import (
	"net/http"

	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type ControlCenterHandler struct {
	agents   store.AgentStore
	traces   store.TracingStore
	channels store.ChannelInstanceStore
	token    string
}

func NewControlCenterHandler(agents store.AgentStore, traces store.TracingStore, channels store.ChannelInstanceStore, token string) *ControlCenterHandler {
	return &ControlCenterHandler{agents: agents, traces: traces, channels: channels, token: token}
}

func (h *ControlCenterHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/admin/control-center", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleGet))
}

func (h *ControlCenterHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	agents, err := h.agents.List(r.Context(), "")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list agents"})
		return
	}

	channelInstances, err := h.channels.ListPaged(r.Context(), store.ChannelInstanceListOpts{Limit: 500, Offset: 0})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list channel instances"})
		return
	}

	traces, err := h.traces.ListTraces(r.Context(), store.TraceListOpts{Limit: 100, Offset: 0})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list traces"})
		return
	}

	type agentItem struct {
		ID          string `json:"id"`
		AgentKey    string `json:"agent_key"`
		DisplayName string `json:"display_name,omitempty"`
		Status      string `json:"status"`
		OwnerID     string `json:"owner_id"`
		LastAction  string `json:"last_action,omitempty"`
	}
	type errorItem struct {
		ID      string `json:"id"`
		AgentID string `json:"agent_id,omitempty"`
		Name    string `json:"name,omitempty"`
		Error   string `json:"error,omitempty"`
		Created string `json:"created_at"`
	}
	type actionItem struct {
		ID      string `json:"id"`
		AgentID string `json:"agent_id,omitempty"`
		Name    string `json:"name,omitempty"`
		Status  string `json:"status"`
		Created string `json:"created_at"`
	}

	lastActionByAgent := make(map[string]string)
	errors := make([]errorItem, 0, 10)
	actions := make([]actionItem, 0, 20)
	for _, tr := range traces {
		aid := ""
		if tr.AgentID != nil {
			aid = tr.AgentID.String()
			if _, ok := lastActionByAgent[aid]; !ok {
				if tr.Name != "" {
					lastActionByAgent[aid] = tr.Name
				} else {
					lastActionByAgent[aid] = tr.Status
				}
			}
		}
		actions = append(actions, actionItem{
			ID:      tr.ID.String(),
			AgentID: aid,
			Name:    tr.Name,
			Status:  tr.Status,
			Created: tr.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
		if tr.Status == store.TraceStatusError && len(errors) < 10 {
			errors = append(errors, errorItem{
				ID:      tr.ID.String(),
				AgentID: aid,
				Name:    tr.Name,
				Error:   tr.Error,
				Created: tr.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	}

	agentItems := make([]agentItem, 0, len(agents))
	for _, ag := range agents {
		agentItems = append(agentItems, agentItem{
			ID:          ag.ID.String(),
			AgentKey:    ag.AgentKey,
			DisplayName: ag.DisplayName,
			Status:      ag.Status,
			OwnerID:     ag.OwnerID,
			LastAction:  lastActionByAgent[ag.ID.String()],
		})
	}

	enabledChannels := 0
	for _, ch := range channelInstances {
		if ch.Enabled {
			enabledChannels++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agents":           agentItems,
		"channel_total":    len(channelInstances),
		"channel_enabled":  enabledChannels,
		"errors":           errors,
		"recent_actions":   actions,
		"trace_total_used": len(traces),
	})
}
