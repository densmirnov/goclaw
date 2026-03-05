DROP INDEX IF EXISTS idx_task_operator_actions_team_created;
DROP INDEX IF EXISTS idx_task_operator_actions_task_created;
DROP TABLE IF EXISTS team_task_operator_actions;

DROP INDEX IF EXISTS idx_team_tasks_sla_due;
DROP INDEX IF EXISTS idx_team_tasks_blocked_overdue;

ALTER TABLE team_tasks
    DROP COLUMN IF EXISTS escalation_reason,
    DROP COLUMN IF EXISTS escalated_at,
    DROP COLUMN IF EXISTS blocked_at,
    DROP COLUMN IF EXISTS sla_due_at;
