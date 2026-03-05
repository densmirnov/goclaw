package http

import (
	"encoding/json"
	"net/http"
	"os"
	"sort"
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
	mux.HandleFunc("GET /v1/admin/control-center/governance", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleGovernance))
	mux.HandleFunc("GET /v1/admin/control-center/knowledge", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleKnowledge))
	mux.HandleFunc("GET /v1/admin/control-center/delegation-map", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleDelegationMap))
	mux.HandleFunc("GET /v1/admin/control-center/cost", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleCost))
	mux.HandleFunc("GET /v1/admin/control-center/health", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleHealthScore))
	mux.HandleFunc("GET /v1/admin/control-center/freshness", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleFreshness))
	mux.HandleFunc("GET /v1/admin/control-center/slo-alerts", requireRoleHTTP(h.token, permissions.RoleAdmin, h.handleSLOAlerts))
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

func estimatedUSDPer1kTokens() float64 {
	v := 0.002
	if raw := strings.TrimSpace(os.Getenv("GOCLAW_ESTIMATED_USD_PER_1K_TOKENS")); raw != "" {
		if n, err := strconv.ParseFloat(raw, 64); err == nil && n > 0 {
			v = n
		}
	}
	return v
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

func (h *ControlCenterHandler) handleGovernance(w http.ResponseWriter, r *http.Request) {
	if h.traces == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "tracing store is not available"})
		return
	}
	traces, err := h.traces.ListTraces(r.Context(), store.TraceListOpts{Limit: 300, Offset: 0})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load governance data"})
		return
	}
	accessViolations := 0
	errorRuns := 0
	for _, tr := range traces {
		if tr.Status == store.TraceStatusError {
			errorRuns++
		}
		l := strings.ToLower(tr.Error + " " + tr.Name)
		if strings.Contains(l, "forbidden") || strings.Contains(l, "unauthorized") || strings.Contains(l, "policy") {
			accessViolations++
		}
	}
	alerts := make([]map[string]interface{}, 0, 4)
	if accessViolations > 0 {
		alerts = append(alerts, map[string]interface{}{
			"type":   "access_violations",
			"count":  accessViolations,
			"level":  "high",
			"status": "open",
		})
	}
	if errorRuns > 0 {
		alerts = append(alerts, map[string]interface{}{
			"type":   "error_runs",
			"count":  errorRuns,
			"level":  "medium",
			"status": "open",
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"policy_alerts":      alerts,
		"approval_queue":     []map[string]interface{}{},
		"access_violations":  accessViolations,
		"recent_error_runs":  errorRuns,
		"sample_size_traces": len(traces),
	})
}

func (h *ControlCenterHandler) handleKnowledge(w http.ResponseWriter, r *http.Request) {
	if h.traces == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "tracing store is not available"})
		return
	}
	traces, err := h.traces.ListTraces(r.Context(), store.TraceListOpts{Limit: 200, Offset: 0})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load knowledge data"})
		return
	}
	lastByAgent := map[string]time.Time{}
	for _, tr := range traces {
		if tr.AgentID == nil {
			continue
		}
		k := tr.AgentID.String()
		if lastByAgent[k].IsZero() || tr.CreatedAt.After(lastByAgent[k]) {
			lastByAgent[k] = tr.CreatedAt
		}
	}
	stale := 0
	now := time.Now().UTC()
	for _, ts := range lastByAgent {
		if now.Sub(ts) > 7*24*time.Hour {
			stale++
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"sources": []map[string]interface{}{
			{"name": "agent_runtime_traces", "freshness_hours": 1, "status": "fresh"},
			{"name": "team_task_board", "freshness_hours": 1, "status": "fresh"},
		},
		"coverage_gaps": []map[string]interface{}{
			{"type": "stale_agents_7d", "count": stale},
		},
		"agents_with_recent_activity": len(lastByAgent) - stale,
		"agents_stale_activity":       stale,
	})
}

func (h *ControlCenterHandler) handleDelegationMap(w http.ResponseWriter, r *http.Request) {
	if h.teams == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "team store is not available"})
		return
	}
	records, _, err := h.teams.ListDelegationHistory(r.Context(), store.DelegationHistoryListOpts{Limit: 500, Offset: 0})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load delegation map"})
		return
	}
	type edge struct {
		SourceAgentID  string `json:"source_agent_id"`
		TargetAgentID  string `json:"target_agent_id"`
		SourceAgentKey string `json:"source_agent_key,omitempty"`
		TargetAgentKey string `json:"target_agent_key,omitempty"`
		Count          int    `json:"count"`
		Failures       int    `json:"failures"`
		AvgDurationMS  int    `json:"avg_duration_ms"`
	}
	m := map[string]*edge{}
	for _, r := range records {
		k := r.SourceAgentID.String() + "->" + r.TargetAgentID.String()
		item, ok := m[k]
		if !ok {
			item = &edge{
				SourceAgentID:  r.SourceAgentID.String(),
				TargetAgentID:  r.TargetAgentID.String(),
				SourceAgentKey: r.SourceAgentKey,
				TargetAgentKey: r.TargetAgentKey,
			}
			m[k] = item
		}
		item.Count++
		item.AvgDurationMS += r.DurationMS
		if r.Status == "failed" {
			item.Failures++
		}
	}
	out := make([]edge, 0, len(m))
	for _, v := range m {
		if v.Count > 0 {
			v.AvgDurationMS = v.AvgDurationMS / v.Count
		}
		out = append(out, *v)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"edges": out,
		"total": len(out),
	})
}

func (h *ControlCenterHandler) handleCost(w http.ResponseWriter, r *http.Request) {
	if h.traces == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "tracing store is not available"})
		return
	}
	traces, err := h.traces.ListTraces(r.Context(), store.TraceListOpts{Limit: 1000, Offset: 0})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load cost data"})
		return
	}
	type row struct {
		AgentID      string  `json:"agent_id"`
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		TotalTokens  int     `json:"total_tokens"`
		EstCostUSD   float64 `json:"est_cost_usd"`
	}
	perAgent := map[string]*row{}
	price := estimatedUSDPer1kTokens()
	totalTokens := 0
	for _, tr := range traces {
		aid := "unassigned"
		if tr.AgentID != nil {
			aid = tr.AgentID.String()
		}
		item, ok := perAgent[aid]
		if !ok {
			item = &row{AgentID: aid}
			perAgent[aid] = item
		}
		item.InputTokens += tr.TotalInputTokens
		item.OutputTokens += tr.TotalOutputTokens
		item.TotalTokens += tr.TotalInputTokens + tr.TotalOutputTokens
		totalTokens += tr.TotalInputTokens + tr.TotalOutputTokens
	}
	out := make([]row, 0, len(perAgent))
	for _, v := range perAgent {
		v.EstCostUSD = float64(v.TotalTokens) / 1000.0 * price
		out = append(out, *v)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rows":                   out,
		"total_tokens":           totalTokens,
		"estimated_cost_usd":     float64(totalTokens) / 1000.0 * price,
		"usd_per_1k_tokens_used": price,
	})
}

func (h *ControlCenterHandler) handleHealthScore(w http.ResponseWriter, r *http.Request) {
	score := 100
	errors := 0
	total := 0
	blockedOverdue := 0
	activeTasks := 0

	if h.traces != nil {
		if traces, err := h.traces.ListTraces(r.Context(), store.TraceListOpts{Limit: 300, Offset: 0}); err == nil {
			total = len(traces)
			for _, tr := range traces {
				if tr.Status == store.TraceStatusError {
					errors++
				}
			}
		}
	}
	if h.teams != nil {
		if teams, err := h.teams.ListTeams(r.Context()); err == nil {
			for _, tm := range teams {
				if tasks, err := h.teams.ListTasks(r.Context(), tm.ID, "priority", store.TeamTaskFilterAll, ""); err == nil {
					for _, t := range tasks {
						if t.Status == store.TeamTaskStatusCompleted {
							continue
						}
						activeTasks++
						if t.Status == store.TeamTaskStatusBlocked && t.BlockedAt != nil && time.Since(*t.BlockedAt) > blockedEscalationThreshold() {
							blockedOverdue++
						}
					}
				}
			}
		}
	}
	if total > 0 {
		errorRate := float64(errors) / float64(total)
		score -= int(errorRate * 60.0)
	}
	if activeTasks > 0 {
		stuckRate := float64(blockedOverdue) / float64(activeTasks)
		score -= int(stuckRate * 40.0)
	}
	if score < 0 {
		score = 0
	}
	status := "healthy"
	if score < 80 {
		status = "degraded"
	}
	if score < 50 {
		status = "critical"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"score":           score,
		"status":          status,
		"error_runs":      errors,
		"trace_sample":    total,
		"blocked_overdue": blockedOverdue,
		"active_tasks":    activeTasks,
	})
}

func (h *ControlCenterHandler) handleFreshness(w http.ResponseWriter, r *http.Request) {
	var lastTraceAt *time.Time
	var lastTaskAt *time.Time

	if h.traces != nil {
		if traces, err := h.traces.ListTraces(r.Context(), store.TraceListOpts{Limit: 1, Offset: 0}); err == nil && len(traces) > 0 {
			t := traces[0].CreatedAt.UTC()
			lastTraceAt = &t
		}
	}
	if h.teams != nil {
		if teams, err := h.teams.ListTeams(r.Context()); err == nil {
			for _, tm := range teams {
				if tasks, err := h.teams.ListTasks(r.Context(), tm.ID, "newest", store.TeamTaskFilterAll, ""); err == nil {
					for _, t := range tasks {
						if lastTaskAt == nil || t.UpdatedAt.After(*lastTaskAt) {
							tt := t.UpdatedAt.UTC()
							lastTaskAt = &tt
						}
					}
				}
			}
		}
	}
	now := time.Now().UTC()
	secSinceTrace := -1
	secSinceTask := -1
	if lastTraceAt != nil {
		secSinceTrace = int(now.Sub(*lastTraceAt).Seconds())
	}
	if lastTaskAt != nil {
		secSinceTask = int(now.Sub(*lastTaskAt).Seconds())
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"last_trace_at":       formatOptionalTime(lastTraceAt),
		"last_task_update_at": formatOptionalTime(lastTaskAt),
		"seconds_since_trace": secSinceTrace,
		"seconds_since_task":  secSinceTask,
	})
}

func formatOptionalTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(timeFormat)
}

func (h *ControlCenterHandler) handleSLOAlerts(w http.ResponseWriter, r *http.Request) {
	if h.traces == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "tracing store is not available"})
		return
	}
	traces, err := h.traces.ListTraces(r.Context(), store.TraceListOpts{Limit: 500, Offset: 0})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load traces for slo alerts"})
		return
	}
	if len(traces) == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"alerts": []map[string]interface{}{}})
		return
	}
	durations := make([]int, 0, len(traces))
	errors := 0
	for _, tr := range traces {
		durations = append(durations, tr.DurationMS)
		if tr.Status == store.TraceStatusError {
			errors++
		}
	}
	sort.Ints(durations)
	p95Idx := int(0.95 * float64(len(durations)-1))
	if p95Idx < 0 {
		p95Idx = 0
	}
	p95 := durations[p95Idx]
	errorRate := float64(errors) / float64(len(traces))

	maxP95MS := 5000
	if raw := strings.TrimSpace(os.Getenv("GOCLAW_SLO_P95_MS")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			maxP95MS = n
		}
	}
	maxErrorRate := 0.03
	if raw := strings.TrimSpace(os.Getenv("GOCLAW_SLO_ERROR_RATE")); raw != "" {
		if n, err := strconv.ParseFloat(raw, 64); err == nil && n > 0 {
			maxErrorRate = n
		}
	}
	alerts := make([]map[string]interface{}, 0, 3)
	if p95 > maxP95MS {
		alerts = append(alerts, map[string]interface{}{"type": "p95_latency", "value": p95, "threshold": maxP95MS, "severity": "high"})
	}
	if errorRate > maxErrorRate {
		alerts = append(alerts, map[string]interface{}{"type": "error_rate", "value": errorRate, "threshold": maxErrorRate, "severity": "high"})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts":      alerts,
		"sample_size": len(traces),
		"p95_ms":      p95,
		"error_rate":  errorRate,
		"thresholds": map[string]interface{}{
			"p95_ms":     maxP95MS,
			"error_rate": maxErrorRate,
		},
	})
}
