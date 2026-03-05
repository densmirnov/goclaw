package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

type fakeTeamsStore struct {
	store.TeamStore
	teams   []store.TeamData
	tasks   map[uuid.UUID]*store.TeamTaskData
	actions []store.TeamTaskOperatorActionData
}

func (f *fakeTeamsStore) ListTeams(ctx context.Context) ([]store.TeamData, error) {
	return f.teams, nil
}

func (f *fakeTeamsStore) ListTasks(ctx context.Context, teamID uuid.UUID, orderBy string, statusFilter string, userID string) ([]store.TeamTaskData, error) {
	out := make([]store.TeamTaskData, 0)
	for _, t := range f.tasks {
		if t.TeamID == teamID {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (f *fakeTeamsStore) GetTask(ctx context.Context, taskID uuid.UUID) (*store.TeamTaskData, error) {
	t, ok := f.tasks[taskID]
	if !ok {
		return nil, context.Canceled
	}
	cp := *t
	return &cp, nil
}

func (f *fakeTeamsStore) UpdateTask(ctx context.Context, taskID uuid.UUID, updates map[string]any) error {
	t, ok := f.tasks[taskID]
	if !ok {
		return context.Canceled
	}
	if v, ok := updates["status"].(string); ok {
		t.Status = v
	}
	if v, ok := updates["owner_agent_id"].(uuid.UUID); ok {
		t.OwnerAgentID = &v
	}
	if _, exists := updates["blocked_at"]; exists {
		switch v := updates["blocked_at"].(type) {
		case nil:
			t.BlockedAt = nil
		case time.Time:
			vv := v
			t.BlockedAt = &vv
		}
	}
	if v, ok := updates["escalated_at"].(time.Time); ok {
		vv := v
		t.EscalatedAt = &vv
	}
	if v, ok := updates["escalation_reason"].(string); ok {
		t.EscalationReason = v
	}
	return nil
}

func (f *fakeTeamsStore) AppendTaskOperatorAction(ctx context.Context, action *store.TeamTaskOperatorActionData) error {
	if action.ID == uuid.Nil {
		action.ID = store.GenNewID()
	}
	if action.CreatedAt.IsZero() {
		action.CreatedAt = time.Now().UTC()
	}
	f.actions = append(f.actions, *action)
	return nil
}

func (f *fakeTeamsStore) ListTaskOperatorActions(ctx context.Context, teamID *uuid.UUID, limit int) ([]store.TeamTaskOperatorActionData, error) {
	if limit <= 0 || limit > len(f.actions) {
		limit = len(f.actions)
	}
	out := make([]store.TeamTaskOperatorActionData, 0, limit)
	for i := len(f.actions) - 1; i >= 0 && len(out) < limit; i-- {
		item := f.actions[i]
		if teamID != nil && item.TeamID != *teamID {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func TestControlCenter_KanbanAutoEscalatesBlockedTask(t *testing.T) {
	teamID := uuid.New()
	taskID := uuid.New()
	blockedAt := time.Now().UTC().Add(-2 * time.Hour)
	st := &fakeTeamsStore{
		teams: []store.TeamData{{BaseModel: store.BaseModel{ID: teamID}, Name: "T1"}},
		tasks: map[uuid.UUID]*store.TeamTaskData{
			taskID: {
				BaseModel: store.BaseModel{ID: taskID},
				TeamID:    teamID,
				Subject:   "stuck task",
				Status:    store.TeamTaskStatusBlocked,
				BlockedAt: &blockedAt,
			},
		},
	}

	h := NewControlCenterHandler(nil, nil, nil, st, "secret")
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/control-center/tasks/kanban", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if st.tasks[taskID].EscalatedAt == nil {
		t.Fatalf("expected task to be auto escalated")
	}
	if len(st.actions) == 0 || st.actions[len(st.actions)-1].Action != "auto_escalate" {
		t.Fatalf("expected auto_escalate action to be recorded")
	}
}

func TestControlCenter_BatchAndActionsEndpoint(t *testing.T) {
	teamID := uuid.New()
	taskID := uuid.New()
	st := &fakeTeamsStore{
		teams: []store.TeamData{{BaseModel: store.BaseModel{ID: teamID}, Name: "T1"}},
		tasks: map[uuid.UUID]*store.TeamTaskData{
			taskID: {
				BaseModel: store.BaseModel{ID: taskID},
				TeamID:    teamID,
				Subject:   "task",
				Status:    store.TeamTaskStatusPending,
			},
		},
	}

	h := NewControlCenterHandler(nil, nil, nil, st, "secret")
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)

	payload := `{"action":"pause","task_ids":["` + taskID.String() + `"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/control-center/tasks/batch", strings.NewReader(payload))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("X-GoClaw-User-Id", "ops-user-1")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("batch expected 200, got %d", rec.Code)
	}
	if st.tasks[taskID].Status != store.TeamTaskStatusBlocked {
		t.Fatalf("expected task to become blocked, got %q", st.tasks[taskID].Status)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/admin/control-center/tasks/actions?limit=10", nil)
	req2.Header.Set("Authorization", "Bearer secret")
	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("actions expected 200, got %d", rec2.Code)
	}
	var body struct {
		Actions []map[string]any `json:"actions"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode actions response: %v", err)
	}
	if len(body.Actions) == 0 {
		t.Fatalf("expected at least one action in journal")
	}
}
