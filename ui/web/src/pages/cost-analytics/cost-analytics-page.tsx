import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/shared/page-header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useHttp } from "@/hooks/use-ws";

export function CostAnalyticsPage() {
  const http = useHttp();
  const { data } = useQuery({
    queryKey: ["control-center-cost"],
    queryFn: async () =>
      http.get<{
        rows: Array<{ agent_id: string; total_tokens: number; est_cost_usd: number }>;
        total_tokens: number;
        estimated_cost_usd: number;
      }>("/v1/admin/control-center/cost"),
    refetchInterval: 8000,
  });

  return (
    <div className="space-y-6 p-6">
      <PageHeader title="Cost Analytics" description="Token and estimated cost by agents" />
      <div className="grid gap-4 md:grid-cols-2">
        <Card><CardHeader><CardTitle className="text-base">Total Tokens</CardTitle></CardHeader><CardContent>{data?.total_tokens ?? 0}</CardContent></Card>
        <Card><CardHeader><CardTitle className="text-base">Estimated USD</CardTitle></CardHeader><CardContent>{(data?.estimated_cost_usd ?? 0).toFixed(4)}</CardContent></Card>
      </div>
      <Card>
        <CardHeader><CardTitle className="text-base">By Agent</CardTitle></CardHeader>
        <CardContent>
          <div className="space-y-2">
            {(data?.rows ?? []).map((r) => (
              <div key={r.agent_id} className="rounded border p-2 text-sm">
                <span className="font-medium">{r.agent_id.slice(0, 8)}</span>
                {` · tokens: ${r.total_tokens} · est: $${r.est_cost_usd.toFixed(4)}`}
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
