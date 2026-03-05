-- Control Center hot-path indexes

-- traces: live runs + recent activity/error feeds
CREATE INDEX IF NOT EXISTS idx_traces_status_created_at
  ON traces (status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_traces_agent_created_at
  ON traces (agent_id, created_at DESC);

-- team_tasks: kanban and team-scoped task views
CREATE INDEX IF NOT EXISTS idx_team_tasks_team_status_priority_created
  ON team_tasks (team_id, status, priority DESC, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_team_tasks_team_updated_at
  ON team_tasks (team_id, updated_at DESC);

-- channel_instances: overview counters
CREATE INDEX IF NOT EXISTS idx_channel_instances_enabled
  ON channel_instances (enabled);

-- agents: fleet list filters
CREATE INDEX IF NOT EXISTS idx_agents_owner_status
  ON agents (owner_id, status);
