ALTER TABLE team_tasks
    ADD COLUMN IF NOT EXISTS sla_due_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS blocked_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS escalated_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS escalation_reason TEXT;

CREATE INDEX IF NOT EXISTS idx_team_tasks_blocked_overdue
    ON team_tasks (team_id, blocked_at)
    WHERE status = 'blocked' AND escalated_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_team_tasks_sla_due
    ON team_tasks (team_id, sla_due_at)
    WHERE sla_due_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS team_task_operator_actions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    task_id UUID NOT NULL REFERENCES team_tasks(id) ON DELETE CASCADE,
    team_id UUID NOT NULL REFERENCES agent_teams(id) ON DELETE CASCADE,
    actor_user_id VARCHAR(255) NOT NULL DEFAULT 'system',
    action VARCHAR(50) NOT NULL,
    details JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_operator_actions_task_created
    ON team_task_operator_actions(task_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_task_operator_actions_team_created
    ON team_task_operator_actions(team_id, created_at DESC);
