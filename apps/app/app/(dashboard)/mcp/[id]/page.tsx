"use client";

import { useState } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { AppLayout } from "@/components/AppLayout";
import { PageHeader } from "@/components/molecules/PageHeader";
import {
  Card,
  CardHeader,
  CardTitle,
  CardContent,
  CardDescription,
} from "@/components/molecules/Card";
import { Button } from "@/components/atoms/Button";
import { Badge, StatusDot } from "@/components/atoms/Badge";
import { Progress } from "@/components/atoms/Progress";
import { MetricCard } from "@/components/atoms/MetricCard";
import { ConfirmDialog } from "@/components/molecules/ConfirmDialog";
import { ErrorState, EmptyState } from "@/components/molecules/StateAlerts";
import { useMCPServersQuery, useMCPServerStatsQuery, useMCPServerToolsQuery } from "@/hooks/queries/useMCPServersQuery";
import { useDeleteMCPServerMutation, useSyncMCPServerMutation } from "@/hooks/mutations/useMCPMutations";
import { ApiMCPServer } from "@/lib/api";import { AppRoutes } from "@/constants/routes";
import { MCPServerFormDialog } from "../_components/MCPServerFormDialog";
import { MCPTestModal } from "../_components/MCPTestModal";
import {
  ArrowLeft,
  Activity,
  Bot,
  CheckCircle2,
  Cpu,
  Globe,
  Loader2,
  Pencil,
  Play,
  RefreshCw,
  Trash2,
  XCircle,
  Zap,
} from "lucide-react";
import { toast } from "sonner";
import { getErrorMessage } from "@/types/ui";

function statusToDot(status: string) {
  switch (status) {
    case "healthy":
    case "connected":
      return "healthy" as const;
    case "degraded":
      return "degraded" as const;
    case "error":
      return "exhausted" as const;
    case "disconnected":
    case "offline":
      return "cooldown" as const;
    default:
      return "disabled" as const;
  }
}

const WINDOW_OPTIONS = [
  { label: "7 days", value: 7 },
  { label: "30 days", value: 30 },
  { label: "90 days", value: 90 },
] as const;

function formatNumber(n: number): string {
  return Number.isFinite(n) ? n.toLocaleString() : "0";
}

function MCPServerDetailSkeleton() {
  return (
    <AppLayout>
      {/* Page Header Skeleton */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
        <div className="space-y-2">
          <div className="h-7 w-48 rounded bg-muted/60 animate-pulse" />
          <div className="h-4 w-80 rounded bg-muted/40 animate-pulse" />
        </div>
        <div className="h-8 w-36 rounded border border-border bg-muted/30 animate-pulse self-start sm:self-auto" />
      </div>

      {/* Badges Bar Skeleton */}
      <div className="mb-6 flex flex-wrap items-center gap-2">
        <div className="h-5 w-16 rounded border border-border bg-muted/40 animate-pulse" />
        <div className="h-5 w-14 rounded border border-border bg-muted/40 animate-pulse" />
        <div className="h-5 w-24 rounded border border-border bg-muted/40 animate-pulse" />
        <div className="h-5 w-16 rounded border border-border bg-muted/40 animate-pulse" />
      </div>

      {/* Endpoint Banner Skeleton */}
      <div className="mb-6">
        <Card className="animate-pulse">
          <CardContent className="p-4">
            <div className="flex items-center gap-2">
              <div className="h-3.5 w-16 rounded bg-muted/50" />
              <div className="h-3.5 w-72 rounded bg-muted/30" />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* 4 Metric Cards Skeleton */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard title="Total Requests" value="" subtitle="Last 30 days" loading={true} />
        <MetricCard title="Success Rate" value="" loading={true} />
        <MetricCard title="Avg Latency" value="" subtitle="per tool call" loading={true} />
        <MetricCard title="Errors" value="" subtitle="failed invocations" loading={true} />
      </div>

      {/* Usage Analytics Header Skeleton */}
      <div className="mt-8 mb-4 flex items-center justify-between">
        <div className="h-5 w-32 rounded bg-muted/60 animate-pulse" />
        <div className="flex items-center gap-1 rounded border border-border p-1 bg-muted/20">
          <div className="h-6 w-14 rounded bg-muted/30 animate-pulse" />
          <div className="h-6 w-14 rounded bg-muted/50 animate-pulse" />
          <div className="h-6 w-14 rounded bg-muted/30 animate-pulse" />
        </div>
      </div>

      {/* Analytics 2-Column Grid Skeleton */}
      <div className="grid gap-4 lg:grid-cols-2">
        <Card className="animate-pulse">
          <CardHeader className="pb-3 space-y-1.5">
            <div className="h-4 w-36 rounded bg-muted/60" />
            <div className="h-3 w-56 rounded bg-muted/40" />
          </CardHeader>
          <CardContent className="pt-0 space-y-3">
            <div className="h-6 w-full rounded bg-muted/30" />
            <div className="h-6 w-4/5 rounded bg-muted/30" />
          </CardContent>
        </Card>

        <Card className="animate-pulse">
          <CardHeader className="pb-3 space-y-1.5">
            <div className="h-4 w-32 rounded bg-muted/60" />
            <div className="h-3 w-64 rounded bg-muted/40" />
          </CardHeader>
          <CardContent className="pt-0 space-y-2">
            <div className="h-12 w-full rounded border border-border bg-muted/30" />
          </CardContent>
        </Card>
      </div>

      {/* Tools Table Skeleton */}
      <div className="mt-6">
        <Card className="animate-pulse">
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <div className="space-y-1.5">
                <div className="h-4 w-16 rounded bg-muted/60" />
                <div className="h-3 w-40 rounded bg-muted/40" />
              </div>
              <div className="flex items-center gap-2">
                <div className="h-7 w-16 rounded border border-border bg-muted/40" />
                <div className="h-7 w-20 rounded border border-border bg-muted/40" />
                <div className="h-7 w-16 rounded border border-border bg-muted/40" />
              </div>
            </div>
          </CardHeader>
          <CardContent className="pt-0">
            <div className="space-y-3">
              <div className="h-8 w-full rounded bg-muted/40" />
              <div className="h-10 w-full rounded bg-muted/20" />
              <div className="h-10 w-full rounded bg-muted/20" />
              <div className="h-10 w-full rounded bg-muted/20" />
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Delete Button Skeleton */}
      <div className="mt-6 flex justify-end">
        <div className="h-8 w-28 rounded bg-destructive/20 animate-pulse" />
      </div>
    </AppLayout>
  );
}

export default function MCPServerDetailPage() {
  const params = useParams<{ id: string }>();
  const serverId = params?.id || "";
  const router = useRouter();

  const serversQuery = useMCPServersQuery();
  const [testModalOpen, setTestModalOpen] = useState(false);
  const [formModalOpen, setFormModalOpen] = useState(false);
  const [editingForForm, setEditingForForm] = useState<ApiMCPServer | null>(null);
  const [days, setDays] = useState<number>(30);
  const [syncingId, setSyncingId] = useState<string | null>(null);

  const deleteMutation = useDeleteMCPServerMutation();
  const syncMutation = useSyncMCPServerMutation();

  const statsQuery = useMCPServerStatsQuery(serverId, days);
  const toolsQuery = useMCPServerToolsQuery(serverId);

  const servers: ApiMCPServer[] = serversQuery.data && Array.isArray(serversQuery.data) ? serversQuery.data : [];
  const server = servers.find((s) => s.id === serverId);
  const stats = statsQuery.data;
  const tools = toolsQuery.data && Array.isArray(toolsQuery.data) ? toolsQuery.data : [];

  const openEdit = () => {
    if (!server) return;
    setEditingForForm(server);
    setFormModalOpen(true);
  };

  if (serversQuery.isLoading) {
    return <MCPServerDetailSkeleton />;
  }

  if (!server) {
    return (
      <AppLayout>
        <PageHeader
          title="MCP Server"
          extra={
            <Button variant="outline" size="sm" asChild>
              <Link href={AppRoutes.MCP}>
                <ArrowLeft className="h-4 w-4" />
                <span>Back to MCP Gateway</span>
              </Link>
            </Button>
          }
        />
        <Card>
          <CardContent className="py-8">
            <ErrorState
              title="MCP server not found"
              description="The requested server may have been removed."
              onRetry={() => serversQuery.refetch()}
            />
          </CardContent>
        </Card>
      </AppLayout>
    );
  }

  const isSyncingThis = syncMutation.isPending && syncingId === server.id;

  return (
    <AppLayout>
      <PageHeader
        title={server.displayName || server.name}
        description={`${server.type === "local" ? "Local" : "Remote"} Model Context Protocol server via ${server.transportType} transport.`}
        extra={
          <Button variant="outline" size="sm" asChild>
            <Link href={AppRoutes.MCP}>
              <ArrowLeft className="h-4 w-4" />
              <span>Back to MCP Gateway</span>
            </Link>
          </Button>
        }
      />

      <div className="mb-6 flex flex-wrap items-center gap-2">
        <Badge variant="outline" className="font-mono text-[10px] uppercase">
          {server.type === "local" ? <Cpu className="mr-1 h-3 w-3" /> : <Globe className="mr-1 h-3 w-3" />}
          {server.type}
        </Badge>
        <Badge variant="outline" className="font-mono text-[10px] uppercase">
          {server.transportType}
        </Badge>
        <StatusDot status={statusToDot(server.status || server.healthStatus || 'unknown')} />
        <span className="capitalize text-xs font-mono text-muted-foreground">{server.status || server.healthStatus || 'unknown'}</span>
        <Badge variant={server.enabled ? "success" : "outline"} className="font-mono text-[10px] uppercase">
          {server.enabled ? "Enabled" : "Disabled"}
        </Badge>
      </div>

      <div className="mb-6">
        {server.type === "local" ? (
          <Card>
            <CardContent className="p-4 font-mono text-[11px]">
              <span className="text-muted-foreground">Command: </span>
              <span className="text-foreground">{server.command}</span>
              {(server.args || []).length > 0 && (
                <span className="ml-2 text-muted-foreground">args=[{(server.args || []).join(", ")}]</span>
              )}
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent className="p-4 font-mono text-[11px] truncate">
              <span className="text-muted-foreground">Endpoint: </span>
              <span className="text-foreground">{server.endpointUrl}</span>
            </CardContent>
          </Card>
        )}
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          title="Total Requests"
          value={formatNumber(stats?.totalRequests ?? 0)}
          icon={<Activity className="h-4 w-4" />}
          subtitle={`Last ${days} days`}
          loading={statsQuery.isLoading}
        />
        <MetricCard
          title="Success Rate"
          value={stats ? `${(stats.successRate * 100).toFixed(1)}%` : "0%"}
          icon={<CheckCircle2 className="h-4 w-4 text-emerald-500" />}
          delta={`${formatNumber(stats?.successCount ?? 0)} ok`}
          deltaType="positive"
          subtitle={`${formatNumber(stats?.errorCount ?? 0)} failed`}
          loading={statsQuery.isLoading}
        />
        <MetricCard
          title="Avg Latency"
          value={stats ? `${Math.round(stats.avgLatencyMs)}ms` : "0ms"}
          icon={<Zap className="h-4 w-4" />}
          deltaType="neutral"
          subtitle="per tool call"
          loading={statsQuery.isLoading}
        />
        <MetricCard
          title="Errors"
          value={formatNumber(stats?.errorCount ?? 0)}
          icon={<XCircle className="h-4 w-4 text-red-500" />}
          delta={`${formatNumber(stats?.totalRequests ?? 0)} total`}
          deltaType="negative"
          subtitle="failed invocations"
          loading={statsQuery.isLoading}
        />
      </div>

      <div className="mt-6 flex items-center justify-between gap-3">
        <h2 className="text-sm font-semibold text-foreground">Usage Analytics</h2>
        <div className="flex gap-1.5">
          {WINDOW_OPTIONS.map((opt) => (
            <Button
              key={opt.value}
              variant={days === opt.value ? "prismViolet" : "outline"}
              size="sm"
              className="h-7 px-3 text-xs"
              onClick={() => setDays(opt.value)}
            >
              {opt.label}
            </Button>
          ))}
        </div>
      </div>

      {statsQuery.isError ? (
        <Card className="mt-4">
          <CardContent className="py-8">
            <ErrorState title="Failed to load usage analytics" onRetry={() => statsQuery.refetch()} />
          </CardContent>
        </Card>
      ) : (
        <div className="mt-4 grid gap-6 lg:grid-cols-2">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">Tool Usage Breakdown</CardTitle>
              <CardDescription>Invocations per tool over the selected window.</CardDescription>
            </CardHeader>
            <CardContent className="pt-0">
              {statsQuery.isLoading ? (
                <div className="space-y-3">
                  <div className="h-4 animate-pulse rounded bg-muted" />
                  <div className="h-4 animate-pulse rounded bg-muted" />
                  <div className="h-4 animate-pulse rounded bg-muted" />
                </div>
              ) : stats?.tools && stats.tools.length > 0 ? (
                <div className="space-y-4">
                  {stats.tools.slice(0, 8).map((t) => {
                    const pct = stats.totalRequests > 0 ? (t.requests / stats.totalRequests) * 100 : 0;
                    return (
                      <div key={t.tool}>
                        <div className="mb-1 flex items-center justify-between text-xs">
                          <span className="font-mono text-foreground">{t.tool}</span>
                          <span className="text-muted-foreground">
                            {formatNumber(t.requests)} req
                            {t.errors > 0 && <span className="ml-1 text-red-500">({t.errors} err)</span>}
                            <span className="ml-1">· {Math.round(t.avgLatencyMs)}ms</span>
                          </span>
                        </div>
                        <Progress value={pct} />
                      </div>
                    );
                  })}
                </div>
              ) : (
                <EmptyState
                  title="No usage data"
                  description="This server has not been invoked within the selected window."
                />
              )}
            </CardContent>
          </Card>

          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">Connected Agents</CardTitle>
              <CardDescription>Agents currently allowed to use this MCP server.</CardDescription>
            </CardHeader>
            <CardContent className="pt-0">
              {stats?.agents && stats.agents.length > 0 ? (
                <div className="space-y-2">
                  {stats.agents.map((a) => (
                    <div
                      key={a.id}
                      className="flex items-center justify-between rounded border border-border bg-muted/40 p-3"
                    >
                      <div className="flex items-center gap-2.5">
                        <Bot className="h-4 w-4 text-muted-foreground" />
                        <div>
                          <p className="text-sm font-medium text-foreground">{a.displayName || a.name}</p>
                          <p className="font-mono text-[11px] text-muted-foreground">{a.name}</p>
                        </div>
                      </div>
                      <Badge variant={a.enabled ? "success" : "outline"} className="font-mono text-[10px] uppercase">
                        {a.enabled ? "Enabled" : "Disabled"}
                      </Badge>
                    </div>
                  ))}
                </div>
              ) : (
                <EmptyState
                  title="No agents connected"
                  description="No agents reference this MCP server in their allowed toolset."
                />
              )}
            </CardContent>
          </Card>
        </div>
      )}

      <div className="mt-6">
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="text-sm">Tools</CardTitle>
                <CardDescription>{formatNumber(tools.length)} tools exposed by this server.</CardDescription>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 text-xs gap-1"
                  onClick={() => {
                    setSyncingId(server.id);
                    syncMutation.mutate(server.id, {
                      onSuccess: () => toast.success("MCP server tools synced"),
                      onError: (err) => toast.error(`Sync failed: ${getErrorMessage(err)}`),
                      onSettled: () => setSyncingId(null),
                    });
                  }}
                  disabled={isSyncingThis}
                >
                  {isSyncingThis ? (
                    <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />
                  ) : (
                    <RefreshCw className="h-3 w-3" />
                  )}
                  <span>Sync</span>
                </Button>
                <Button variant="outline" size="sm" className="h-7 text-xs gap-1" onClick={() => setTestModalOpen(true)}>
                  <Play className="h-3 w-3 text-muted-foreground" />
                  <span>Test Tool</span>
                </Button>
                <Button variant="outline" size="sm" className="h-7 text-xs gap-1" onClick={openEdit}>
                  <Pencil className="h-3 w-3" />
                  <span>Edit</span>
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent className="pt-0">
            {tools.length > 0 ? (
              <div className="overflow-x-auto">
                <table className="w-full text-left text-sm">
                  <thead>
                    <tr className="border-b border-border text-xs uppercase tracking-wider text-muted-foreground">
                      <th className="py-2 pr-3 font-medium">Tool</th>
                      <th className="py-2 pr-3 font-medium">Description</th>
                      <th className="py-2 pr-3 font-medium">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {tools.map((t) => (
                      <tr key={t.id} className="border-b border-border/60 last:border-0 hover:bg-muted/30 transition-colors">
                        <td className="py-3 pr-4 font-mono text-xs font-semibold text-foreground align-top whitespace-nowrap">{t.name}</td>
                        <td className="py-3 pr-4 align-top">
                          <p className="text-xs text-muted-foreground leading-relaxed line-clamp-2 max-w-3xl" title={t.description || ""}>
                            {t.description || "No description provided."}
                          </p>
                        </td>
                        <td className="py-3 align-top whitespace-nowrap">
                          <Badge variant={t.enabled ? "success" : "outline"} className="font-mono text-[10px] uppercase">
                            {t.enabled ? "Enabled" : "Disabled"}
                          </Badge>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <EmptyState title="No tools synced" description="Sync tools from this server to make them available." />
            )}
          </CardContent>
        </Card>
      </div>

      <MCPServerFormDialog open={formModalOpen} onOpenChange={setFormModalOpen} editingServer={editingForForm} />

      <MCPTestModal open={testModalOpen} onOpenChange={setTestModalOpen} server={server} />

      <div className="mt-6 flex justify-end">
        <ConfirmDialog
          title="Delete MCP Server"
          description={`Are you sure you want to unregister MCP server "${server.displayName || server.name}"? This also removes its usage history.`}
          confirmText="Delete"
          variant="destructive"
          onConfirm={() => {
            deleteMutation.mutate(server.id, {
              onSuccess: () => {
                toast.success("MCP server removed");
                router.push(AppRoutes.MCP);
              },
              onError: (err) => toast.error(`Delete failed: ${getErrorMessage(err)}`),
            });
          }}
          trigger={
            <Button variant="destructive" size="sm" className="gap-1.5">
              <Trash2 className="h-4 w-4" />
              <span>Delete Server</span>
            </Button>
          }
        />
      </div>
    </AppLayout>
  );
}
