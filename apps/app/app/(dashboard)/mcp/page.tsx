"use client";

import { useState } from "react";
import Link from "next/link";
import { AppLayout } from "@/components/AppLayout";
import { PageHeader } from "@/components/molecules/PageHeader";
import {
  Card,
  CardHeader,
  CardContent,
  CardDescription,
  CardFooter,
} from "@/components/molecules/Card";
import { Button } from "@/components/atoms/Button";
import { Badge, StatusDot } from "@/components/atoms/Badge";
import { Switch } from "@/components/atoms/Switch";
import { ConfirmDialog } from "@/components/molecules/ConfirmDialog";
import { ErrorState, EmptyState } from "@/components/molecules/StateAlerts";
import { CardSkeletonGrid } from "@/components/molecules/CardSkeleton";
import { useMCPServersQuery } from "@/hooks/queries/useMCPServersQuery";
import { ApiMCPServer } from "@/lib/api";
import { mcpDetailRoute } from "@/constants/routes";
import {
  useUpdateMCPServerMutation,
  useDeleteMCPServerMutation,
  useSyncMCPServerMutation,
} from "@/hooks/mutations/useMCPMutations";
import { MCPServerFormDialog } from "./_components/MCPServerFormDialog";
import { MCPTestModal } from "./_components/MCPTestModal";
import {
  Globe,
  Plus,
  RefreshCw,
  Pencil,
  Trash2,
  Loader2,
  Play,
  Cpu,
  Activity,
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

export default function MCPPage() {
  const { data, isLoading, isError, refetch } = useMCPServersQuery();
  const updateMutation = useUpdateMCPServerMutation();
  const deleteMutation = useDeleteMCPServerMutation();
  const syncMutation = useSyncMCPServerMutation();

  const [modalOpen, setModalOpen] = useState(false);
  const [editingServer, setEditingServer] = useState<ApiMCPServer | null>(null);

  const [testModalOpen, setTestModalOpen] = useState(false);
  const [testingServer, setTestingServer] = useState<ApiMCPServer | null>(null);

  const [syncingId, setSyncingId] = useState<string | null>(null);

  const servers: ApiMCPServer[] = data && Array.isArray(data) ? data : [];

  const openCreate = () => {
    setEditingServer(null);
    setModalOpen(true);
  };

  const openEdit = (server: ApiMCPServer) => {
    setEditingServer(server);
    setModalOpen(true);
  };

  const openTestModal = (server: ApiMCPServer) => {
    setTestingServer(server);
    setTestModalOpen(true);
  };

  if (isLoading) {
    return (
      <AppLayout>
        <PageHeader
          title="MCP Gateway"
          description="Model Context Protocol servers providing external tools, prompts, and resources to agents."
        />
        <CardSkeletonGrid count={3} />
      </AppLayout>
    );
  }

  if (isError) {
    return (
      <AppLayout>
        <PageHeader
          title="MCP Gateway"
          description="Model Context Protocol servers providing external tools, prompts, and resources to agents."
        />
        <Card>
          <CardContent className="py-8">
            <ErrorState
              title="Failed to load MCP servers"
              onRetry={() => refetch()}
            />
          </CardContent>
        </Card>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <PageHeader
        title="MCP Gateway"
        description="Model Context Protocol servers providing external tools, prompts, and resources to agents."
        extra={
          <Button variant="prismViolet" onClick={openCreate} className="gap-2">
            <Plus className="h-4 w-4" />
            <span>Register MCP Server</span>
          </Button>
        }
      />

      {servers.length === 0 ? (
        <EmptyState
          title="No MCP Servers Registered"
          description="Connect Model Context Protocol (MCP) endpoints to expand agent capabilities."
          action={
            <Button variant="prismViolet" onClick={openCreate}>
              Register MCP Server
            </Button>
          }
        />
      ) : (
        <div className="grid gap-6 md:grid-cols-1 lg:grid-cols-2 xl:grid-cols-3">
          {servers.map((s) => {
            const isSyncingThis = syncMutation.isPending && syncingId === s.id;
            return (
              <Card key={s.id} className="flex flex-col justify-between">
                <CardHeader className="pb-3">
                  <div className="flex items-start justify-between">
                    <div className="flex items-center gap-2">
                      <Globe className="h-4 w-4 text-muted-foreground" />
                      <Link
                        href={mcpDetailRoute(s.id)}
                        className="font-semibold text-sm hover:text-foreground hover:underline"
                      >
                        {s.displayName || s.name}
                      </Link>
                    </div>
                    <div className="flex items-center gap-1.5">
                      <Badge
                        variant="outline"
                        className="font-mono text-[10px] uppercase"
                      >
                        {s.type === "local" ? (
                          <Cpu className="mr-1 h-3 w-3" />
                        ) : (
                          <Globe className="mr-1 h-3 w-3" />
                        )}
                        {s.type}
                      </Badge>
                      <StatusDot status={statusToDot(s.status || s.healthStatus || 'unknown')} />
                      <span className="capitalize text-xs font-mono text-muted-foreground">
                        {s.status || s.healthStatus || 'unknown'}
                      </span>
                    </div>
                  </div>
                  <CardDescription className="line-clamp-2 mt-1">
                    {s.description || "No description"}
                  </CardDescription>
                </CardHeader>
                <CardContent className="space-y-3 pt-0 text-xs">
                  <div className="p-2 rounded border border-border bg-muted/40 font-mono text-[11px] truncate">
                    {s.type === "local" ? (
                      <>
                        <span className="text-muted-foreground">Command: </span>
                        <span className="text-foreground">{s.command}</span>
                        {(s.args || []).length > 0 && (
                          <span className="ml-2 text-muted-foreground">
                            {" "}
                            args=[{(s.args || []).join(", ")}]
                          </span>
                        )}
                      </>
                    ) : (
                      <>
                        <span className="text-muted-foreground">Endpoint: </span>
                        <span className="text-foreground">{s.endpointUrl}</span>
                      </>
                    )}
                  </div>
                  <div className="flex items-center justify-between text-muted-foreground">
                    <span>
                      Transport:{" "}
                      <Badge
                        variant="outline"
                        className="font-mono text-[10px] uppercase"
                      >
                        {s.transportType}
                      </Badge>
                    </span>
                    <span className="flex items-center gap-1.5">
                      <span>Enabled</span>
                      <Switch
                        checked={s.enabled}
                        onCheckedChange={(checked) => {
                          updateMutation.mutate(
                            { id: s.id, data: { enabled: checked } },
                            {
                              onSuccess: () =>
                                toast.success(
                                  `MCP server ${checked ? "enabled" : "disabled"}`,
                                ),
                              onError: (err) =>
                                toast.error(
                                  `Update failed: ${getErrorMessage(err)}`,
                                ),
                            },
                          );
                        }}
                        disabled={updateMutation.isPending}
                        aria-label="Toggle MCP server enabled"
                      />
                    </span>
                  </div>
                </CardContent>
                <CardFooter className="pt-3 border-t border-border flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-7 text-xs gap-1"
                      asChild
                    >
                      <Link href={mcpDetailRoute(s.id)}>
                        <Activity className="h-3 w-3 text-muted-foreground" />
                        <span>Details</span>
                      </Link>
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-7 text-xs gap-1"
                      onClick={() => openTestModal(s)}
                    >
                      <Play className="h-3 w-3 text-muted-foreground" />
                      <span>Test</span>
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      className="h-7 text-xs gap-1"
                      onClick={() => {
                        setSyncingId(s.id);
                        syncMutation.mutate(s.id, {
                          onSuccess: () =>
                            toast.success("MCP server tools synced"),
                          onError: (err) =>
                            toast.error(`Sync failed: ${getErrorMessage(err)}`),
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
                  </div>
                  <div className="flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-7 w-7 p-0 text-muted-foreground hover:text-foreground"
                      onClick={() => openEdit(s)}
                    >
                      <Pencil className="h-3.5 w-3.5" />
                    </Button>
                    <ConfirmDialog
                      title="Delete MCP Server"
                      description={`Are you sure you want to unregister MCP server "${s.displayName || s.name}"?`}
                      confirmText="Delete"
                      variant="destructive"
                      onConfirm={() => {
                        deleteMutation.mutate(s.id, {
                          onSuccess: () => toast.success("MCP server removed"),
                          onError: (err) =>
                            toast.error(`Delete failed: ${getErrorMessage(err)}`),
                        });
                      }}
                      trigger={
                        <Button
                          variant="ghost"
                          size="sm"
                          className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive"
                        >
                          <Trash2 className="h-3.5 w-3.5" />
                        </Button>
                      }
                    />
                  </div>
                </CardFooter>
              </Card>
            );
          })}
        </div>
      )}

      <MCPServerFormDialog
        open={modalOpen}
        onOpenChange={setModalOpen}
        editingServer={editingServer}
      />

      <MCPTestModal
        open={testModalOpen}
        onOpenChange={setTestModalOpen}
        server={testingServer}
      />
    </AppLayout>
  );
}
