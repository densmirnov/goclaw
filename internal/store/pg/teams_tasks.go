package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"

	"github.com/nextlevelbuilder/goclaw/internal/store"
)

// ============================================================
// Tasks
// ============================================================

func (s *PGTeamStore) CreateTask(ctx context.Context, task *store.TeamTaskData) error {
	if task.ID == uuid.Nil {
		task.ID = store.GenNewID()
	}
	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now

	var err error
	if s.hasTaskSLA(ctx) {
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO team_tasks (id, team_id, subject, description, status, owner_agent_id, blocked_by, priority, result, user_id, channel, sla_due_at, blocked_at, escalated_at, escalation_reason, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
			task.ID, task.TeamID, task.Subject, task.Description,
			task.Status, task.OwnerAgentID, pq.Array(task.BlockedBy),
			task.Priority, task.Result,
			sql.NullString{String: task.UserID, Valid: task.UserID != ""},
			sql.NullString{String: task.Channel, Valid: task.Channel != ""},
			nilTime(task.SLADueAt),
			nilTime(task.BlockedAt),
			nilTime(task.EscalatedAt),
			sql.NullString{String: task.EscalationReason, Valid: task.EscalationReason != ""},
			now, now,
		)
		if isUndefinedColumnErr(err) {
			s.disableTaskSLA()
			_, err = s.db.ExecContext(ctx,
				`INSERT INTO team_tasks (id, team_id, subject, description, status, owner_agent_id, blocked_by, priority, result, user_id, channel, created_at, updated_at)
				 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
				task.ID, task.TeamID, task.Subject, task.Description,
				task.Status, task.OwnerAgentID, pq.Array(task.BlockedBy),
				task.Priority, task.Result,
				sql.NullString{String: task.UserID, Valid: task.UserID != ""},
				sql.NullString{String: task.Channel, Valid: task.Channel != ""},
				now, now,
			)
		}
	} else {
		_, err = s.db.ExecContext(ctx,
			`INSERT INTO team_tasks (id, team_id, subject, description, status, owner_agent_id, blocked_by, priority, result, user_id, channel, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			task.ID, task.TeamID, task.Subject, task.Description,
			task.Status, task.OwnerAgentID, pq.Array(task.BlockedBy),
			task.Priority, task.Result,
			sql.NullString{String: task.UserID, Valid: task.UserID != ""},
			sql.NullString{String: task.Channel, Valid: task.Channel != ""},
			now, now,
		)
	}
	return err
}

func (s *PGTeamStore) UpdateTask(ctx context.Context, taskID uuid.UUID, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	if !s.hasTaskSLA(ctx) {
		delete(updates, "sla_due_at")
		delete(updates, "blocked_at")
		delete(updates, "escalated_at")
		delete(updates, "escalation_reason")
	}
	updates["updated_at"] = time.Now()
	return execMapUpdate(ctx, s.db, "team_tasks", taskID, updates)
}

func (s *PGTeamStore) ListTasks(ctx context.Context, teamID uuid.UUID, orderBy string, statusFilter string, userID string) ([]store.TeamTaskData, error) {
	orderClause := "t.priority DESC, t.created_at"
	if orderBy == "newest" {
		orderClause = "t.created_at DESC"
	}

	statusWhere := "AND t.status != 'completed'" // default: active only
	switch statusFilter {
	case store.TeamTaskFilterAll:
		statusWhere = ""
	case store.TeamTaskFilterCompleted:
		statusWhere = "AND t.status = 'completed'"
	}

	query := `SELECT t.id, t.team_id, t.subject, t.description, t.status, t.owner_agent_id, t.blocked_by, t.priority, t.result, t.user_id, t.channel, t.created_at, t.updated_at,
		 COALESCE(a.agent_key, '') AS owner_agent_key
		 FROM team_tasks t
		 LEFT JOIN agents a ON a.id = t.owner_agent_id
		 WHERE t.team_id = $1 AND ($2 = '' OR t.user_id = $2) ` + statusWhere + `
		 ORDER BY ` + orderClause
	if s.hasTaskSLA(ctx) {
		query = `SELECT t.id, t.team_id, t.subject, t.description, t.status, t.owner_agent_id, t.blocked_by, t.priority, t.result, t.user_id, t.channel, t.sla_due_at, t.blocked_at, t.escalated_at, t.escalation_reason, t.created_at, t.updated_at,
		 COALESCE(a.agent_key, '') AS owner_agent_key
		 FROM team_tasks t
		 LEFT JOIN agents a ON a.id = t.owner_agent_id
		 WHERE t.team_id = $1 AND ($2 = '' OR t.user_id = $2) ` + statusWhere + `
		 ORDER BY ` + orderClause
	}
	if s.hasTaskSLA(ctx) {
		rows, err := s.db.QueryContext(ctx, query, teamID, userID)
		if err != nil {
			if isUndefinedColumnErr(err) {
				s.disableTaskSLA()
				return s.listTasksLegacy(ctx, teamID, orderClause, statusWhere, userID)
			}
			return nil, err
		}
		defer rows.Close()
		return scanTaskRowsJoined(rows)
	}
	return s.listTasksLegacy(ctx, teamID, orderClause, statusWhere, userID)
}

func (s *PGTeamStore) GetTask(ctx context.Context, taskID uuid.UUID) (*store.TeamTaskData, error) {
	query := `SELECT t.id, t.team_id, t.subject, t.description, t.status, t.owner_agent_id, t.blocked_by, t.priority, t.result, t.user_id, t.channel, t.created_at, t.updated_at,
		 COALESCE(a.agent_key, '') AS owner_agent_key
		 FROM team_tasks t
		 LEFT JOIN agents a ON a.id = t.owner_agent_id
		 WHERE t.id = $1`
	if s.hasTaskSLA(ctx) {
		query = `SELECT t.id, t.team_id, t.subject, t.description, t.status, t.owner_agent_id, t.blocked_by, t.priority, t.result, t.user_id, t.channel, t.sla_due_at, t.blocked_at, t.escalated_at, t.escalation_reason, t.created_at, t.updated_at,
		 COALESCE(a.agent_key, '') AS owner_agent_key
		 FROM team_tasks t
		 LEFT JOIN agents a ON a.id = t.owner_agent_id
		 WHERE t.id = $1`
	}
	rows, err := s.db.QueryContext(ctx, query, taskID)
	if err != nil {
		if s.hasTaskSLA(ctx) && isUndefinedColumnErr(err) {
			s.disableTaskSLA()
			query = `SELECT t.id, t.team_id, t.subject, t.description, t.status, t.owner_agent_id, t.blocked_by, t.priority, t.result, t.user_id, t.channel, t.created_at, t.updated_at,
		 COALESCE(a.agent_key, '') AS owner_agent_key
		 FROM team_tasks t
		 LEFT JOIN agents a ON a.id = t.owner_agent_id
		 WHERE t.id = $1`
			rows, err = s.db.QueryContext(ctx, query, taskID)
		}
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []store.TeamTaskData
	if s.hasTaskSLA(ctx) {
		tasks, err = scanTaskRowsJoined(rows)
	} else {
		tasks, err = scanTaskRowsJoinedLegacy(rows)
	}
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, fmt.Errorf("task not found")
	}
	return &tasks[0], nil
}

func (s *PGTeamStore) SearchTasks(ctx context.Context, teamID uuid.UUID, query string, limit int, userID string) ([]store.TeamTaskData, error) {
	if limit <= 0 {
		limit = 20
	}
	sqlQuery := `SELECT t.id, t.team_id, t.subject, t.description, t.status, t.owner_agent_id, t.blocked_by, t.priority, t.result, t.user_id, t.channel, t.created_at, t.updated_at,
		 COALESCE(a.agent_key, '') AS owner_agent_key
		 FROM team_tasks t
		 LEFT JOIN agents a ON a.id = t.owner_agent_id
		 WHERE t.team_id = $1 AND t.tsv @@ plainto_tsquery('simple', $2) AND ($4 = '' OR t.user_id = $4)
		 ORDER BY ts_rank(t.tsv, plainto_tsquery('simple', $2)) DESC
		 LIMIT $3`
	if s.hasTaskSLA(ctx) {
		sqlQuery = `SELECT t.id, t.team_id, t.subject, t.description, t.status, t.owner_agent_id, t.blocked_by, t.priority, t.result, t.user_id, t.channel, t.sla_due_at, t.blocked_at, t.escalated_at, t.escalation_reason, t.created_at, t.updated_at,
		 COALESCE(a.agent_key, '') AS owner_agent_key
		 FROM team_tasks t
		 LEFT JOIN agents a ON a.id = t.owner_agent_id
		 WHERE t.team_id = $1 AND t.tsv @@ plainto_tsquery('simple', $2) AND ($4 = '' OR t.user_id = $4)
		 ORDER BY ts_rank(t.tsv, plainto_tsquery('simple', $2)) DESC
		 LIMIT $3`
	}
	rows, err := s.db.QueryContext(ctx, sqlQuery, teamID, query, limit, userID)
	if err != nil {
		if s.hasTaskSLA(ctx) && isUndefinedColumnErr(err) {
			s.disableTaskSLA()
			sqlQuery = `SELECT t.id, t.team_id, t.subject, t.description, t.status, t.owner_agent_id, t.blocked_by, t.priority, t.result, t.user_id, t.channel, t.created_at, t.updated_at,
		 COALESCE(a.agent_key, '') AS owner_agent_key
		 FROM team_tasks t
		 LEFT JOIN agents a ON a.id = t.owner_agent_id
		 WHERE t.team_id = $1 AND t.tsv @@ plainto_tsquery('simple', $2) AND ($4 = '' OR t.user_id = $4)
		 ORDER BY ts_rank(t.tsv, plainto_tsquery('simple', $2)) DESC
		 LIMIT $3`
			rows, err = s.db.QueryContext(ctx, sqlQuery, teamID, query, limit, userID)
		}
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if s.hasTaskSLA(ctx) {
		return scanTaskRowsJoined(rows)
	}
	return scanTaskRowsJoinedLegacy(rows)
}

func (s *PGTeamStore) ClaimTask(ctx context.Context, taskID, agentID, teamID uuid.UUID) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE team_tasks SET status = $1, owner_agent_id = $2, updated_at = $3
		 WHERE id = $4 AND status = $5 AND owner_agent_id IS NULL AND team_id = $6`,
		store.TeamTaskStatusInProgress, agentID, time.Now(),
		taskID, store.TeamTaskStatusPending, teamID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("task not available for claiming (already claimed or not pending)")
	}
	return nil
}

func (s *PGTeamStore) CompleteTask(ctx context.Context, taskID, teamID uuid.UUID, result string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Mark task as completed (must be in_progress — use ClaimTask first)
	res, err := tx.ExecContext(ctx,
		`UPDATE team_tasks SET status = $1, result = $2, updated_at = $3
		 WHERE id = $4 AND status = $5 AND team_id = $6`,
		store.TeamTaskStatusCompleted, result, time.Now(),
		taskID, store.TeamTaskStatusInProgress, teamID,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("task not in progress or not found")
	}

	// Unblock dependent tasks: remove this taskID from their blocked_by arrays.
	// Tasks with empty blocked_by after removal become claimable.
	_, err = tx.ExecContext(ctx,
		`UPDATE team_tasks SET blocked_by = array_remove(blocked_by, $1), updated_at = $2
		 WHERE $1 = ANY(blocked_by)`,
		taskID, time.Now(),
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// CountActiveTasks returns lightweight counts for pending/in_progress tasks.
// Used by agent reminder checks to avoid loading full task rows.
func (s *PGTeamStore) CountActiveTasks(ctx context.Context, teamID uuid.UUID, userID string) (pending int, inProgress int, err error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT
			COUNT(*) FILTER (WHERE status = $3) AS pending_count,
			COUNT(*) FILTER (WHERE status = $4) AS in_progress_count
		FROM team_tasks
		WHERE team_id = $1
		  AND status != $2
		  AND ($5 = '' OR user_id = $5)`,
		teamID,
		store.TeamTaskStatusCompleted,
		store.TeamTaskStatusPending,
		store.TeamTaskStatusInProgress,
		userID,
	)
	if scanErr := row.Scan(&pending, &inProgress); scanErr != nil {
		return 0, 0, scanErr
	}
	return pending, inProgress, nil
}

func scanTaskRowsJoined(rows *sql.Rows) ([]store.TeamTaskData, error) {
	var tasks []store.TeamTaskData
	for rows.Next() {
		var d store.TeamTaskData
		var desc, result, userID, channel, escalationReason sql.NullString
		var slaDueAt, blockedAt, escalatedAt sql.NullTime
		var ownerID *uuid.UUID
		var blockedBy []uuid.UUID
		if err := rows.Scan(
			&d.ID, &d.TeamID, &d.Subject, &desc, &d.Status,
			&ownerID, pq.Array(&blockedBy), &d.Priority, &result,
			&userID, &channel,
			&slaDueAt, &blockedAt, &escalatedAt, &escalationReason,
			&d.CreatedAt, &d.UpdatedAt,
			&d.OwnerAgentKey,
		); err != nil {
			return nil, err
		}
		if desc.Valid {
			d.Description = desc.String
		}
		if result.Valid {
			d.Result = &result.String
		}
		if userID.Valid {
			d.UserID = userID.String
		}
		if channel.Valid {
			d.Channel = channel.String
		}
		if slaDueAt.Valid {
			t := slaDueAt.Time
			d.SLADueAt = &t
		}
		if blockedAt.Valid {
			t := blockedAt.Time
			d.BlockedAt = &t
		}
		if escalatedAt.Valid {
			t := escalatedAt.Time
			d.EscalatedAt = &t
		}
		if escalationReason.Valid {
			d.EscalationReason = escalationReason.String
		}
		d.OwnerAgentID = ownerID
		d.BlockedBy = blockedBy
		tasks = append(tasks, d)
	}
	return tasks, rows.Err()
}

func scanTaskRowsJoinedLegacy(rows *sql.Rows) ([]store.TeamTaskData, error) {
	var tasks []store.TeamTaskData
	for rows.Next() {
		var d store.TeamTaskData
		var desc, result, userID, channel sql.NullString
		var ownerID *uuid.UUID
		var blockedBy []uuid.UUID
		if err := rows.Scan(
			&d.ID, &d.TeamID, &d.Subject, &desc, &d.Status,
			&ownerID, pq.Array(&blockedBy), &d.Priority, &result,
			&userID, &channel,
			&d.CreatedAt, &d.UpdatedAt,
			&d.OwnerAgentKey,
		); err != nil {
			return nil, err
		}
		if desc.Valid {
			d.Description = desc.String
		}
		if result.Valid {
			d.Result = &result.String
		}
		if userID.Valid {
			d.UserID = userID.String
		}
		if channel.Valid {
			d.Channel = channel.String
		}
		d.OwnerAgentID = ownerID
		d.BlockedBy = blockedBy
		tasks = append(tasks, d)
	}
	return tasks, rows.Err()
}

func (s *PGTeamStore) AppendTaskOperatorAction(ctx context.Context, action *store.TeamTaskOperatorActionData) error {
	if !s.hasTaskOperatorActions(ctx) {
		return nil
	}
	if action.ID == uuid.Nil {
		action.ID = store.GenNewID()
	}
	if action.ActorUserID == "" {
		action.ActorUserID = "system"
	}
	details := []byte("{}")
	if len(action.Details) > 0 {
		raw, err := json.Marshal(action.Details)
		if err != nil {
			return err
		}
		details = raw
	}
	now := nowUTC()
	action.CreatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO team_task_operator_actions (id, task_id, team_id, actor_user_id, action, details, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)`,
		action.ID, action.TaskID, action.TeamID, action.ActorUserID, action.Action, details, now,
	)
	return err
}

func (s *PGTeamStore) ListTaskOperatorActions(ctx context.Context, teamID *uuid.UUID, limit int) ([]store.TeamTaskOperatorActionData, error) {
	if !s.hasTaskOperatorActions(ctx) {
		return []store.TeamTaskOperatorActionData{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows *sql.Rows
	var err error
	if teamID != nil && *teamID != uuid.Nil {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, task_id, team_id, actor_user_id, action, details, created_at
			 FROM team_task_operator_actions
			 WHERE team_id = $1
			 ORDER BY created_at DESC
			 LIMIT $2`, *teamID, limit)
	} else {
		rows, err = s.db.QueryContext(ctx,
			`SELECT id, task_id, team_id, actor_user_id, action, details, created_at
			 FROM team_task_operator_actions
			 ORDER BY created_at DESC
			 LIMIT $1`, limit)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]store.TeamTaskOperatorActionData, 0, limit)
	for rows.Next() {
		var item store.TeamTaskOperatorActionData
		var details []byte
		if err := rows.Scan(&item.ID, &item.TaskID, &item.TeamID, &item.ActorUserID, &item.Action, &details, &item.CreatedAt); err != nil {
			return nil, err
		}
		if len(details) > 0 {
			_ = json.Unmarshal(details, &item.Details)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *PGTeamStore) hasTaskSLA(ctx context.Context) bool {
	s.initTaskSchemaFlags(ctx)
	return s.hasTaskSLAColumns
}

func (s *PGTeamStore) hasTaskOperatorActions(ctx context.Context) bool {
	s.initTaskSchemaFlags(ctx)
	return s.hasTaskOperatorActionsTable
}

func (s *PGTeamStore) initTaskSchemaFlags(ctx context.Context) {
	s.schemaOnce.Do(func() {
		probeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var n int
		err := s.db.QueryRowContext(probeCtx,
			`SELECT count(*) FROM information_schema.columns
			 WHERE table_schema='public' AND table_name='team_tasks'
			   AND column_name IN ('sla_due_at','blocked_at','escalated_at','escalation_reason')`,
		).Scan(&n)
		s.hasTaskSLAColumns = err == nil && n == 4

		var t int
		err = s.db.QueryRowContext(probeCtx,
			`SELECT count(*) FROM information_schema.tables
			 WHERE table_schema='public' AND table_name='team_task_operator_actions'`,
		).Scan(&t)
		s.hasTaskOperatorActionsTable = err == nil && t == 1
	})
}

func (s *PGTeamStore) disableTaskSLA() {
	s.hasTaskSLAColumns = false
}

func isUndefinedColumnErr(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "42703"
}

func (s *PGTeamStore) listTasksLegacy(ctx context.Context, teamID uuid.UUID, orderClause, statusWhere, userID string) ([]store.TeamTaskData, error) {
	query := `SELECT t.id, t.team_id, t.subject, t.description, t.status, t.owner_agent_id, t.blocked_by, t.priority, t.result, t.user_id, t.channel, t.created_at, t.updated_at,
		 COALESCE(a.agent_key, '') AS owner_agent_key
		 FROM team_tasks t
		 LEFT JOIN agents a ON a.id = t.owner_agent_id
		 WHERE t.team_id = $1 AND ($2 = '' OR t.user_id = $2) ` + statusWhere + `
		 ORDER BY ` + orderClause
	rows, err := s.db.QueryContext(ctx, query, teamID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanTaskRowsJoinedLegacy(rows)
}
