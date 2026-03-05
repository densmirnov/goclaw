import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/shared/page-header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useHttp } from "@/hooks/use-ws";

export function DelegationMapPage() {
  const http = useHttp();
  const { data } = useQuery({
    queryKey: ["control-center-delegation-map"],
    queryFn: async () =>
      http.get<{
        edges: Array<{
          source_agent_key?: string;
          target_agent_key?: string;
          source_agent_id: string;
          target_agent_id: string;
          count: number;
          failures: number;
          avg_duration_ms: number;
        }>;
      }>("/v1/admin/control-center/delegation-map"),
    refetchInterval: 5000,
  });

  return (
    <div className="space-y-6 p-6">
      <PageHeader title="Delegation Map" description="Graph of task handoff between agents" />
      <Card>
        <CardHeader><CardTitle className="text-base">Edges</CardTitle></CardHeader>
        <CardContent>
          <div className="space-y-2">
            {(data?.edges ?? []).map((e) => (
              <div key={`${e.source_agent_id}-${e.target_agent_id}`} className="rounded border p-2 text-sm">
                <span className="font-medium">{e.source_agent_key || e.source_agent_id.slice(0, 8)}</span>
                {" -> "}
                <span className="font-medium">{e.target_agent_key || e.target_agent_id.slice(0, 8)}</span>
                {` · count: ${e.count} · fail: ${e.failures} · avg: ${e.avg_duration_ms}ms`}
              </div>
            ))}
            {(data?.edges ?? []).length === 0 && <p className="text-sm text-muted-foreground">No delegations yet</p>}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
