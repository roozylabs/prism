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
import { MetricCard } from "@/components/atoms/MetricCard";
import { ConfirmDialog } from "@/components/molecules/ConfirmDialog";
import { ErrorState, EmptyState } from "@/components/molecules/StateAlerts";
import { DataTable, Column } from "@/components/organisms/DataTable";
import {
  useAgentDetailQuery,
  useAgentStatsQuery,
  useDeleteAgent,
} from "@/hooks/queries/useAgentsQuery";
import { useToolsQuery } from "@/hooks/queries/useToolsQuery";
import { useResourcesQuery } from "@/hooks/queries/useResourcesQuery";
import { useMCPServersQuery } from "@/hooks/queries/useMCPServersQuery";
import { useLogsQuery } from "@/hooks/queries/useLogsQuery";
import { ApiRequestLog } from "@/lib/api";
import { AppRoutes, mcpDetailRoute } from "@/constants/routes";
import { AgentFormDialog } from "../_components/AgentFormDialog";
import { MultiSelectOption } from "@/components/molecules/MultiSelect";
import {
  ArrowLeft,
  Bot,
  Check,
  Code2,
  Copy,
  Cpu,
  ExternalLink,
  Layers,
  Play,
  Settings,
  Terminal,
  Trash2,
  Wrench,
  Zap,
} from "lucide-react";
import { toast } from "sonner";
import { getErrorMessage } from "@/types/ui";

function formatNumber(n: number): string {
  return Number.isFinite(n) ? n.toLocaleString() : "0";
}

function formatTokens(tokens: number): string {
  if (!tokens || tokens === 0) return "0 tokens";
  if (tokens >= 1_000_000) return `${(tokens / 1_000_000).toFixed(2)}M tokens`;
  if (tokens >= 1_000) return `${(tokens / 1_000).toFixed(1)}k tokens`;
  return `${tokens} tokens`;
}

function formatUSD(amount: number): string {
  if (!amount || amount === 0) return "$0.00";
  if (amount < 0.01) return `$${amount.toFixed(4)}`;
  return `$${amount.toFixed(2)}`;
}

function AgentDetailSkeleton() {
  return (
    <AppLayout>
      {/* Header Skeleton */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-6">
        <div className="space-y-2">
          <div className="h-7 w-52 rounded bg-muted/60 animate-pulse" />
          <div className="h-4 w-96 rounded bg-muted/40 animate-pulse" />
        </div>
        <div className="flex items-center gap-2">
          <div className="h-8 w-32 rounded border border-border bg-muted/30 animate-pulse" />
          <div className="h-8 w-36 rounded border border-border bg-muted/30 animate-pulse" />
        </div>
      </div>

      {/* Badges Bar Skeleton */}
      <div className="mb-6 flex flex-wrap items-center gap-2">
        <div className="h-5 w-20 rounded border border-border bg-muted/40 animate-pulse" />
        <div className="h-5 w-16 rounded border border-border bg-muted/40 animate-pulse" />
        <div className="h-5 w-24 rounded border border-border bg-muted/40 animate-pulse" />
        <div className="h-5 w-32 rounded border border-border bg-muted/40 animate-pulse" />
      </div>

      {/* 4 Metric Cards Skeleton */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard title="Total Requests" value="" subtitle="Last 30 days" loading={true} />
        <MetricCard title="Tokens Consumed" value="" subtitle="Prompt & Completion" loading={true} />
        <MetricCard title="Operational Spend" value="" subtitle="Budget & Spend Cap" loading={true} />
        <MetricCard title="Tool Invocations" value="" subtitle="Tools & MCP calls" loading={true} />
      </div>

      {/* Persona Prompt Skeleton */}
      <div className="mt-8">
        <Card className="animate-pulse">
          <CardHeader className="pb-3 space-y-1.5">
            <div className="h-4 w-44 rounded bg-muted/60" />
            <div className="h-3 w-72 rounded bg-muted/40" />
          </CardHeader>
          <CardContent className="pt-0">
            <div className="h-28 w-full rounded bg-muted/30" />
          </CardContent>
        </Card>
      </div>

      {/* Boundaries 3-Grid Skeleton */}
      <div className="mt-6 grid gap-4 lg:grid-cols-3">
        <Card className="animate-pulse">
          <CardHeader className="pb-3 space-y-1.5">
            <div className="h-4 w-32 rounded bg-muted/60" />
            <div className="h-3 w-48 rounded bg-muted/40" />
          </CardHeader>
          <CardContent className="pt-0 space-y-2">
            <div className="h-10 w-full rounded bg-muted/30" />
            <div className="h-10 w-full rounded bg-muted/30" />
          </CardContent>
        </Card>
        <Card className="animate-pulse">
          <CardHeader className="pb-3 space-y-1.5">
            <div className="h-4 w-28 rounded bg-muted/60" />
            <div className="h-3 w-44 rounded bg-muted/40" />
          </CardHeader>
          <CardContent className="pt-0 space-y-2">
            <div className="h-8 w-full rounded bg-muted/30" />
            <div className="h-8 w-full rounded bg-muted/30" />
          </CardContent>
        </Card>
        <Card className="animate-pulse">
          <CardHeader className="pb-3 space-y-1.5">
            <div className="h-4 w-32 rounded bg-muted/60" />
            <div className="h-3 w-40 rounded bg-muted/40" />
          </CardHeader>
          <CardContent className="pt-0 space-y-2">
            <div className="h-8 w-full rounded bg-muted/30" />
            <div className="h-8 w-full rounded bg-muted/30" />
          </CardContent>
        </Card>
      </div>

      {/* Activity Logs Table Skeleton */}
      <div className="mt-6">
        <Card className="animate-pulse">
          <CardHeader className="pb-3 space-y-1.5">
            <div className="h-4 w-40 rounded bg-muted/60" />
            <div className="h-3 w-60 rounded bg-muted/40" />
          </CardHeader>
          <CardContent className="pt-0 space-y-2">
            <div className="h-10 w-full rounded bg-muted/40" />
            <div className="h-10 w-full rounded bg-muted/20" />
            <div className="h-10 w-full rounded bg-muted/20" />
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

export default function AgentDetailPage() {
  const params = useParams<{ id: string }>();
  const agentId = params?.id || "";
  const router = useRouter();

  const [copiedPrompt, setCopiedPrompt] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);

  const { data: agent, isLoading: agentLoading, isError, refetch } = useAgentDetailQuery(agentId);
  const { data: stats, isLoading: statsLoading } = useAgentStatsQuery(agentId);
  const deleteMutation = useDeleteAgent();

  const toolsQuery = useToolsQuery();
  const resourcesQuery = useResourcesQuery();
  const mcpServersQuery = useMCPServersQuery();

  const logsQuery = useLogsQuery({ agentId, limit: 8 });

  const allTools = (toolsQuery.data && Array.isArray(toolsQuery.data) ? toolsQuery.data : []) as Array<{ id: string; name: string; displayName?: string; description?: string }>;
  const allResources = (resourcesQuery.data && Array.isArray(resourcesQuery.data) ? resourcesQuery.data : []) as Array<{ id: string; name: string; displayName?: string; description?: string }>;
  const allMCPServers = (mcpServersQuery.data && Array.isArray(mcpServersQuery.data) ? mcpServersQuery.data : []) as Array<{ id: string; name: string; displayName?: string; transportType?: string; status?: string; enabled?: boolean }>;

  const toolOptions: MultiSelectOption[] = allTools
    .filter((t) => t.name)
    .map((t) => ({ value: t.name, label: t.displayName || t.name }));

  const resourceOptions: MultiSelectOption[] = allResources
    .filter((r) => r.name)
    .map((r) => ({ value: r.name, label: r.displayName || r.name }));

  const mcpServerOptions: MultiSelectOption[] = allMCPServers
    .filter((s) => s.name)
    .map((s) => ({ value: s.name, label: s.displayName || s.name }));

  const handleCopyPrompt = () => {
    if (!agent?.systemPromptOverride) return;
    navigator.clipboard.writeText(agent.systemPromptOverride);
    setCopiedPrompt(true);
    toast.success("System prompt copied to clipboard");
    setTimeout(() => setCopiedPrompt(false), 2000);
  };

  const handleDelete = () => {
    if (!agent) return;
    deleteMutation.mutate(agent.id, {
      onSuccess: () => {
        toast.success(`Agent "${agent.displayName || agent.name}" deleted`);
        router.push(AppRoutes.AGENTS);
      },
      onError: (err) => toast.error(`Failed to delete agent: ${getErrorMessage(err)}`),
    });
  };

  if (agentLoading) {
    return <AgentDetailSkeleton />;
  }

  if (isError || !agent) {
    return (
      <AppLayout>
        <PageHeader
          title="Agent Gateway"
          extra={
            <Button variant="outline" size="sm" asChild>
              <Link href={AppRoutes.AGENTS}>
                <ArrowLeft className="h-4 w-4" />
                <span>Back to Agents</span>
              </Link>
            </Button>
          }
        />
        <Card>
          <CardContent className="py-8">
            <ErrorState
              title="Agent Not Found"
              description="The requested AI agent could not be found or may have been deleted."
              onRetry={refetch}
            />
          </CardContent>
        </Card>
      </AppLayout>
    );
  }

  const spendCapUSD = (agent.maxBudgetCents ?? 0) / 100;
  const currentSpendUSD = stats?.totalCostUSD ?? 0;
  const spendPercentage = spendCapUSD > 0 ? Math.min(100, Math.round((currentSpendUSD / spendCapUSD) * 100)) : 0;

  // Estimate token length of prompt (~4 chars/token)
  const promptTokensEstimate = Math.ceil((agent.systemPromptOverride || "").length / 4);

  const logsList: ApiRequestLog[] = (logsQuery.data && Array.isArray(logsQuery.data.data))
    ? logsQuery.data.data
    : [];

  const logColumns: Column<ApiRequestLog>[] = [
    {
      title: "Timestamp",
      key: "createdAt",
      render: (_, record) => (
        <span className="font-mono text-xs text-muted-foreground whitespace-nowrap">
          {record.createdAt ? new Date(record.createdAt).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" }) : "-"}
        </span>
      ),
    },
    {
      title: "Model",
      dataIndex: "model",
      key: "model",
      render: (model) => (
        <Badge variant="outline" className="font-mono text-xs">
          {String(model || "prism-auto")}
        </Badge>
      ),
    },
    {
      title: "Status",
      dataIndex: "statusCode",
      key: "statusCode",
      render: (status) => (
        <Badge
          variant={Number(status) >= 200 && Number(status) < 300 ? "success" : "destructive"}
          className="font-mono text-[10px]"
        >
          {String(status || 200)}
        </Badge>
      ),
    },
    {
      title: "Latency",
      key: "latency",
      render: (_, record) => (
        <span className="font-mono text-xs text-muted-foreground whitespace-nowrap">
          {record.latencyMs ?? 0}ms
        </span>
      ),
    },
    {
      title: "Tokens",
      key: "tokens",
      render: (_, record) => (
        <span className="font-mono text-xs text-muted-foreground whitespace-nowrap">
          {formatTokens(record.totalTokens ?? 0)}
        </span>
      ),
    },
    {
      title: "Cost",
      key: "cost",
      render: (_, record) => (
        <span className="font-mono text-xs text-emerald-500 font-medium whitespace-nowrap">
          {formatUSD(record.costUSD ?? 0)}
        </span>
      ),
    },
  ];

  return (
    <AppLayout>
      <PageHeader
        title={agent.displayName || agent.name}
        description={agent.description || `Autonomous AI agent identity (type: ${agent.agentType || "assistant"}).`}
        extra={
          <div className="flex items-center gap-2 flex-wrap">
            <Button variant="outline" size="sm" asChild>
              <Link href={AppRoutes.AGENTS}>
                <ArrowLeft className="h-4 w-4" />
                <span>Back to Agents</span>
              </Link>
            </Button>
            <Button variant="outline" size="sm" asChild>
              <Link href={`${AppRoutes.SANDBOX}?agent=${agent.name}`}>
                <Play className="h-3.5 w-3.5 text-muted-foreground" />
                <span>Open in Sandbox</span>
              </Link>
            </Button>
            <Button variant="outline" size="sm" className="gap-1.5" onClick={() => setModalOpen(true)}>
              <Settings className="h-3.5 w-3.5" />
              <span>Edit Agent</span>
            </Button>
          </div>
        }
      />

      {/* Badges Metadata Bar */}
      <div className="mb-6 flex flex-wrap items-center gap-2">
        <Badge variant="outline" className="font-mono text-[10px] uppercase">
          <Bot className="mr-1 h-3 w-3 text-muted-foreground" />
          {agent.agentType || "assistant"}
        </Badge>
        <StatusDot status={agent.enabled ? "healthy" : "cooldown"} />
        <span className="capitalize text-xs font-mono text-muted-foreground">
          {agent.enabled ? "Active Boundary" : "Disabled"}
        </span>
        <Badge variant="outline" className="font-mono text-[10px]">
          Spend Cap: ${spendCapUSD.toFixed(2)}/mo
        </Badge>
        {agent.createdAt && (
          <span className="text-[11px] font-mono text-muted-foreground ml-auto">
            Created: {new Date(agent.createdAt).toLocaleDateString()}
          </span>
        )}
      </div>

      {/* 4 Metric Cards */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <MetricCard
          title="Total Requests"
          value={formatNumber(stats?.totalRequests ?? 0)}
          icon={<Zap className="h-4 w-4" />}
          subtitle="Last 30 days"
          loading={statsLoading}
        />
        <MetricCard
          title="Tokens Consumed"
          value={formatTokens(stats?.totalTokens ?? 0)}
          icon={<Cpu className="h-4 w-4" />}
          subtitle="Prompt & Output"
          loading={statsLoading}
        />
        <MetricCard
          title="Spend vs Cap"
          value={formatUSD(currentSpendUSD)}
          icon={<Terminal className="h-4 w-4" />}
          subtitle={spendCapUSD > 0 ? `${spendPercentage}% of $${spendCapUSD.toFixed(2)} cap` : "No limit set"}
          loading={statsLoading}
        />
        <MetricCard
          title="Tool & MCP Calls"
          value={formatNumber(stats?.toolCallsCount ?? 0)}
          icon={<Wrench className="h-4 w-4" />}
          subtitle={`${(agent.allowedTools || []).length} tools · ${(agent.allowedMcpServers || []).length} MCP bound`}
          loading={statsLoading}
        />
      </div>

      {/* Persona & System Prompt Directives */}
      <div className="mt-8">
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="text-sm flex items-center gap-2">
                  <Terminal className="h-4 w-4 text-muted-foreground" />
                  <span>Bound System Prompt & Persona</span>
                </CardTitle>
                <CardDescription>
                  Injected into the context boundary for every execution invoked under this agent identity.
                </CardDescription>
              </div>
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="font-mono text-[10px]">
                  ~{promptTokensEstimate} tokens
                </Badge>
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 text-xs gap-1.5"
                  onClick={handleCopyPrompt}
                >
                  {copiedPrompt ? (
                    <>
                      <Check className="h-3 w-3 text-emerald-500" />
                      <span>Copied</span>
                    </>
                  ) : (
                    <>
                      <Copy className="h-3 w-3" />
                      <span>Copy Prompt</span>
                    </>
                  )}
                </Button>
              </div>
            </div>
          </CardHeader>
          <CardContent className="pt-0">
            <div className="relative rounded-none border border-border bg-muted/40 p-4 font-mono text-xs leading-relaxed text-foreground whitespace-pre-wrap max-h-64 overflow-y-auto">
              {agent.systemPromptOverride || "No system prompt configured. Default router prompt will be applied."}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Bound Models Card */}
      <div className="mt-6">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm flex items-center gap-2">
              <Layers className="h-4 w-4 text-muted-foreground" />
              <span>Allowed LLM Models</span>
            </CardTitle>
            <CardDescription>
              Models this agent is permitted to query. Requests specifying other models will be blocked by Governance RBAC.
            </CardDescription>
          </CardHeader>
          <CardContent className="pt-0">
            {(agent.allowedModels || []).length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {(agent.allowedModels || []).map((m: string) => (
                  <Badge key={m} variant="outline" className="font-mono text-xs py-1 px-2.5 gap-1.5">
                    <Cpu className="h-3 w-3 text-muted-foreground" />
                    <span>{m}</span>
                  </Badge>
                ))}
              </div>
            ) : (
              <p className="text-xs text-muted-foreground font-mono">
                Unrestricted · Permitted to route to all workspace LLM models.
              </p>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Execution Boundaries 3-Grid (MCP Servers, Tools, Resources) */}
      <div className="mt-6 grid gap-4 lg:grid-cols-3">
        {/* Allowed MCP Servers */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm flex items-center gap-2">
              <Cpu className="h-4 w-4 text-muted-foreground" />
              <span>Allowed MCP Servers ({(agent.allowedMcpServers || []).length})</span>
            </CardTitle>
            <CardDescription>Model Context Protocol servers bound to this agent.</CardDescription>
          </CardHeader>
          <CardContent className="pt-0">
            {(agent.allowedMcpServers || []).length > 0 ? (
              <div className="space-y-2">
                {(agent.allowedMcpServers || []).map((sName: string) => {
                  const srv = allMCPServers.find((s) => s.name === sName);
                  return (
                    <div
                      key={sName}
                      className="flex items-center justify-between rounded-none border border-border bg-muted/40 p-2.5 text-xs"
                    >
                      <div className="flex items-center gap-2 min-w-0">
                        <Cpu className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                        <span className="font-mono font-medium truncate">{srv?.displayName || sName}</span>
                      </div>
                      {srv ? (
                        <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px] gap-1" asChild>
                          <Link href={mcpDetailRoute(srv.id)}>
                            <span>View</span>
                            <ExternalLink className="h-2.5 w-2.5" />
                          </Link>
                        </Button>
                      ) : (
                        <Badge variant="outline" className="font-mono text-[9px] uppercase">
                          Bound
                        </Badge>
                      )}
                    </div>
                  );
                })}
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">No MCP servers bound.</p>
            )}
          </CardContent>
        </Card>

        {/* Allowed Custom Tools */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm flex items-center gap-2">
              <Wrench className="h-4 w-4 text-muted-foreground" />
              <span>Allowed Tools ({(agent.allowedTools || []).length})</span>
            </CardTitle>
            <CardDescription>Native Tool Gateway functions permitted.</CardDescription>
          </CardHeader>
          <CardContent className="pt-0">
            {(agent.allowedTools || []).length > 0 ? (
              <div className="flex flex-wrap gap-1.5">
                {(agent.allowedTools || []).map((t: string) => (
                  <Badge key={t} variant="outline" className="font-mono text-[11px] py-0.5 px-2">
                    {t}
                  </Badge>
                ))}
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">No native tools bound.</p>
            )}
          </CardContent>
        </Card>

        {/* Allowed Resources */}
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm flex items-center gap-2">
              <Code2 className="h-4 w-4 text-muted-foreground" />
              <span>Allowed Resources ({(agent.allowedResources || []).length})</span>
            </CardTitle>
            <CardDescription>Resource Gateway documents and datasets accessible.</CardDescription>
          </CardHeader>
          <CardContent className="pt-0">
            {(agent.allowedResources || []).length > 0 ? (
              <div className="flex flex-wrap gap-1.5">
                {(agent.allowedResources || []).map((r: string) => (
                  <Badge key={r} variant="outline" className="font-mono text-[11px] py-0.5 px-2">
                    {r}
                  </Badge>
                ))}
              </div>
            ) : (
              <p className="text-xs text-muted-foreground">No resources bound.</p>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Recent Agent Activity Logs */}
      <div className="mt-6">
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="text-sm flex items-center gap-2">
                  <Zap className="h-4 w-4 text-muted-foreground" />
                  <span>Recent Agent Executions</span>
                </CardTitle>
                <CardDescription>Recent proxy requests made under this agent identity boundary.</CardDescription>
              </div>
              <Button variant="outline" size="sm" className="h-7 text-xs gap-1" asChild>
                <Link href={AppRoutes.LOGS}>
                  <span>All Logs</span>
                  <ExternalLink className="h-3 w-3" />
                </Link>
              </Button>
            </div>
          </CardHeader>
          <CardContent className="pt-0">
            {logsList.length > 0 ? (
              <DataTable
                dataSource={logsList}
                columns={logColumns}
                rowKey="id"
                loading={logsQuery.isLoading}
                searchable={false}
                pageSize={6}
                onRefresh={logsQuery.refetch}
              />
            ) : (
              <EmptyState
                title="No Activity Logs Yet"
                description="This agent has not executed any requests through the proxy gateway yet."
                action={
                  <Button variant="prismViolet" size="sm" asChild>
                    <Link href={`${AppRoutes.SANDBOX}?agent=${agent.name}`}>
                      <Play className="mr-1.5 h-3.5 w-3.5" />
                      <span>Test in AI Sandbox</span>
                    </Link>
                  </Button>
                }
              />
            )}
          </CardContent>
        </Card>
      </div>

      {/* Delete Agent Footer Action */}
      <div className="mt-8 flex justify-end">
        <ConfirmDialog
          title="Delete Agent Identity"
          description={`Are you sure you want to permanently delete agent "${agent.displayName || agent.name}"? This action cannot be undone.`}
          confirmText="Delete Agent"
          onConfirm={handleDelete}
          trigger={
            <Button variant="destructive" size="sm" className="gap-1.5" disabled={deleteMutation.isPending}>
              <Trash2 className="h-4 w-4" />
              <span>Delete Agent</span>
            </Button>
          }
        />
      </div>

      {/* Edit Agent Dialog */}
      <AgentFormDialog
        open={modalOpen}
        onOpenChange={setModalOpen}
        editingAgent={agent}
        toolOptions={toolOptions}
        resourceOptions={resourceOptions}
        mcpServerOptions={mcpServerOptions}
      />
    </AppLayout>
  );
}
