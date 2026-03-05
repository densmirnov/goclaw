package cmd

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/nextlevelbuilder/goclaw/internal/agent"
	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/config"
	"github.com/nextlevelbuilder/goclaw/internal/hooks"
	httpapi "github.com/nextlevelbuilder/goclaw/internal/http"
	"github.com/nextlevelbuilder/goclaw/internal/providers"
	"github.com/nextlevelbuilder/goclaw/internal/sandbox"
	"github.com/nextlevelbuilder/goclaw/internal/skills"
	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/internal/store/pg"
	"github.com/nextlevelbuilder/goclaw/internal/tools"
	"github.com/nextlevelbuilder/goclaw/internal/tracing"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

type backgroundLoopHealth struct {
	lastSuccessUnix    atomic.Int64
	consecutiveFailure atomic.Int64
}

var rollupRefresherHealth backgroundLoopHealth

// wireManagedExtras wires managed-mode components that require PG stores:
// agent resolver (lazy-creates Loops from DB), virtual FS interceptors, memory tools,
// and cache invalidation event subscribers.
// PG store creation and tracing are handled in gateway.go before this is called.
// Returns the ContextFileInterceptor so callers can pass it to AgentsMethods
// for immediate cache invalidation on agents.files.set.
func wireManagedExtras(
	stores *store.Stores,
	agentRouter *agent.Router,
	providerReg *providers.Registry,
	msgBus *bus.MessageBus,
	sessStore store.SessionStore,
	toolsReg *tools.Registry,
	toolPE *tools.PolicyEngine,
	skillsLoader *skills.Loader,
	hasMemory bool,
	traceCollector *tracing.Collector,
	workspace string,
	injectionAction string,
	appCfg *config.Config,
	sandboxMgr sandbox.Manager,
	dynamicLoader *tools.DynamicToolLoader,
) *tools.ContextFileInterceptor {
	// 1. Context file interceptor (created before resolver so callbacks can reference it)
	var contextFileInterceptor *tools.ContextFileInterceptor
	if stores.Agents != nil {
		contextFileInterceptor = tools.NewContextFileInterceptor(stores.Agents, workspace)
	}

	// 1b. Group writer cache (wraps ListGroupFileWriters with TTL cache)
	var groupWriterCache *store.GroupWriterCache
	if stores.Agents != nil {
		groupWriterCache = store.NewGroupWriterCache(stores.Agents)
	}

	// 2. User seeding callback: seeds per-user context files on first chat
	var ensureUserFiles agent.EnsureUserFilesFunc
	if stores.Agents != nil {
		ensureUserFiles = buildEnsureUserFiles(stores.Agents, msgBus)
	}

	// 3. Context file loader callback: loads per-user context files dynamically
	var contextFileLoader agent.ContextFileLoaderFunc
	if contextFileInterceptor != nil {
		contextFileLoader = buildContextFileLoader(contextFileInterceptor)
	}

	// 4. Compute global sandbox defaults for resolver
	sandboxEnabled := sandboxMgr != nil
	sandboxContainerDir := ""
	sandboxWorkspaceAccess := ""
	if sandboxEnabled {
		sbCfg := appCfg.Agents.Defaults.Sandbox
		if sbCfg != nil {
			resolved := sbCfg.ToSandboxConfig()
			sandboxContainerDir = resolved.ContainerWorkdir()
			sandboxWorkspaceAccess = string(resolved.WorkspaceAccess)
		}
	}

	// 5. Set up agent resolver: lazy-creates Loops from DB
	resolver := agent.NewManagedResolver(agent.ResolverDeps{
		AgentStore:             stores.Agents,
		ProviderReg:            providerReg,
		Bus:                    msgBus,
		Sessions:               sessStore,
		Tools:                  toolsReg,
		ToolPolicy:             toolPE,
		Skills:                 skillsLoader,
		HasMemory:              hasMemory,
		TraceCollector:         traceCollector,
		EnsureUserFiles:        ensureUserFiles,
		ContextFileLoader:      contextFileLoader,
		BootstrapCleanup:       buildBootstrapCleanup(stores.Agents),
		InjectionAction:        injectionAction,
		MaxMessageChars:        appCfg.Gateway.MaxMessageChars,
		CompactionCfg:          appCfg.Agents.Defaults.Compaction,
		ContextPruningCfg:      appCfg.Agents.Defaults.ContextPruning,
		SandboxEnabled:         sandboxEnabled,
		SandboxContainerDir:    sandboxContainerDir,
		SandboxWorkspaceAccess: sandboxWorkspaceAccess,
		DynamicLoader:          dynamicLoader,
		AgentLinkStore:         stores.AgentLinks,
		TeamStore:              stores.Teams,
		BuiltinToolStore:       stores.BuiltinTools,
		GroupWriterCache:       groupWriterCache,
		OnEvent: func(event agent.AgentEvent) {
			msgBus.Broadcast(bus.Event{
				Name:    protocol.EventAgent,
				Payload: event,
			})
			msgBus.Broadcast(bus.Event{
				Name: protocol.EventRunUpdated,
				Payload: map[string]interface{}{
					"type":     event.Type,
					"agent_id": event.AgentID,
					"run_id":   event.RunID,
				},
			})
		},
	})
	agentRouter.SetResolver(resolver)

	// Wire virtual FS interceptors: route context + memory file reads/writes to DB.
	// Share ONE ContextFileInterceptor instance between read_file and write_file
	// so they share the same cache.
	if readTool, ok := toolsReg.Get("read_file"); ok {
		if ia, ok := readTool.(tools.InterceptorAware); ok {
			if contextFileInterceptor != nil {
				ia.SetContextFileInterceptor(contextFileInterceptor)
			}
			if stores.Memory != nil {
				ia.SetMemoryInterceptor(tools.NewMemoryInterceptor(stores.Memory, workspace))
			}
		}
	}
	if writeTool, ok := toolsReg.Get("write_file"); ok {
		if ia, ok := writeTool.(tools.InterceptorAware); ok {
			if contextFileInterceptor != nil {
				ia.SetContextFileInterceptor(contextFileInterceptor)
			}
			if stores.Memory != nil {
				ia.SetMemoryInterceptor(tools.NewMemoryInterceptor(stores.Memory, workspace))
			}
		}
	}
	if editTool, ok := toolsReg.Get("edit"); ok {
		if ia, ok := editTool.(tools.InterceptorAware); ok {
			if contextFileInterceptor != nil {
				ia.SetContextFileInterceptor(contextFileInterceptor)
			}
			if stores.Memory != nil {
				ia.SetMemoryInterceptor(tools.NewMemoryInterceptor(stores.Memory, workspace))
			}
		}
	}

	// Wire group writer cache for permission checks (managed mode only)
	if groupWriterCache != nil {
		for _, toolName := range []string{"read_file", "write_file", "edit", "cron"} {
			if t, ok := toolsReg.Get(toolName); ok {
				if gwa, ok := t.(tools.GroupWriterAware); ok {
					gwa.SetGroupWriterCache(groupWriterCache)
				}
			}
		}
		if contextFileInterceptor != nil {
			contextFileInterceptor.SetGroupWriterCache(groupWriterCache)
		}
	}

	// Wire memory store on memory tools (search + get)
	if stores.Memory != nil {
		if searchTool, ok := toolsReg.Get("memory_search"); ok {
			if ms, ok := searchTool.(tools.MemoryStoreAware); ok {
				ms.SetMemoryStore(stores.Memory)
			}
		}
		if getTool, ok := toolsReg.Get("memory_get"); ok {
			if ms, ok := getTool.(tools.MemoryStoreAware); ok {
				ms.SetMemoryStore(stores.Memory)
			}
		}
		slog.Info("memory layering enabled (Postgres)")
	}

	// --- Cache invalidation event subscribers ---

	// Context file cache: invalidate on agent/context data changes
	if contextFileInterceptor != nil {
		msgBus.Subscribe(bus.TopicCacheBootstrap, func(event bus.Event) {
			if event.Name != protocol.EventCacheInvalidate {
				return
			}
			payload, ok := event.Payload.(bus.CacheInvalidatePayload)
			if !ok {
				return
			}
			if payload.Kind == bus.CacheKindBootstrap || payload.Kind == bus.CacheKindAgent {
				if payload.Key != "" {
					agentID, err := uuid.Parse(payload.Key)
					if err == nil {
						contextFileInterceptor.InvalidateAgent(agentID)
					}
				} else {
					contextFileInterceptor.InvalidateAll()
				}
			}
		})
	}

	// Agent router: invalidate Loop cache on agent config changes
	msgBus.Subscribe(bus.TopicCacheAgent, func(event bus.Event) {
		if event.Name != protocol.EventCacheInvalidate {
			return
		}
		payload, ok := event.Payload.(bus.CacheInvalidatePayload)
		if !ok || payload.Kind != bus.CacheKindAgent {
			return
		}
		if payload.Key != "" {
			agentRouter.InvalidateAgent(payload.Key)
		}
	})

	// Skills cache: bump version on skill changes
	if stores.Skills != nil {
		msgBus.Subscribe(bus.TopicCacheSkills, func(event bus.Event) {
			if event.Name != protocol.EventCacheInvalidate {
				return
			}
			payload, ok := event.Payload.(bus.CacheInvalidatePayload)
			if !ok || payload.Kind != bus.CacheKindSkills {
				return
			}
			stores.Skills.BumpVersion()
		})
	}

	// Skill grants cache: invalidate all agent caches when grants change
	msgBus.Subscribe(bus.TopicCacheSkillGrants, func(event bus.Event) {
		if event.Name != protocol.EventCacheInvalidate {
			return
		}
		payload, ok := event.Payload.(bus.CacheInvalidatePayload)
		if !ok || payload.Kind != bus.CacheKindSkillGrants {
			return
		}
		agentRouter.InvalidateAll()
	})

	// Cron cache: invalidate job cache on cron changes
	if ci, ok := stores.Cron.(store.CacheInvalidatable); ok {
		msgBus.Subscribe(bus.TopicCacheCron, func(event bus.Event) {
			if event.Name != protocol.EventCacheInvalidate {
				return
			}
			payload, ok := event.Payload.(bus.CacheInvalidatePayload)
			if !ok || payload.Kind != bus.CacheKindCron {
				return
			}
			ci.InvalidateCache()
		})
	}

	// Custom tools cache: reload global tools on create/update/delete
	if dynamicLoader != nil {
		msgBus.Subscribe(bus.TopicCacheCustomTools, func(event bus.Event) {
			if event.Name != protocol.EventCacheInvalidate {
				return
			}
			payload, ok := event.Payload.(bus.CacheInvalidatePayload)
			if !ok || payload.Kind != bus.CacheKindCustomTools {
				return
			}
			dynamicLoader.ReloadGlobal(context.Background(), toolsReg)
			// Invalidate all agent caches so they re-resolve with updated tools
			agentRouter.InvalidateAll()
		})
	}

	// Builtin tools cache: re-apply disables on settings/enabled changes
	if stores.BuiltinTools != nil {
		msgBus.Subscribe(bus.TopicCacheBuiltinTools, func(event bus.Event) {
			if event.Name != protocol.EventCacheInvalidate {
				return
			}
			payload, ok := event.Payload.(bus.CacheInvalidatePayload)
			if !ok || payload.Kind != bus.CacheKindBuiltinTools {
				return
			}
			applyBuiltinToolDisables(context.Background(), stores.BuiltinTools, toolsReg)
			agentRouter.InvalidateAll()
		})
	}

	// Register delegate tool (inter-agent delegation) if link store is available.
	// Uses a callback to bridge tools.DelegateRunRequest → agent.RunRequest,
	// avoiding import cycle between tools and agent packages.
	if stores.AgentLinks != nil && stores.Agents != nil {
		runAgentFn := func(ctx context.Context, agentKey string, req tools.DelegateRunRequest) (*tools.DelegateRunResult, error) {
			loop, err := agentRouter.Get(agentKey)
			if err != nil {
				return nil, err
			}
			result, err := loop.Run(ctx, agent.RunRequest{
				SessionKey:        req.SessionKey,
				Message:           req.Message,
				UserID:            req.UserID,
				Channel:           req.Channel,
				ChatID:            req.ChatID,
				PeerKind:          req.PeerKind,
				RunID:             req.RunID,
				Stream:            req.Stream,
				ExtraSystemPrompt: req.ExtraSystemPrompt,
				MaxIterations:     req.MaxIterations,
			})
			if err != nil {
				return nil, err
			}
			dr := &tools.DelegateRunResult{
				Content:      result.Content,
				Iterations:   result.Iterations,
				Deliverables: result.Deliverables,
			}
			for _, m := range result.Media {
				dr.MediaPaths = append(dr.MediaPaths, m.Path)
			}
			return dr, nil
		}
		delegateMgr := tools.NewDelegateManager(runAgentFn, stores.AgentLinks, stores.Agents, msgBus)
		if stores.Teams != nil {
			delegateMgr.SetTeamStore(stores.Teams)
		}
		delegateMgr.SetSessionStore(stores.Sessions)

		// Hook engine (quality gates)
		hookEngine := hooks.NewEngine()
		hookEngine.RegisterEvaluator(hooks.HookTypeCommand, hooks.NewCommandEvaluator(workspace))
		agentEvalFn := func(ctx context.Context, agentKey, task string) (string, error) {
			result, err := delegateMgr.Delegate(hooks.WithSkipHooks(ctx, true), tools.DelegateOpts{
				TargetAgentKey: agentKey, Task: task, Mode: "sync",
			})
			if err != nil {
				return "", err
			}
			return result.Content, nil
		}
		hookEngine.RegisterEvaluator(hooks.HookTypeAgent, hooks.NewAgentEvaluator(agentEvalFn))
		delegateMgr.SetHookEngine(hookEngine)

		// Evaluate-optimize loop tool
		toolsReg.Register(tools.NewEvaluateLoopTool(delegateMgr))

		// Handoff tool (agent-to-agent conversation transfer)
		toolsReg.Register(tools.NewHandoffTool(delegateMgr, stores.Teams, stores.Sessions, msgBus))

		// Inject delegation capability into existing SpawnTool
		if st, ok := toolsReg.Get("spawn"); ok {
			if spawnTool, ok := st.(*tools.SpawnTool); ok {
				spawnTool.SetDelegateManager(delegateMgr)
				slog.Info("spawn tool: delegation enabled")
			}
		}

		// Register delegate_search tool (hybrid FTS + semantic agent discovery)
		var delegateEmbProvider store.EmbeddingProvider
		if agentStore, ok := stores.Agents.(*pg.PGAgentStore); ok {
			memCfg := appCfg.Agents.Defaults.Memory
			if embProvider := resolveEmbeddingProvider(appCfg, memCfg); embProvider != nil {
				agentStore.SetEmbeddingProvider(embProvider)
				delegateEmbProvider = embProvider
				slog.Info("managed mode: agent embeddings enabled")

				// Backfill embeddings for existing agents with frontmatter
				go func() {
					count, err := agentStore.BackfillAgentEmbeddings(context.Background())
					if err != nil {
						slog.Warn("agent embeddings backfill failed", "error", err)
					} else if count > 0 {
						slog.Info("agent embeddings backfill complete", "updated", count)
					}
				}()
			}
		}
		toolsReg.Register(tools.NewDelegateSearchTool(stores.AgentLinks, delegateEmbProvider))
		slog.Info("managed mode: delegate + delegate_search tools registered")
	}

	// Register team tools (team_tasks + team_message) if team store is available.
	if stores.Teams != nil && stores.Agents != nil {
		teamMgr := tools.NewTeamToolManager(stores.Teams, stores.Agents, msgBus)
		toolsReg.Register(tools.NewTeamTasksTool(teamMgr))
		toolsReg.Register(tools.NewTeamMessageTool(teamMgr))

		// Team cache invalidation via pub/sub
		msgBus.Subscribe(bus.TopicCacheTeam, func(event bus.Event) {
			if event.Name != protocol.EventCacheInvalidate {
				return
			}
			payload, ok := event.Payload.(bus.CacheInvalidatePayload)
			if !ok || payload.Kind != bus.CacheKindTeam {
				return
			}
			teamMgr.InvalidateTeam()
		})
		slog.Info("managed mode: team tools registered")
	}

	ensureControlCenterSchema(stores.DB)
	startControlCenterRollupRefresher(stores.DB)

	// User workspace cache: invalidate per-user workspace path on profile changes
	msgBus.Subscribe(bus.TopicCacheUserWorkspace, func(event bus.Event) {
		if event.Name != protocol.EventCacheInvalidate {
			return
		}
		payload, ok := event.Payload.(bus.CacheInvalidatePayload)
		if !ok || payload.Kind != bus.CacheKindUserWorkspace {
			return
		}
		if payload.Key != "" {
			agentRouter.InvalidateUserWorkspace(payload.Key)
		}
	})

	// Group writer cache: invalidate on writer list changes
	if groupWriterCache != nil {
		msgBus.Subscribe(bus.TopicCacheGroupFileWriters, func(event bus.Event) {
			if event.Name != protocol.EventCacheInvalidate {
				return
			}
			payload, ok := event.Payload.(bus.CacheInvalidatePayload)
			if !ok || payload.Kind != bus.CacheKindGroupFileWriters {
				return
			}
			if payload.Key != "" {
				groupWriterCache.Invalidate(payload.Key)
			} else {
				groupWriterCache.InvalidateAll()
			}
		})
	}

	slog.Info("managed mode: resolver + interceptors + cache subscribers wired")
	return contextFileInterceptor
}

func ensureControlCenterSchema(db *sql.DB) {
	if db == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	stmts := []string{
		`ALTER TABLE team_tasks ADD COLUMN IF NOT EXISTS sla_due_at TIMESTAMPTZ`,
		`ALTER TABLE team_tasks ADD COLUMN IF NOT EXISTS blocked_at TIMESTAMPTZ`,
		`ALTER TABLE team_tasks ADD COLUMN IF NOT EXISTS escalated_at TIMESTAMPTZ`,
		`ALTER TABLE team_tasks ADD COLUMN IF NOT EXISTS escalation_reason TEXT`,
		`CREATE INDEX IF NOT EXISTS idx_team_tasks_blocked_overdue
			ON team_tasks (team_id, blocked_at)
			WHERE status = 'blocked' AND escalated_at IS NULL`,
		`CREATE INDEX IF NOT EXISTS idx_team_tasks_sla_due
			ON team_tasks (team_id, sla_due_at)
			WHERE sla_due_at IS NOT NULL`,
		`CREATE TABLE IF NOT EXISTS team_task_operator_actions (
			id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
			task_id UUID NOT NULL REFERENCES team_tasks(id) ON DELETE CASCADE,
			team_id UUID NOT NULL REFERENCES agent_teams(id) ON DELETE CASCADE,
			actor_user_id VARCHAR(255) NOT NULL DEFAULT 'system',
			action VARCHAR(50) NOT NULL,
			details JSONB NOT NULL DEFAULT '{}'::jsonb,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_operator_actions_task_created
			ON team_task_operator_actions(task_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_task_operator_actions_team_created
			ON team_task_operator_actions(team_id, created_at DESC)`,
		`CREATE TABLE IF NOT EXISTS control_center_rollup_state (
			name VARCHAR(100) PRIMARY KEY,
			last_refresh_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE MATERIALIZED VIEW IF NOT EXISTS cc_agent_trace_rollup AS
		 SELECT
			COALESCE(agent_id::text, 'unassigned') AS agent_id,
			COUNT(*) AS run_count,
			COUNT(*) FILTER (WHERE status = 'error') AS error_count,
			COALESCE(SUM(total_input_tokens + total_output_tokens), 0) AS total_tokens,
			COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY duration_ms), 0)::INT AS p95_duration_ms,
			MAX(created_at) AS last_run_at
		 FROM traces
		 GROUP BY COALESCE(agent_id::text, 'unassigned')`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_cc_agent_trace_rollup_agent ON cc_agent_trace_rollup(agent_id)`,
		`CREATE MATERIALIZED VIEW IF NOT EXISTS cc_team_task_rollup AS
		 SELECT
			team_id,
			status,
			COUNT(*) AS task_count,
			COUNT(*) FILTER (WHERE status = 'blocked' AND escalated_at IS NOT NULL) AS escalated_count,
			MAX(updated_at) AS last_task_update_at
		 FROM team_tasks
		 GROUP BY team_id, status`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_cc_team_task_rollup_key ON cc_team_task_rollup(team_id, status)`,
		`INSERT INTO control_center_rollup_state (name, last_refresh_at)
		 VALUES ('cc_agent_trace_rollup', NOW()), ('cc_team_task_rollup', NOW())
		 ON CONFLICT (name) DO UPDATE SET last_refresh_at = EXCLUDED.last_refresh_at`,
	}

	for _, q := range stmts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			slog.Warn("control-center schema ensure failed", "error", err)
			return
		}
	}
	slog.Info("control-center schema ensure complete")
}

func startControlCenterRollupRefresher(db *sql.DB) {
	if db == nil {
		return
	}
	intervalSec := 60
	if raw := os.Getenv("GOCLAW_ROLLUP_REFRESH_SEC"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			intervalSec = n
		}
	}
	fastFail := os.Getenv("GOCLAW_BG_FAST_FAIL") == "1" || os.Getenv("GOCLAW_BG_FAST_FAIL") == "true"
	fastFailAfter := 5
	if raw := os.Getenv("GOCLAW_ROLLUP_FAST_FAIL_AFTER"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			fastFailAfter = n
		}
	}

	go func() {
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		defer ticker.Stop()

		viewExists := func(ctx context.Context, name string) bool {
			var reg sql.NullString
			if err := db.QueryRowContext(ctx, "SELECT to_regclass($1)", name).Scan(&reg); err != nil {
				slog.Debug("rollup.exists.check_failed", "view", name, "error", err)
				return false
			}
			return reg.Valid && reg.String != ""
		}

		refresh := func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var failures int
			var refreshed int
			if viewExists(ctx, "cc_agent_trace_rollup") {
				if _, err := db.ExecContext(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY cc_agent_trace_rollup"); err != nil {
					slog.Debug("rollup.refresh.failed", "view", "cc_agent_trace_rollup", "error", err)
					failures++
				} else {
					_, _ = db.ExecContext(ctx, "INSERT INTO control_center_rollup_state(name,last_refresh_at) VALUES('cc_agent_trace_rollup',NOW()) ON CONFLICT(name) DO UPDATE SET last_refresh_at=EXCLUDED.last_refresh_at")
					refreshed++
				}
			}
			if viewExists(ctx, "cc_team_task_rollup") {
				if _, err := db.ExecContext(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY cc_team_task_rollup"); err != nil {
					slog.Debug("rollup.refresh.failed", "view", "cc_team_task_rollup", "error", err)
					failures++
				} else {
					_, _ = db.ExecContext(ctx, "INSERT INTO control_center_rollup_state(name,last_refresh_at) VALUES('cc_team_task_rollup',NOW()) ON CONFLICT(name) DO UPDATE SET last_refresh_at=EXCLUDED.last_refresh_at")
					refreshed++
				}
			}
			if failures > 0 {
				consecutive := rollupRefresherHealth.consecutiveFailure.Add(int64(failures))
				slog.Warn("rollup.refresh.health.degraded",
					"failures", failures,
					"consecutive_failures", consecutive,
					"refreshed", refreshed,
					"fast_fail_enabled", fastFail,
				)
				if fastFail && consecutive >= int64(fastFailAfter) {
					slog.Error("rollup.refresh.health.fast_fail",
						"consecutive_failures", consecutive,
						"threshold", fastFailAfter,
					)
					os.Exit(1)
				}
			} else if refreshed > 0 {
				rollupRefresherHealth.consecutiveFailure.Store(0)
				rollupRefresherHealth.lastSuccessUnix.Store(time.Now().Unix())
			}
		}
		refresh()
		for range ticker.C {
			refresh()
		}
	}()
	slog.Info("control-center rollup refresher started",
		"interval_sec", intervalSec,
		"fast_fail_enabled", fastFail,
		"fast_fail_after", fastFailAfter)
}

// wireManagedHTTP creates managed-mode HTTP handlers (agents + skills + traces + MCP + custom tools + channel instances + providers + delegations + builtin tools).
func wireManagedHTTP(stores *store.Stores, token string, msgBus *bus.MessageBus, toolsReg *tools.Registry, providerReg *providers.Registry, isOwner func(string) bool) (*httpapi.AgentsHandler, *httpapi.SkillsHandler, *httpapi.TracesHandler, *httpapi.MCPHandler, *httpapi.CustomToolsHandler, *httpapi.ChannelInstancesHandler, *httpapi.ProvidersHandler, *httpapi.DelegationsHandler, *httpapi.BuiltinToolsHandler, *httpapi.ControlCenterHandler) {
	var agentsH *httpapi.AgentsHandler
	var skillsH *httpapi.SkillsHandler
	var tracesH *httpapi.TracesHandler
	var mcpH *httpapi.MCPHandler
	var customToolsH *httpapi.CustomToolsHandler
	var channelInstancesH *httpapi.ChannelInstancesHandler
	var providersH *httpapi.ProvidersHandler
	var delegationsH *httpapi.DelegationsHandler
	var builtinToolsH *httpapi.BuiltinToolsHandler
	var controlCenterH *httpapi.ControlCenterHandler

	if stores != nil && stores.Agents != nil {
		var summoner *httpapi.AgentSummoner
		if providerReg != nil {
			summoner = httpapi.NewAgentSummoner(stores.Agents, providerReg, msgBus)
		}
		agentsH = httpapi.NewAgentsHandler(stores.Agents, token, msgBus, summoner, isOwner)
	}

	if stores != nil && stores.Skills != nil {
		if pgSkills, ok := stores.Skills.(*pg.PGSkillStore); ok {
			dirs := pgSkills.Dirs()
			if len(dirs) > 0 {
				skillsH = httpapi.NewSkillsHandler(pgSkills, dirs[0], token, msgBus)
			}
		}
	}

	if stores != nil && stores.Tracing != nil {
		tracesH = httpapi.NewTracesHandler(stores.Tracing, token)
	}

	if stores != nil && stores.MCP != nil {
		mcpH = httpapi.NewMCPHandler(stores.MCP, token)
	}

	if stores != nil && stores.CustomTools != nil {
		customToolsH = httpapi.NewCustomToolsHandler(stores.CustomTools, token, msgBus, toolsReg)
	}

	if stores != nil && stores.ChannelInstances != nil {
		channelInstancesH = httpapi.NewChannelInstancesHandler(stores.ChannelInstances, stores.Agents, token, msgBus)
	}

	if stores != nil && stores.Providers != nil {
		providersH = httpapi.NewProvidersHandler(stores.Providers, token, providerReg)
	}

	if stores != nil && stores.Teams != nil {
		delegationsH = httpapi.NewDelegationsHandler(stores.Teams, token)
	}

	if stores != nil && stores.BuiltinTools != nil {
		builtinToolsH = httpapi.NewBuiltinToolsHandler(stores.BuiltinTools, token, msgBus)
	}
	if stores != nil && stores.Agents != nil && stores.Tracing != nil && stores.ChannelInstances != nil {
		controlCenterH = httpapi.NewControlCenterHandler(stores.Agents, stores.Tracing, stores.ChannelInstances, stores.Teams, stores.DB, token)
	}

	return agentsH, skillsH, tracesH, mcpH, customToolsH, channelInstancesH, providersH, delegationsH, builtinToolsH, controlCenterH
}
