DROP INDEX IF EXISTS idx_cc_team_task_rollup_key;
DROP MATERIALIZED VIEW IF EXISTS cc_team_task_rollup;

DROP INDEX IF EXISTS idx_cc_agent_trace_rollup_agent;
DROP MATERIALIZED VIEW IF EXISTS cc_agent_trace_rollup;

DROP TABLE IF EXISTS control_center_rollup_state;
