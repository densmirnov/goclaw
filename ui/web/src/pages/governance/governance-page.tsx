import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/shared/page-header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useHttp } from "@/hooks/use-ws";

export function GovernancePage() {
  const http = useHttp();
  const { data } = useQuery({
    queryKey: ["control-center-governance"],
    queryFn: async () =>
      http.get<{
        policy_alerts: Array<{ type: string; count: number; level: string; status: string }>;
        approval_queue: Array<Record<string, unknown>>;
        access_violations: number;
        recent_error_runs: number;
      }>("/v1/admin/control-center/governance"),
    refetchInterval: 5000,
  });

  return (
    <div className="space-y-6 p-6">
      <PageHeader title="Governance" description="Policy alerts, approval queue, access violations" />
      <div className="grid gap-4 md:grid-cols-3">
        <Card><CardHeader><CardTitle className="text-base">Access Violations</CardTitle></CardHeader><CardContent>{data?.access_violations ?? 0}</CardContent></Card>
        <Card><CardHeader><CardTitle className="text-base">Error Runs</CardTitle></CardHeader><CardContent>{data?.recent_error_runs ?? 0}</CardContent></Card>
        <Card><CardHeader><CardTitle className="text-base">Approval Queue</CardTitle></CardHeader><CardContent>{data?.approval_queue?.length ?? 0}</CardContent></Card>
      </div>
      <Card>
        <CardHeader><CardTitle className="text-base">Policy Alerts</CardTitle></CardHeader>
        <CardContent>
          <div className="space-y-2">
            {(data?.policy_alerts ?? []).map((a, i) => (
              <div key={i} className="rounded border p-2 text-sm">
                <span className="font-medium">{a.type}</span> · count: {a.count} · level: {a.level}
              </div>
            ))}
            {(data?.policy_alerts ?? []).length === 0 && <p className="text-sm text-muted-foreground">No policy alerts</p>}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
