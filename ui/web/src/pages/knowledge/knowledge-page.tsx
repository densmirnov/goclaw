import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/shared/page-header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useHttp } from "@/hooks/use-ws";

export function KnowledgePage() {
  const http = useHttp();
  const { data } = useQuery({
    queryKey: ["control-center-knowledge"],
    queryFn: async () =>
      http.get<{
        sources: Array<{ name: string; freshness_hours: number; status: string }>;
        coverage_gaps: Array<{ type: string; count: number }>;
        agents_with_recent_activity: number;
        agents_stale_activity: number;
      }>("/v1/admin/control-center/knowledge"),
    refetchInterval: 7000,
  });

  return (
    <div className="space-y-6 p-6">
      <PageHeader title="Knowledge" description="Sources, freshness and coverage gaps" />
      <div className="grid gap-4 md:grid-cols-2">
        <Card><CardHeader><CardTitle className="text-base">Recent Activity</CardTitle></CardHeader><CardContent>{data?.agents_with_recent_activity ?? 0}</CardContent></Card>
        <Card><CardHeader><CardTitle className="text-base">Stale Agents</CardTitle></CardHeader><CardContent>{data?.agents_stale_activity ?? 0}</CardContent></Card>
      </div>
      <Card>
        <CardHeader><CardTitle className="text-base">Sources</CardTitle></CardHeader>
        <CardContent>
          <div className="space-y-2">
            {(data?.sources ?? []).map((s) => (
              <div key={s.name} className="rounded border p-2 text-sm">
                <span className="font-medium">{s.name}</span> · freshness: {s.freshness_hours}h · {s.status}
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
