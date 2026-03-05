import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/shared/page-header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useHttp } from "@/hooks/use-ws";

export function SwarmHealthPage() {
  const http = useHttp();
  const { data } = useQuery({
    queryKey: ["control-center-health"],
    queryFn: async () =>
      http.get<{
        score: number;
        status: string;
        error_runs: number;
        blocked_overdue: number;
        active_tasks: number;
        tool_latency_top?: Array<{
          tool: string;
          count: number;
          error_count: number;
          error_rate: number;
          avg_ms: number;
          p50_ms: number;
          p95_ms: number;
          max_ms: number;
          in_flight?: number;
        }>;
      }>("/v1/admin/control-center/health"),
    refetchInterval: 4000,
  });
  const { data: slo } = useQuery({
    queryKey: ["control-center-slo-alerts"],
    queryFn: async () =>
      http.get<{
        alerts: Array<{
          type: string;
          tool?: string;
          value: number;
          threshold: number;
          severity: string;
        }>;
      }>("/v1/admin/control-center/slo-alerts"),
    refetchInterval: 5000,
  });
  const { data: toolLatency } = useQuery({
    queryKey: ["control-center-tools-latency"],
    queryFn: async () =>
      http.get<{
        rows: Array<{
          tool: string;
          count: number;
          error_count: number;
          error_rate: number;
          avg_ms: number;
          p50_ms: number;
          p95_ms: number;
          max_ms: number;
          in_flight: number;
        }>;
      }>("/v1/admin/control-center/tools/latency?limit=12"),
    refetchInterval: 5000,
  });

  const toolAlerts = (slo?.alerts ?? []).filter(
    (a) => a.type === "tool_p95_latency",
  );

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Swarm Health"
        description="Composite health score for enterprise swarm"
      />
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Health Score</CardTitle>
          </CardHeader>
          <CardContent className="text-2xl font-semibold">
            {data?.score ?? 0}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Status</CardTitle>
          </CardHeader>
          <CardContent>{data?.status ?? "unknown"}</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Error Runs</CardTitle>
          </CardHeader>
          <CardContent>{data?.error_runs ?? 0}</CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Blocked Overdue</CardTitle>
          </CardHeader>
          <CardContent>
            {data?.blocked_overdue ?? 0}/{data?.active_tasks ?? 0}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Tool Latency Alerts</CardTitle>
        </CardHeader>
        <CardContent className="space-y-2">
          {toolAlerts.length === 0 && (
            <p className="text-sm text-muted-foreground">
              No tool latency alerts
            </p>
          )}
          {toolAlerts.map((a, idx) => (
            <div
              key={`${a.type}-${a.tool}-${idx}`}
              className="rounded border border-amber-500/40 bg-amber-500/10 p-3 text-sm"
            >
              <span className="font-medium">{a.tool ?? "unknown tool"}</span>{" "}
              p95: {a.value}ms (threshold: {a.threshold}ms)
            </div>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Tool Latency (Top)</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead>
                <tr className="border-b text-left text-muted-foreground">
                  <th className="py-2 pr-4 font-medium">Tool</th>
                  <th className="py-2 pr-4 font-medium">Count</th>
                  <th className="py-2 pr-4 font-medium">p50</th>
                  <th className="py-2 pr-4 font-medium">p95</th>
                  <th className="py-2 pr-4 font-medium">Avg</th>
                  <th className="py-2 pr-4 font-medium">Max</th>
                  <th className="py-2 pr-4 font-medium">Errors</th>
                  <th className="py-2 font-medium">In Flight</th>
                </tr>
              </thead>
              <tbody>
                {(toolLatency?.rows ?? data?.tool_latency_top ?? []).map(
                  (row) => (
                    <tr key={row.tool} className="border-b">
                      <td className="py-2 pr-4 font-medium">{row.tool}</td>
                      <td className="py-2 pr-4 tabular-nums">{row.count}</td>
                      <td className="py-2 pr-4 tabular-nums">{row.p50_ms}ms</td>
                      <td className="py-2 pr-4 tabular-nums">{row.p95_ms}ms</td>
                      <td className="py-2 pr-4 tabular-nums">
                        {Math.round(row.avg_ms)}ms
                      </td>
                      <td className="py-2 pr-4 tabular-nums">{row.max_ms}ms</td>
                      <td className="py-2 pr-4 tabular-nums">
                        {row.error_count} ({(row.error_rate * 100).toFixed(1)}%)
                      </td>
                      <td className="py-2 tabular-nums">
                        {row.in_flight ?? 0}
                      </td>
                    </tr>
                  ),
                )}
                {(toolLatency?.rows ?? data?.tool_latency_top ?? []).length ===
                  0 && (
                  <tr>
                    <td className="py-3 text-muted-foreground" colSpan={8}>
                      No tool latency data yet
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
