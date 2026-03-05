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
      }>("/v1/admin/control-center/health"),
    refetchInterval: 4000,
  });

  return (
    <div className="space-y-6 p-6">
      <PageHeader title="Swarm Health" description="Composite health score for enterprise swarm" />
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card><CardHeader><CardTitle className="text-base">Health Score</CardTitle></CardHeader><CardContent className="text-2xl font-semibold">{data?.score ?? 0}</CardContent></Card>
        <Card><CardHeader><CardTitle className="text-base">Status</CardTitle></CardHeader><CardContent>{data?.status ?? "unknown"}</CardContent></Card>
        <Card><CardHeader><CardTitle className="text-base">Error Runs</CardTitle></CardHeader><CardContent>{data?.error_runs ?? 0}</CardContent></Card>
        <Card><CardHeader><CardTitle className="text-base">Blocked Overdue</CardTitle></CardHeader><CardContent>{data?.blocked_overdue ?? 0}/{data?.active_tasks ?? 0}</CardContent></Card>
      </div>
    </div>
  );
}
