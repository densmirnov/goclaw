import { lazy, Suspense } from "react";
import { Routes, Route, Navigate } from "react-router";
import { AppLayout } from "@/components/layout/app-layout";
import { RequireAuth } from "@/components/shared/require-auth";
import { RequireRole } from "@/components/shared/require-role";
import { ROUTES } from "@/lib/constants";

// Lazy-loaded pages
const LoginPage = lazy(() =>
  import("@/pages/login/login-page").then((m) => ({ default: m.LoginPage })),
);
const OverviewPage = lazy(() =>
  import("@/pages/overview/overview-page").then((m) => ({
    default: m.OverviewPage,
  })),
);
const ChatPage = lazy(() =>
  import("@/pages/chat/chat-page").then((m) => ({ default: m.ChatPage })),
);
const AgentsPage = lazy(() =>
  import("@/pages/agents/agents-page").then((m) => ({ default: m.AgentsPage })),
);
const SessionsPage = lazy(() =>
  import("@/pages/sessions/sessions-page").then((m) => ({
    default: m.SessionsPage,
  })),
);
const SkillsPage = lazy(() =>
  import("@/pages/skills/skills-page").then((m) => ({ default: m.SkillsPage })),
);
const CronPage = lazy(() =>
  import("@/pages/cron/cron-page").then((m) => ({ default: m.CronPage })),
);
const ConfigPage = lazy(() =>
  import("@/pages/config/config-page").then((m) => ({ default: m.ConfigPage })),
);
const TracesPage = lazy(() =>
  import("@/pages/traces/traces-page").then((m) => ({ default: m.TracesPage })),
);
const UsagePage = lazy(() =>
  import("@/pages/usage/usage-page").then((m) => ({ default: m.UsagePage })),
);
const ChannelsPage = lazy(() =>
  import("@/pages/channels/channels-page").then((m) => ({
    default: m.ChannelsPage,
  })),
);
const ApprovalsPage = lazy(() =>
  import("@/pages/approvals/approvals-page").then((m) => ({
    default: m.ApprovalsPage,
  })),
);
const NodesPage = lazy(() =>
  import("@/pages/nodes/nodes-page").then((m) => ({ default: m.NodesPage })),
);
const LogsPage = lazy(() =>
  import("@/pages/logs/logs-page").then((m) => ({ default: m.LogsPage })),
);
const ProvidersPage = lazy(() =>
  import("@/pages/providers/providers-page").then((m) => ({
    default: m.ProvidersPage,
  })),
);
const CustomToolsPage = lazy(() =>
  import("@/pages/custom-tools/custom-tools-page").then((m) => ({
    default: m.CustomToolsPage,
  })),
);
const MCPPage = lazy(() =>
  import("@/pages/mcp/mcp-page").then((m) => ({ default: m.MCPPage })),
);
const TeamsPage = lazy(() =>
  import("@/pages/teams/teams-page").then((m) => ({ default: m.TeamsPage })),
);
const BuiltinToolsPage = lazy(() =>
  import("@/pages/builtin-tools/builtin-tools-page").then((m) => ({
    default: m.BuiltinToolsPage,
  })),
);
const TtsPage = lazy(() =>
  import("@/pages/tts/tts-page").then((m) => ({ default: m.TtsPage })),
);
const DelegationsPage = lazy(() =>
  import("@/pages/delegations/delegations-page").then((m) => ({
    default: m.DelegationsPage,
  })),
);
const OperationsPage = lazy(() =>
  import("@/pages/operations/operations-page").then((m) => ({
    default: m.OperationsPage,
  })),
);
const GovernancePage = lazy(() =>
  import("@/pages/governance/governance-page").then((m) => ({
    default: m.GovernancePage,
  })),
);
const KnowledgePage = lazy(() =>
  import("@/pages/knowledge/knowledge-page").then((m) => ({
    default: m.KnowledgePage,
  })),
);
const DelegationMapPage = lazy(() =>
  import("@/pages/delegation-map/delegation-map-page").then((m) => ({
    default: m.DelegationMapPage,
  })),
);
const CostAnalyticsPage = lazy(() =>
  import("@/pages/cost-analytics/cost-analytics-page").then((m) => ({
    default: m.CostAnalyticsPage,
  })),
);
const SwarmHealthPage = lazy(() =>
  import("@/pages/swarm-health/swarm-health-page").then((m) => ({
    default: m.SwarmHealthPage,
  })),
);

function PageLoader() {
  return (
    <div className="flex h-full items-center justify-center">
      <div className="h-6 w-6 animate-spin rounded-full border-2 border-muted-foreground border-t-transparent" />
    </div>
  );
}

export function AppRoutes() {
  return (
    <Suspense fallback={<PageLoader />}>
      <Routes>
        <Route path={ROUTES.LOGIN} element={<LoginPage />} />

        <Route
          element={
            <RequireAuth>
              <AppLayout />
            </RequireAuth>
          }
        >
          <Route index element={<Navigate to={ROUTES.OVERVIEW} replace />} />
          <Route path={ROUTES.OVERVIEW} element={<OverviewPage />} />
          <Route path={ROUTES.CHAT} element={<ChatPage />} />
          <Route path={ROUTES.CHAT_SESSION} element={<ChatPage />} />
          <Route
            path={ROUTES.AGENTS}
            element={
              <RequireRole minRole="operator">
                <AgentsPage key="list" />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.AGENT_DETAIL}
            element={
              <RequireRole minRole="operator">
                <AgentsPage key="detail" />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.TEAMS}
            element={
              <RequireRole minRole="operator">
                <TeamsPage key="list" />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.TEAM_DETAIL}
            element={
              <RequireRole minRole="operator">
                <TeamsPage key="detail" />
              </RequireRole>
            }
          />
          <Route path={ROUTES.SESSIONS} element={<SessionsPage key="list" />} />
          <Route
            path={ROUTES.SESSION_DETAIL}
            element={<SessionsPage key="detail" />}
          />
          <Route
            path={ROUTES.SKILLS}
            element={
              <RequireRole minRole="operator">
                <SkillsPage key="list" />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.SKILL_DETAIL}
            element={
              <RequireRole minRole="operator">
                <SkillsPage key="detail" />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.CRON}
            element={
              <RequireRole minRole="operator">
                <CronPage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.CONFIG}
            element={
              <RequireRole minRole="admin">
                <ConfigPage />
              </RequireRole>
            }
          />
          <Route path={ROUTES.TRACES} element={<TracesPage key="list" />} />
          <Route
            path={ROUTES.TRACE_DETAIL}
            element={<TracesPage key="detail" />}
          />
          <Route path={ROUTES.DELEGATIONS} element={<DelegationsPage />} />
          <Route
            path={ROUTES.OPERATIONS}
            element={
              <RequireRole minRole="operator">
                <OperationsPage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.GOVERNANCE}
            element={
              <RequireRole minRole="admin">
                <GovernancePage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.KNOWLEDGE}
            element={
              <RequireRole minRole="admin">
                <KnowledgePage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.DELEGATION_MAP}
            element={
              <RequireRole minRole="admin">
                <DelegationMapPage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.COST_ANALYTICS}
            element={
              <RequireRole minRole="admin">
                <CostAnalyticsPage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.SWARM_HEALTH}
            element={
              <RequireRole minRole="admin">
                <SwarmHealthPage />
              </RequireRole>
            }
          />
          <Route path={ROUTES.USAGE} element={<UsagePage />} />
          <Route
            path={ROUTES.CHANNELS}
            element={
              <RequireRole minRole="operator">
                <ChannelsPage key="list" />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.CHANNEL_DETAIL}
            element={
              <RequireRole minRole="operator">
                <ChannelsPage key="detail" />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.APPROVALS}
            element={
              <RequireRole minRole="operator">
                <ApprovalsPage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.NODES}
            element={
              <RequireRole minRole="admin">
                <NodesPage />
              </RequireRole>
            }
          />
          <Route path={ROUTES.LOGS} element={<LogsPage />} />
          <Route
            path={ROUTES.PROVIDERS}
            element={
              <RequireRole minRole="admin">
                <ProvidersPage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.CUSTOM_TOOLS}
            element={
              <RequireRole minRole="admin">
                <CustomToolsPage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.BUILTIN_TOOLS}
            element={
              <RequireRole minRole="admin">
                <BuiltinToolsPage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.MCP}
            element={
              <RequireRole minRole="admin">
                <MCPPage />
              </RequireRole>
            }
          />
          <Route
            path={ROUTES.TTS}
            element={
              <RequireRole minRole="admin">
                <TtsPage />
              </RequireRole>
            }
          />
        </Route>

        {/* Catch-all → overview */}
        <Route path="*" element={<Navigate to={ROUTES.OVERVIEW} replace />} />
      </Routes>
    </Suspense>
  );
}
