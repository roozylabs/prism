"use client";

import { useState } from "react";
import Link from "next/link";
import { AppLayout } from "@/components/AppLayout";
import { PageHeader } from "@/components/molecules/PageHeader";
import { Card, CardHeader, CardTitle, CardContent, CardDescription, CardFooter } from "@/components/molecules/Card";
import { Button } from "@/components/atoms/Button";
import { Badge, StatusDot } from "@/components/atoms/Badge";
import { useAgentsQuery, useDeleteAgent } from "@/hooks/queries/useAgentsQuery";
import { useToolsQuery } from "@/hooks/queries/useToolsQuery";
import { useResourcesQuery } from "@/hooks/queries/useResourcesQuery";
import { useMCPServersQuery } from "@/hooks/queries/useMCPServersQuery";
import { ApiAgent } from "@/lib/api";
import { ErrorState, EmptyState } from "@/components/molecules/StateAlerts";
import { CardSkeletonGrid } from "@/components/molecules/CardSkeleton";
import { Bot, Plus, Settings, Trash2, ArrowUpRight, Eye } from "lucide-react";
import { toast } from "sonner";
import { getErrorMessage } from "@/types/ui";
import { ConfirmDialog } from "@/components/molecules/ConfirmDialog";
import { MultiSelectOption } from "@/components/molecules/MultiSelect";
import { agentDetailRoute } from "@/constants/routes";
import { AgentFormDialog } from "./_components/AgentFormDialog";

function formatBudgetCents(cents: number): string {
  return `$${(cents / 100).toFixed(2)}/mo`;
}

export default function AgentsPage() {
  const { data, isLoading, isError, refetch } = useAgentsQuery();
  const deleteMutation = useDeleteAgent();
  const toolsData = useToolsQuery();
  const resourcesData = useResourcesQuery();
  const mcpServersData = useMCPServersQuery();

  const toolOptions: MultiSelectOption[] = ((toolsData.data && Array.isArray(toolsData.data) ? toolsData.data : []) as Array<{ name: string; displayName?: string }>)
    .filter((t) => t.name)
    .map((t) => ({ value: t.name, label: t.displayName || t.name }));

  const resourceOptions: MultiSelectOption[] = ((resourcesData.data && Array.isArray(resourcesData.data) ? resourcesData.data : []) as Array<{ name: string; displayName?: string }>)
    .filter((r) => r.name)
    .map((r) => ({ value: r.name, label: r.displayName || r.name }));

  const mcpServerOptions: MultiSelectOption[] = ((mcpServersData.data && Array.isArray(mcpServersData.data) ? mcpServersData.data : []) as Array<{ name: string; displayName?: string }>)
    .filter((s) => s.name)
    .map((s) => ({ value: s.name, label: s.displayName || s.name }));

  const [modalOpen, setModalOpen] = useState(false);
  const [editingAgent, setEditingAgent] = useState<ApiAgent | null>(null);

  const openCreateDrawer = () => {
    setEditingAgent(null);
    setModalOpen(true);
  };

  const openEditDrawer = (agent: ApiAgent) => {
    setEditingAgent(agent);
    setModalOpen(true);
  };

  const handleDelete = (agent: ApiAgent) => {
    deleteMutation.mutate(agent.id, {
      onSuccess: () => toast.success(`Agent "${agent.name}" deleted`),
      onError: (err) => toast.error(`Failed to delete: ${getErrorMessage(err)}`),
    });
  };

  const agents: ApiAgent[] = (data && Array.isArray(data)) ? data : [];
  const isPending = deleteMutation.isPending;

  return (
    <AppLayout>
      <PageHeader
        title="Agent Gateway & Agent Catalog"
        description="Provision autonomous AI agent identities with bound system prompts, tool boundaries, and key quotas."
        extra={
          <Button variant="prismViolet" size="sm" className="gap-1.5" onClick={openCreateDrawer}>
            <Plus className="h-4 w-4" /> Instantiate New Agent
          </Button>
        }
      />

      {isError ? (
        <ErrorState
          title="Failed to fetch agents"
          description="Could not communicate with the Prism Agent Gateway backend."
          onRetry={refetch}
        />
      ) : isLoading ? (
        <CardSkeletonGrid count={3} />
      ) : agents.length === 0 ? (
        <EmptyState
          title="No Agents Configured"
          description="There are no AI agents provisioned in this workspace yet."
          action={
            <Button variant="prismViolet" size="sm" className="gap-1.5" onClick={openCreateDrawer}>
              <Plus className="h-4 w-4" /> Instantiate New Agent
            </Button>
          }
        />
      ) : (
        <div className="grid gap-6 md:grid-cols-1 lg:grid-cols-2 xl:grid-cols-3">
          {agents.map((agent) => (
            <Card key={agent.id} className="flex flex-col justify-between hover:border-foreground/40 transition-colors">
              <CardHeader>
                <div className="flex items-center justify-between">
                  <CardTitle className="text-base font-bold">
                    <Link
                      href={agentDetailRoute(agent.id)}
                      className="flex items-center gap-2 hover:text-foreground transition-colors group"
                    >
                      <Bot className="h-5 w-5 text-muted-foreground" />
                      <span>{agent.displayName || agent.name}</span>
                      <ArrowUpRight className="h-3.5 w-3.5 opacity-0 group-hover:opacity-100 transition-opacity text-muted-foreground" />
                    </Link>
                  </CardTitle>
                  <StatusDot status={agent.enabled ? "healthy" : "cooldown"} />
                </div>
                <CardDescription className="font-mono text-xs">
                  {agent.description || `type: ${agent.agentType}`}
                </CardDescription>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex items-center justify-between text-xs border-b border-border pb-2">
                  <span className="text-muted-foreground">Status</span>
                  <Badge variant={agent.enabled ? "success" : "default"}>
                    {agent.enabled ? "Enabled" : "Disabled"}
                  </Badge>
                </div>
                {(agent.allowedModels || []).length > 0 && (
                  <div className="flex items-center justify-between text-xs">
                    <span className="text-muted-foreground">Models</span>
                    <div className="flex gap-1 flex-wrap justify-end max-w-[180px]">
                      {(agent.allowedModels || []).slice(0, 3).map((m: string) => (
                        <Badge key={m} variant="outline" className="text-[10px]">
                          {m}
                        </Badge>
                      ))}
                      {(agent.allowedModels || []).length > 3 && (
                        <Badge variant="outline" className="text-[10px]">
                          +{(agent.allowedModels || []).length - 3}
                        </Badge>
                      )}
                    </div>
                  </div>
                )}
                <div className="flex items-center justify-between text-xs">
                  <span className="text-muted-foreground">Spend Cap</span>
                  <span className="font-mono text-emerald-500 font-bold">
                    {formatBudgetCents(agent.maxBudgetCents ?? 0)}
                  </span>
                </div>
                <div className="flex items-center justify-between text-xs border-t border-border pt-2">
                  <span className="text-muted-foreground">Bindings</span>
                  <div className="flex gap-1 flex-wrap justify-end">
                    <Badge variant="outline" className="text-[10px]">
                      {agent.allowedTools?.length ?? 0} tools
                    </Badge>
                    <Badge variant="outline" className="text-[10px]">
                      {agent.allowedResources?.length ?? 0} resources
                    </Badge>
                    <Badge variant="outline" className="text-[10px]">
                      {agent.allowedMcpServers?.length ?? 0} MCP
                    </Badge>
                  </div>
                </div>
              </CardContent>
              <CardFooter className="border-t border-border pt-3 gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  className="flex-1 gap-1.5 text-xs"
                  asChild
                >
                  <Link href={agentDetailRoute(agent.id)}>
                    <Eye className="h-3.5 w-3.5" /> Inspect
                  </Link>
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  className="flex-1 gap-1.5 text-xs"
                  onClick={() => openEditDrawer(agent)}
                >
                  <Settings className="h-3.5 w-3.5" /> Edit
                </Button>
                <ConfirmDialog
                  title="Delete Agent"
                  description={`Delete agent "${agent.name}"? This cannot be undone.`}
                  confirmText="Delete"
                  onConfirm={() => handleDelete(agent)}
                  trigger={
                    <Button
                      variant="destructive"
                      size="sm"
                      className="px-2.5 text-xs"
                      disabled={isPending}
                    >
                      <Trash2 className="h-3.5 w-3.5" />
                    </Button>
                  }
                />
              </CardFooter>
            </Card>
          ))}
        </div>
      )}

      <AgentFormDialog
        open={modalOpen}
        onOpenChange={setModalOpen}
        editingAgent={editingAgent}
        toolOptions={toolOptions}
        resourceOptions={resourceOptions}
        mcpServerOptions={mcpServerOptions}
      />
    </AppLayout>
  );
}
