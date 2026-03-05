import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { PageHeader } from "@/components/shared/page-header";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useHttp } from "@/hooks/use-ws";
import { formatRelativeTime } from "@/lib/format";

interface LiveRun {
  id: string;
  agent_id?: string;
  user_id?: string;
  session_key?: string;
  name?: string;
  channel?: string;
  status: string;
  start_time: string;
}

interface KanbanTask {
  id: string;
  team_id: string;
  subject: string;
  description?: string;
  status: string;
  priority: number;
  owner_agent_id?: string;
  owner_agent_key?: string;
  user_id?: string;
  channel?: string;
  updated_at: string;
}

export function OperationsPage() {
  const http = useHttp();
  const [selectedTaskIds, setSelectedTaskIds] = useState<string[]>([]);
  const [ownerAgentID, setOwnerAgentID] = useState("");
  const { data, isLoading, refetch } = useQuery({
    queryKey: ["control-center-live-runs"],
    queryFn: async () =>
      http.get<{ runs: LiveRun[]; total: number; limit: number }>(
        "/v1/admin/control-center/runs/live",
      ),
    refetchInterval: 3000,
  });
  const { data: kanban } = useQuery({
    queryKey: ["control-center-kanban"],
    queryFn: async () =>
      http.get<{
        columns: Record<string, KanbanTask[]>;
      }>("/v1/admin/control-center/tasks/kanban"),
    refetchInterval: 5000,
  });

  async function applyBatch(action: "reassign" | "pause" | "escalate") {
    if (selectedTaskIds.length === 0) return;
    const payload: Record<string, unknown> = {
      action,
      task_ids: selectedTaskIds,
    };
    if (action === "reassign") {
      payload.owner_agent_id = ownerAgentID;
    }
    await http.post("/v1/admin/control-center/tasks/batch", payload);
    setSelectedTaskIds([]);
    await refetch();
  }

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Operations"
        description="Live runs and operational controls"
        actions={
          <Button variant="outline" onClick={() => refetch()}>
            Refresh
          </Button>
        }
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Live Runs</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading...</p>
          ) : (data?.runs ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground">No active runs</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b text-left text-muted-foreground">
                    <th className="pb-2 pr-3 font-medium">Run</th>
                    <th className="pb-2 px-3 font-medium">Agent</th>
                    <th className="pb-2 px-3 font-medium">User</th>
                    <th className="pb-2 px-3 font-medium">Channel</th>
                    <th className="pb-2 px-3 font-medium">Started</th>
                    <th className="pb-2 pl-3 font-medium">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {(data?.runs ?? []).map((run) => (
                    <tr key={run.id} className="border-b last:border-0">
                      <td className="py-2.5 pr-3">
                        <div className="max-w-[260px] truncate">
                          {run.name || run.id}
                        </div>
                      </td>
                      <td className="py-2.5 px-3 font-mono text-xs">
                        {run.agent_id || "—"}
                      </td>
                      <td className="py-2.5 px-3 font-mono text-xs">
                        {run.user_id || "—"}
                      </td>
                      <td className="py-2.5 px-3">{run.channel || "—"}</td>
                      <td className="py-2.5 px-3">
                        {formatRelativeTime(run.start_time)}
                      </td>
                      <td className="py-2.5 pl-3">
                        <div className="flex items-center gap-2">
                          <Button
                            size="sm"
                            variant="outline"
                            disabled
                            title="Retry wiring is next step"
                          >
                            Retry
                          </Button>
                          <Button
                            size="sm"
                            variant="destructive"
                            disabled
                            title="Abort wiring is next step"
                          >
                            Abort
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-base">Kanban</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="mb-3 flex flex-wrap items-center gap-2">
            <input
              value={ownerAgentID}
              onChange={(e) => setOwnerAgentID(e.target.value)}
              placeholder="Owner agent UUID for reassign"
              className="h-9 w-72 rounded-md border bg-background px-3 text-sm"
            />
            <Button
              size="sm"
              variant="outline"
              disabled={selectedTaskIds.length === 0 || !ownerAgentID}
              onClick={() => void applyBatch("reassign")}
            >
              Reassign
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={selectedTaskIds.length === 0}
              onClick={() => void applyBatch("pause")}
            >
              Pause
            </Button>
            <Button
              size="sm"
              variant="outline"
              disabled={selectedTaskIds.length === 0}
              onClick={() => void applyBatch("escalate")}
            >
              Escalate
            </Button>
            <span className="text-xs text-muted-foreground">
              selected: {selectedTaskIds.length}
            </span>
          </div>
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
            {["pending", "in_progress", "blocked", "completed"].map((col) => {
              const items = kanban?.columns?.[col] ?? [];
              return (
                <div key={col} className="rounded-lg border bg-muted/30 p-3">
                  <div className="mb-2 flex items-center justify-between">
                    <h3 className="text-sm font-medium capitalize">
                      {col.replace("_", " ")}
                    </h3>
                    <span className="text-xs text-muted-foreground">
                      {items.length}
                    </span>
                  </div>
                  <div className="space-y-2">
                    {items.slice(0, 8).map((task) => (
                      <div
                        key={task.id}
                        className="rounded border bg-background p-2"
                      >
                        <div className="mb-1">
                          <input
                            type="checkbox"
                            checked={selectedTaskIds.includes(task.id)}
                            onChange={(e) => {
                              setSelectedTaskIds((prev) =>
                                e.target.checked
                                  ? [...prev, task.id]
                                  : prev.filter((id) => id !== task.id),
                              );
                            }}
                          />
                        </div>
                        <div className="truncate text-sm font-medium">
                          {task.subject}
                        </div>
                        <div className="mt-1 flex items-center justify-between text-xs text-muted-foreground">
                          <span>{task.owner_agent_key || "unassigned"}</span>
                          <span>P{task.priority}</span>
                        </div>
                      </div>
                    ))}
                    {items.length > 8 && (
                      <p className="text-xs text-muted-foreground">
                        +{items.length - 8} more
                      </p>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
