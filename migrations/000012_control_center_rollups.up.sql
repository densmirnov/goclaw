CREATE TABLE IF NOT EXISTS control_center_rollup_state (
    name VARCHAR(100) PRIMARY KEY,
    last_refresh_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE MATERIALIZED VIEW IF NOT EXISTS cc_agent_trace_rollup AS
SELECT
    COALESCE(agent_id::text, 'unassigned') AS agent_id,
    COUNT(*) AS run_count,
    COUNT(*) FILTER (WHERE status = 'error') AS error_count,
    COALESCE(SUM(total_input_tokens + total_output_tokens), 0) AS total_tokens,
    COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INT AS p95_duration_ms,
    MAX(created_at) AS last_run_at
FROM traces
GROUP BY COALESCE(agent_id::text, 'unassigned');

CREATE UNIQUE INDEX IF NOT EXISTS idx_cc_agent_trace_rollup_agent ON cc_agent_trace_rollup(agent_id);

CREATE MATERIALIZED VIEW IF NOT EXISTS cc_team_task_rollup AS
SELECT
    team_id,
    status,
    COUNT(*) AS task_count,
    COUNT(*) FILTER (WHERE status = 'blocked' AND escalated_at IS NOT NULL) AS escalated_count,
    MAX(updated_at) AS last_task_update_at
FROM team_tasks
GROUP BY team_id, status;

CREATE UNIQUE INDEX IF NOT EXISTS idx_cc_team_task_rollup_key ON cc_team_task_rollup(team_id, status);

INSERT INTO control_center_rollup_state (name, last_refresh_at)
VALUES
    ('cc_agent_trace_rollup', NOW()),
    ('cc_team_task_rollup', NOW())
ON CONFLICT (name) DO UPDATE SET last_refresh_at = EXCLUDED.last_refresh_at;
