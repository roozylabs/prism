"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

import { Button } from "@/components/atoms/Button";
import { Input } from "@/components/atoms/Input";
import { Textarea } from "@/components/atoms/Textarea";
import { Label } from "@/components/atoms/Label";
import { Switch } from "@/components/atoms/Switch";
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from "@/components/molecules/Select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  DialogDescription,
} from "@/components/molecules/Dialog";
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from "@/components/molecules/Form";
import { useMCPServerEditQuery } from "@/hooks/queries/useMCPServersQuery";
import {
  useCreateMCPServerMutation,
  useUpdateMCPServerMutation,
} from "@/hooks/mutations/useMCPMutations";
import { ApiMCPServer, ApiCreateMCPServerRequest } from "@/lib/api";
import { Plus, Minus, Cpu, KeyRound, Globe } from "lucide-react";
import { toast } from "sonner";
import { getErrorMessage } from "@/types/ui";

interface KeyValueRow {
  key: string;
  value: string;
}

import { mcpServerSchema, MCPServerFormValues } from "@/features/mcp/schemas/create-mcp-server.schema";

interface MCPServerFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  editingServer: ApiMCPServer | null;
}

function kvToMap(rows: KeyValueRow[]): Record<string, string> {
  const out: Record<string, string> = {};
  for (const row of rows) {
    const k = row.key.trim();
    if (k && row.value.trim()) {
      out[k] = row.value.trim();
    }
  }
  return out;
}

function csvToArgs(val: string): string[] {
  if (!val.trim()) return [];
  return val
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);
}

const defaultValues: MCPServerFormValues = {
  name: "",
  displayName: "",
  description: "",
  type: "remote",
  transportType: "http",
  endpointUrl: "",
  authToken: "",
  command: "",
  argsCsv: "",
  headerRows: [],
  envRows: [],
  enabled: true,
};

export function MCPServerFormDialog({
  open,
  onOpenChange,
  editingServer,
}: MCPServerFormDialogProps) {
  const createMutation = useCreateMCPServerMutation();
  const updateMutation = useUpdateMCPServerMutation();
  const [editingId, setEditingId] = useState("");

  const editQuery = useMCPServerEditQuery(editingId);
  const isEditLoading = Boolean(editingId) && editQuery.isLoading;

  const form = useForm<MCPServerFormValues>({
    resolver: zodResolver(mcpServerSchema),
    defaultValues,
  });

  const { reset, watch } = form;

  useEffect(() => {
    if (!open) return;
    if (!editingServer) {
      setEditingId("");
      reset(defaultValues);
      return;
    }
    setEditingId(editingServer.id);
  }, [open, editingServer, reset]);

  useEffect(() => {
    if (!editingId || !editQuery.data) return;
    const s = editQuery.data;

    let extractedAuthToken = "";
    const filteredHeaderRows: KeyValueRow[] = [];
    const headersMap: Record<string, string> = (s.headers || {}) as Record<string, string>;

    Object.entries(headersMap).forEach(([k, v]) => {
      if (k.toLowerCase() === "authorization") {
        let val = String(v ?? "").trim();
        while (/^bearer\s+/i.test(val)) {
          val = val.replace(/^bearer\s+/i, "").trim();
        }
        extractedAuthToken = val;
      } else {
        filteredHeaderRows.push({ key: k, value: String(v ?? "") });
      }
    });

    const envMap: Record<string, string> = (s.env || s.envVars || {}) as Record<string, string>;
    const envRowsList: KeyValueRow[] = Object.entries(envMap).map(([k, v]) => ({ key: k, value: String(v ?? "") }));

    reset({
      name: s.name,
      displayName: s.displayName || s.name,
      description: s.description || "",
      type: s.type === "local" ? "local" : "remote",
      transportType: s.transportType,
      endpointUrl: s.endpointUrl || s.serverUrl || "",
      authToken: extractedAuthToken,
      command: s.command || "",
      argsCsv: (s.args || []).join("\n"),
      headerRows: filteredHeaderRows,
      envRows: envRowsList,
      enabled: s.enabled,
    });
  }, [editQuery.data, editingId, reset]);

  const serverType = watch("type");
  const headerRows = watch("headerRows");
  const envRows = watch("envRows");

  const isSaving = createMutation.isPending || updateMutation.isPending;

  const onSubmit = (values: MCPServerFormValues) => {
    let cleanAuthToken = values.authToken.trim();
    while (/^bearer\s+/i.test(cleanAuthToken)) {
      cleanAuthToken = cleanAuthToken.replace(/^bearer\s+/i, "").trim();
    }

    const cleanedHeaderRows = values.headerRows.map((row) => {
      let val = row.value.trim();
      if (row.key.trim().toLowerCase() === "authorization") {
        while (/^bearer\s+/i.test(val)) {
          val = val.replace(/^bearer\s+/i, "").trim();
        }
        if (val) {
          val = `Bearer ${val}`;
        }
      }
      return { key: row.key.trim(), value: val };
    });

    const payload: ApiCreateMCPServerRequest = {
      name: values.name.trim(),
      displayName: values.displayName.trim() || values.name.trim(),
      description: values.description.trim(),
      type: values.type,
      transportType: values.transportType as 'stdio' | 'sse' | 'http',
      endpointUrl: values.endpointUrl.trim(),
      authToken: cleanAuthToken || undefined,
      headers: kvToMap(cleanedHeaderRows),
      command: values.command.trim(),
      args: csvToArgs(values.argsCsv),
      env: kvToMap(values.envRows),
      enabled: values.enabled,
    };

    if (editingServer) {
      updateMutation.mutate(
        { id: editingServer.id, data: payload },
        {
          onSuccess: () => {
            toast.success("MCP server updated");
            onOpenChange(false);
          },
          onError: (err) => toast.error(`Update failed: ${getErrorMessage(err)}`),
        },
      );
    } else {
      createMutation.mutate(payload, {
        onSuccess: () => {
          toast.success("MCP server registered");
          onOpenChange(false);
        },
        onError: (err) =>
          toast.error(`Registration failed: ${getErrorMessage(err)}`),
      });
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>
            {editingServer ? "Edit MCP Server" : "Register MCP Server"}
          </DialogTitle>
          <DialogDescription>
            Configure Model Context Protocol endpoint for agent tool calls.
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4 py-4"
          >
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Server Identifier Name</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder="e.g., github-mcp"
                      disabled={isSaving || isEditLoading}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="displayName"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Display Name</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder="e.g., GitHub Tools MCP Server"
                      disabled={isSaving || isEditLoading}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="description"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Description</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder="Capabilities provided by this MCP server..."
                      disabled={isSaving || isEditLoading}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="type"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Server Type</FormLabel>
                  <FormControl>
                    <Select
                      value={field.value}
                      onValueChange={(v) =>
                        field.onChange(v === "local" ? "local" : "remote")
                      }
                      disabled={isSaving || isEditLoading}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Select Server Type" />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value="remote">Remote (URL-based)</SelectItem>
                        <SelectItem value="local">Local (stdio command)</SelectItem>
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {serverType === "remote" ? (
              <>
                <FormField
                  control={form.control}
                  name="endpointUrl"
                  render={({ field }) => (
                    <FormItem className="space-y-2">
                      <FormLabel required>Endpoint URL</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder="https://mcp.firecrawl.dev/v2/mcp"
                          disabled={isSaving || isEditLoading}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="transportType"
                  render={({ field }) => (
                    <FormItem className="space-y-2">
                      <FormLabel>Transport Protocol</FormLabel>
                      <FormControl>
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                          disabled={isSaving || isEditLoading}
                        >
                          <SelectTrigger>
                            <SelectValue placeholder="Select Transport" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectItem value="http">
                              HTTP POST JSON-RPC (Recommended)
                            </SelectItem>
                            <SelectItem value="sse">
                              SSE (Server-Sent Events)
                            </SelectItem>
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <p className="text-[11px] text-muted-foreground">
                        HTTP POST JSON-RPC is recommended for Context7, Firecrawl, and standard web MCP endpoints.
                      </p>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="authToken"
                  render={({ field }) => (
                    <FormItem className="space-y-2">
                      <FormLabel>
                        <span className="flex items-center gap-1">
                          <KeyRound className="h-3.5 w-3.5 text-muted-foreground" />
                          Bearer Auth Token (Optional)
                        </span>
                      </FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type="password"
                          placeholder="e.g., fc-b123... (Bearer prefix added automatically)"
                          disabled={isSaving || isEditLoading}
                        />
                      </FormControl>
                      {editingServer && editQuery.data?.hasAuthToken && (
                        <p className="text-[11px] text-muted-foreground">
                          An existing token is set and will be kept; leave blank
                          to retain it.
                        </p>
                      )}
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <div className="space-y-2 rounded-md border border-border p-3">
                  <div className="flex items-center justify-between">
                    <Label className="text-xs font-semibold flex items-center gap-1">
                      <KeyRound className="h-3.5 w-3.5 text-muted-foreground" />
                      Additional Headers (Optional)
                    </Label>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-6 text-xs gap-1 text-muted-foreground"
                      onClick={() =>
                        form.setValue("headerRows", [
                          ...headerRows,
                          { key: "", value: "" },
                        ])
                      }
                      disabled={isSaving || isEditLoading}
                    >
                      <Plus className="h-3 w-3" /> Add
                    </Button>
                  </div>
                  {headerRows.length === 0 && (
                    <p className="text-[11px] text-muted-foreground">
                      e.g., X-Api-Key for firecrawl-style config.
                    </p>
                  )}
                  <div className="space-y-2">
                    {headerRows.map((row, idx) => (
                      <div key={idx} className="flex items-center gap-2">
                        <Input
                          className="h-8 flex-1 font-mono text-xs"
                          placeholder="Header name"
                          value={row.key}
                          onChange={(e) => {
                            const rows = [...headerRows];
                            rows[idx] = { ...rows[idx], key: e.target.value };
                            form.setValue("headerRows", rows);
                          }}
                          disabled={isSaving || isEditLoading}
                        />
                        <Input
                          className="h-8 flex-1 font-mono text-xs"
                          placeholder="Header value"
                          value={row.value}
                          onChange={(e) => {
                            const rows = [...headerRows];
                            rows[idx] = { ...rows[idx], value: e.target.value };
                            form.setValue("headerRows", rows);
                          }}
                          disabled={isSaving || isEditLoading}
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="h-8 w-8 p-0 text-muted-foreground hover:text-destructive"
                          onClick={() =>
                            form.setValue(
                              "headerRows",
                              headerRows.filter((_, i) => i !== idx),
                            )
                          }
                          disabled={isSaving || isEditLoading}
                        >
                          <Minus className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    ))}
                  </div>
                </div>
              </>
            ) : (
              <>
                <FormField
                  control={form.control}
                  name="command"
                  render={({ field }) => (
                    <FormItem className="space-y-2">
                      <FormLabel>
                        <span className="flex items-center gap-1">
                          <Cpu className="h-3.5 w-3.5 text-muted-foreground" />
                          Command
                        </span>
                      </FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          placeholder="e.g., npx -y @modelcontextprotocol/server-github"
                          disabled={isSaving || isEditLoading}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name="argsCsv"
                  render={({ field }) => (
                    <FormItem className="space-y-2">
                      <FormLabel>Arguments</FormLabel>
                      <FormControl>
                        <Textarea
                          counter
                          counterLabel="characters"
                          className="w-full h-20 rounded-md border border-border bg-background p-2 text-xs font-mono text-foreground focus:outline-none focus:ring-1 focus:ring-ring"
                          {...field}
                          placeholder={"One argument per line, e.g.\n--token\nxxx"}
                          disabled={isSaving || isEditLoading}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <div className="space-y-2 rounded-md border border-border p-3">
                  <div className="flex items-center justify-between">
                    <Label className="text-xs font-semibold">
                      <span className="flex items-center gap-1">
                        <Globe className="h-3.5 w-3.5 text-muted-foreground" />
                        Environment
                      </span>
                    </Label>
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      className="h-6 text-xs gap-1 text-muted-foreground"
                      onClick={() =>
                        form.setValue("envRows", [
                          ...envRows,
                          { key: "", value: "" },
                        ])
                      }
                      disabled={isSaving || isEditLoading}
                    >
                      <Plus className="h-3 w-3" /> Add
                    </Button>
                  </div>
                  <div className="space-y-2">
                    {envRows.map((row, idx) => (
                      <div key={idx} className="flex items-center gap-2">
                        <Input
                          className="h-8 flex-1 font-mono text-xs"
                          placeholder="VAR"
                          value={row.key}
                          onChange={(e) => {
                            const rows = [...envRows];
                            rows[idx] = { ...rows[idx], key: e.target.value };
                            form.setValue("envRows", rows);
                          }}
                          disabled={isSaving || isEditLoading}
                        />
                        <Input
                          className="h-8 flex-1 font-mono text-xs"
                          placeholder="value"
                          value={row.value}
                          onChange={(e) => {
                            const rows = [...envRows];
                            rows[idx] = { ...rows[idx], value: e.target.value };
                            form.setValue("envRows", rows);
                          }}
                          disabled={isSaving || isEditLoading}
                        />
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          className="h-8 w-8 p-0 text-muted-foreground hover:text-destructive"
                          onClick={() =>
                            form.setValue(
                              "envRows",
                              envRows.filter((_, i) => i !== idx),
                            )
                          }
                          disabled={isSaving || isEditLoading}
                        >
                          <Minus className="h-3.5 w-3.5" />
                        </Button>
                      </div>
                    ))}
                  </div>
                </div>
              </>
            )}

            <div className="flex items-center justify-between rounded-md border border-border p-3">
              <div className="space-y-0.5">
                <Label htmlFor="mcp-enabled" className="text-sm font-medium">
                  Enable Server
                </Label>
                <p className="text-[11px] text-muted-foreground">
                  Disabled servers will not accept tool calls.
                </p>
              </div>
              <Switch
                id="mcp-enabled"
                checked={form.watch("enabled")}
                onCheckedChange={(v) => form.setValue("enabled", v)}
                disabled={isSaving || isEditLoading}
              />
            </div>

            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
                disabled={isSaving}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                variant="prismViolet"
                disabled={isSaving || isEditLoading}
              >
                {isSaving
                  ? "Saving..."
                  : isEditLoading
                    ? "Loading..."
                    : editingServer
                      ? "Save Changes"
                      : "Register MCP Server"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
