package http

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/permissions"
	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type ControlCenterHandler struct {
	agents   store.AgentStore
	traces   store.TracingStore
	channels store.ChannelInstanceStore
	teams    store.TeamStore
	token    string
}

func NewControlCenterHandler(agents store.AgentStore, traces store.TracingStore, channels store.ChannelInstanceStore, teams store.TeamStore, token string) *ControlCenterHandler {
	return &ControlCenterHandler{
		agents:   agents,
		traces:   traces,
		channels: channels,
		teams:    teams,
		token:    token,
	}
}

func (h *ControlCenterHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/admin/control-center", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleOverview))
	mux.HandleFunc("GET /v1/admin/control-center/overview", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleOverview))
	mux.HandleFunc("GET /v1/admin/control-center/agents", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleAgents))
	mux.HandleFunc("GET /v1/admin/control-center/runs/live", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleLiveRuns))
	mux.HandleFunc("GET /v1/admin/control-center/tasks/kanban", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleKanban))
	mux.HandleFunc("POST /v1/admin/control-center/tasks/batch", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleTaskBatch))
	mux.HandleFunc("GET /v1/admin/control-center/tasks/actions", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleTaskActions))
}

type ccAgentItem struct {
	ID          string `json:"id"`
	AgentKey    string `json:"agent_key"`
	DisplayName string `json:"display_name,omitempty"`
	Status      string `json:"status"`
	OwnerID     string `json:"owner_id"`
	LastAction  string `json:"last_action,omitempty"`
}

type ccErrorItem struct {
	ID      string `json:"id"`
	AgentID string `json:"agent_id,omitempty"`
	Name    string `json:"name,omitempty"`
	Error   string `json:"error,omitempty"`
	Created string `json:"created_at"`
}

type ccActionItem struct {
	ID      string `json:"id"`
	AgentID string `json:"agent_id,omitempty"`
	Name    string `json:"name,omitempty"`
	Status  string `json:"status"`
	Created string `json:"created_at"`
}

func (h *ControlCenterHandler) loadOverviewData(r *http.Request, traceLimit int) ([]ccAgentItem, []ccErrorItem, []ccActionItem, int, int, error) {
	agents, err := h.agents.List(r.Context(), "")
	if err != nil {
		return nil, nil, nil, 0, 0, err
	}
	channelInstances, err := h.channels.ListPaged(r.Context(), store.ChannelInstanceListOpts{Limit: 500, Offset: 0})
	if err != nil {
		return nil, nil, nil, 0, 0, err
	}
	traces, err := h.traces.ListTraces(r.Context(), store.TraceListOpts{Limit: traceLimit, Offset: 0})
	if err != nil {
		return nil, nil, nil, 0, 0, err
	}

	lastActionByAgent := make(map[string]string)
	errors := make([]ccErrorItem, 0, 10)
	actions := make([]ccActionItem, 0, traceLimit)
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
		actions = append(actions, ccActionItem{
			ID:      tr.ID.String(),
			AgentID: aid,
			Name:    tr.Name,
			Status:  tr.Status,
			Created: tr.CreatedAt.Format(timeFormat),
		})
		if tr.Status == store.TraceStatusError && len(errors) < 10 {
			errors = append(errors, ccErrorItem{
				ID:      tr.ID.String(),
				AgentID: aid,
				Name:    tr.Name,
				Error:   tr.Error,
				Created: tr.CreatedAt.Format(timeFormat),
			})
		}
	}

	agentItems := make([]ccAgentItem, 0, len(agents))
	for _, ag := range agents {
		agentItems = append(agentItems, ccAgentItem{
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

	return agentItems, errors, actions, len(channelInstances), enabledChannels, nil
}

const timeFormat = "2006-01-02T15:04:05Z07:00"

func blockedEscalationThreshold() time.Duration {
	sec := 1800
	if raw := strings.TrimSpace(os.Getenv("GOCLAW_BLOCKED_ESCALATION_SEC")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			sec = n
		}
	}
	return time.Duration(sec) * time.Second
}

func (h *ControlCenterHandler) handleOverview(w http.ResponseWriter, r *http.Request) {
	agentItems, errors, actions, chTotal, chEnabled, err := h.loadOverviewData(r, 100)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load control-center overview"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agents":          agentItems,
		"channel_total":   chTotal,
		"channel_enabled": chEnabled,
		"errors":          errors,
		"recent_actions":  actions,
	})
}

func (h *ControlCenterHandler) handleAgents(w http.ResponseWriter, r *http.Request) {
	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	search := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("search")))
	status := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("status")))
	ownerID := strings.TrimSpace(r.URL.Query().Get("owner_id"))

	agentItems, _, _, _, _, err := h.loadOverviewData(r, 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load agents"})
		return
	}

	filtered := make([]ccAgentItem, 0, len(agentItems))
	for _, a := range agentItems {
		if status != "" && strings.ToLower(a.Status) != status {
			continue
		}
		if ownerID != "" && a.OwnerID != ownerID {
			continue
		}
		if search != "" {
			hay := strings.ToLower(a.AgentKey + " " + a.DisplayName + " " + a.OwnerID)
			if !strings.Contains(hay, search) {
				continue
			}
		}
		filtered = append(filtered, a)
	}

	total := len(filtered)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"agents":  filtered[offset:end],
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"filters": map[string]string{"search": search, "status": status, "owner_id": ownerID},
	})
}

func (h *ControlCenterHandler) handleLiveRuns(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	traces, err := h.traces.ListTraces(r.Context(), store.TraceListOpts{Status: store.TraceStatusRunning, Limit: limit, Offset: 0})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list live runs"})
		return
	}

	type liveRun struct {
		ID         string `json:"id"`
		AgentID    string `json:"agent_id,omitempty"`
		UserID     string `json:"user_id,omitempty"`
		SessionKey string `json:"session_key,omitempty"`
		Name       string `json:"name,omitempty"`
		Channel    string `json:"channel,omitempty"`
		Status     string `json:"status"`
		StartTime  string `json:"start_time"`
	}
	out := make([]liveRun, 0, len(traces))
	for _, tr := range traces {
		aid := ""
		if tr.AgentID != nil {
			aid = tr.AgentID.String()
		}
		out = append(out, liveRun{
			ID:         tr.ID.String(),
			AgentID:    aid,
			UserID:     tr.UserID,
			SessionKey: tr.SessionKey,
			Name:       tr.Name,
			Channel:    tr.Channel,
			Status:     tr.Status,
			StartTime:  tr.StartTime.Format(timeFormat),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"runs":  out,
		"total": len(out),
		"limit": limit,
	})
}

func (h *ControlCenterHandler) handleKanban(w http.ResponseWriter, r *http.Request) {
	if h.teams == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "team store is not available"})
		return
	}
	teamIDParam := strings.TrimSpace(r.URL.Query().Get("team_id"))

	teamIDs := make([]uuid.UUID, 0, 8)
	if teamIDParam != "" {
		tid, err := uuid.Parse(teamIDParam)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid team_id"})
			return
		}
		teamIDs = append(teamIDs, tid)
	} else {
		teams, err := h.teams.ListTeams(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list teams"})
			return
		}
		for _, t := range teams {
			teamIDs = append(teamIDs, t.ID)
		}
	}

	type taskCard struct {
		ID            string `json:"id"`
		TeamID        string `json:"team_id"`
		Subject       string `json:"subject"`
		Description   string `json:"description,omitempty"`
		Status        string `json:"status"`
		Priority      int    `json:"priority"`
		OwnerAgentID  string `json:"owner_agent_id,omitempty"`
		OwnerAgentKey string `json:"owner_agent_key,omitempty"`
		UserID        string `json:"user_id,omitempty"`
		Channel       string `json:"channel,omitempty"`
		UpdatedAt     string `json:"updated_at"`
	}
	columns := map[string][]taskCard{
		store.TeamTaskStatusPending:    {},
		store.TeamTaskStatusInProgress: {},
		store.TeamTaskStatusBlocked:    {},
		store.TeamTaskStatusCompleted:  {},
	}

	for _, tid := range teamIDs {
		tasks, err := h.teams.ListTasks(r.Context(), tid, "priority", store.TeamTaskFilterAll, "")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list team tasks"})
			return
		}
		for _, t := range tasks {
			if t.Status == store.TeamTaskStatusBlocked && t.BlockedAt != nil && t.EscalatedAt == nil {
				if time.Since(*t.BlockedAt) > blockedEscalationThreshold() {
					now := time.Now().UTC()
					_ = h.teams.UpdateTask(r.Context(), t.ID, map[string]any{
						"escalated_at":      now,
						"escalation_reason": "blocked_threshold",
					})
					_ = h.teams.AppendTaskOperatorAction(r.Context(), &store.TeamTaskOperatorActionData{
						TaskID:      t.ID,
						TeamID:      t.TeamID,
						ActorUserID: "system",
						Action:      "auto_escalate",
						Details: map[string]interface{}{
							"reason":     "blocked_threshold",
							"blocked_at": t.BlockedAt.UTC().Format(timeFormat),
						},
					})
					t.EscalatedAt = &now
					t.EscalationReason = "blocked_threshold"
				}
			}
			ownerID := ""
			if t.OwnerAgentID != nil {
				ownerID = t.OwnerAgentID.String()
			}
			card := taskCard{
				ID:            t.ID.String(),
				TeamID:        t.TeamID.String(),
				Subject:       t.Subject,
				Description:   t.Description,
				Status:        t.Status,
				Priority:      t.Priority,
				OwnerAgentID:  ownerID,
				OwnerAgentKey: t.OwnerAgentKey,
				UserID:        t.UserID,
				Channel:       t.Channel,
				UpdatedAt:     t.UpdatedAt.Format(timeFormat),
			}
			if _, ok := columns[t.Status]; !ok {
				columns[t.Status] = []taskCard{}
			}
			columns[t.Status] = append(columns[t.Status], card)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"columns": columns,
		"meta": map[string]interface{}{
			"team_count": len(teamIDs),
			"team_id":    teamIDParam,
		},
	})
}

func (h *ControlCenterHandler) handleTaskBatch(w http.ResponseWriter, r *http.Request) {
	if h.teams == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "team store is not available"})
		return
	}
	var body struct {
		Action       string   `json:"action"` // reassign | pause | escalate
		TaskIDs      []string `json:"task_ids"`
		OwnerAgentID string   `json:"owner_agent_id,omitempty"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	if len(body.TaskIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "task_ids is required"})
		return
	}
	action := strings.TrimSpace(strings.ToLower(body.Action))
	if action != "reassign" && action != "pause" && action != "escalate" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported action"})
		return
	}
	var newOwner *uuid.UUID
	if action == "reassign" {
		id, err := uuid.Parse(body.OwnerAgentID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner_agent_id is required for reassign"})
			return
		}
		newOwner = &id
	}

	updated := 0
	failed := make([]map[string]string, 0)
	for _, taskIDStr := range body.TaskIDs {
		taskID, err := uuid.Parse(taskIDStr)
		if err != nil {
			failed = append(failed, map[string]string{"task_id": taskIDStr, "error": "invalid task_id"})
			continue
		}
		task, err := h.teams.GetTask(r.Context(), taskID)
		if err != nil {
			failed = append(failed, map[string]string{"task_id": taskIDStr, "error": "task not found"})
			continue
		}
		updates := map[string]any{}
		switch action {
		case "reassign":
			updates["owner_agent_id"] = *newOwner
			updates["status"] = store.TeamTaskStatusInProgress
			updates["blocked_at"] = nil
		case "pause":
			updates["status"] = store.TeamTaskStatusBlocked
			updates["blocked_at"] = time.Now().UTC()
		case "escalate":
			updates["status"] = store.TeamTaskStatusBlocked
			updates["blocked_at"] = time.Now().UTC()
			updates["escalated_at"] = time.Now().UTC()
			updates["escalation_reason"] = "manual"
		}
		if err := h.teams.UpdateTask(r.Context(), task.ID, updates); err != nil {
			failed = append(failed, map[string]string{"task_id": taskIDStr, "error": err.Error()})
			continue
		}
		actor := store.UserIDFromContext(r.Context())
		if strings.TrimSpace(actor) == "" {
			actor = "admin"
		}
		details := map[string]interface{}{}
		if action == "reassign" {
			details["owner_agent_id"] = newOwner.String()
		}
		_ = h.teams.AppendTaskOperatorAction(r.Context(), &store.TeamTaskOperatorActionData{
			TaskID:      task.ID,
			TeamID:      task.TeamID,
			ActorUserID: actor,
			Action:      action,
			Details:     details,
		})
		updated++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"updated": updated,
		"failed":  failed,
		"action":  action,
	})
}

func (h *ControlCenterHandler) handleTaskActions(w http.ResponseWriter, r *http.Request) {
	if h.teams == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "team store is not available"})
		return
	}
	limit := 50
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	var teamID *uuid.UUID
	if raw := strings.TrimSpace(r.URL.Query().Get("team_id")); raw != "" {
		tid, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid team_id"})
			return
		}
		teamID = &tid
	}
	actions, err := h.teams.ListTaskOperatorActions(r.Context(), teamID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load task actions"})
		return
	}
	type actionItem struct {
		ID          string                 `json:"id"`
		TaskID      string                 `json:"task_id"`
		TeamID      string                 `json:"team_id"`
		ActorUserID string                 `json:"actor_user_id"`
		Action      string                 `json:"action"`
		Details     map[string]interface{} `json:"details,omitempty"`
		CreatedAt   string                 `json:"created_at"`
	}
	out := make([]actionItem, 0, len(actions))
	for _, a := range actions {
		out = append(out, actionItem{
			ID:          a.ID.String(),
			TaskID:      a.TaskID.String(),
			TeamID:      a.TeamID.String(),
			ActorUserID: a.ActorUserID,
			Action:      a.Action,
			Details:     a.Details,
			CreatedAt:   a.CreatedAt.Format(timeFormat),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"actions": out,
		"total":   len(out),
		"limit":   limit,
	})
}
