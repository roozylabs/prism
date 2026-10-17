"use client";

import { useEffect, useMemo, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";

import { Button } from "@/components/atoms/Button";
import { Input } from "@/components/atoms/Input";
import { Label } from "@/components/atoms/Label";
import { Badge } from "@/components/atoms/Badge";
import { Switch } from "@/components/atoms/Switch";
import { Textarea } from "@/components/atoms/Textarea";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/molecules/Select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
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
import { useTestMCPToolMutation } from "@/hooks/mutations/useMCPMutations";
import { useMCPServerToolsQuery } from "@/hooks/queries/useMCPServersQuery";
import { ApiMCPServer, ApiMCPTool, ApiMCPToolExecutionResult } from "@/lib/api";
import { Play, Terminal, Loader2 } from "lucide-react";
import { toast } from "sonner";
import { getErrorMessage, JsonObject, JsonValue } from "@/types/ui";

const testSchema = z.object({
  toolName: z.string().min(1, "Tool name is required"),
  argsJson: z.string().default("{}"),
});

type TestFormValues = z.infer<typeof testSchema>;

type InputMode = "guided" | "raw";

interface MCPTestModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  server: ApiMCPServer | null;
}

function schemaProperties(schema: JsonObject | undefined): JsonObject {
  if (!schema) return {};
  const props = schema.properties;
  return props && typeof props === "object" ? (props as JsonObject) : {};
}

function schemaRequired(schema: JsonObject | undefined): string[] {
  if (!schema) return [];
  if (!Array.isArray(schema.required)) return [];
  return schema.required.filter((r): r is string => typeof r === "string");
}

function defaultForProp(prop: JsonObject): JsonValue {
  if (prop.default !== undefined) return prop.default as JsonValue;
  if (prop.type === "number" || prop.type === "integer") {
    if (prop.minimum !== undefined && typeof prop.minimum === "number" && prop.minimum > 0) {
      return prop.minimum;
    }
    return "";
  }
  switch (prop.type) {
    case "boolean":
      return false;
    default:
      return "";
  }
}

function coerceArg(prop: JsonObject, raw: JsonValue): JsonValue {
  const type = prop.type;
  if (type === "integer" || type === "number") {
    if (raw === "" || raw === null || raw === undefined) {
      return "";
    }
    const parsed = Number(raw);
    return Number.isNaN(parsed) ? raw : parsed;
  }
  if (type === "boolean") {
    return raw === true || raw === "true";
  }
  if (type === "array" || type === "object") {
    const value = raw as string;
    const trimmed = typeof value === "string" ? value.trim() : "";
    if (trimmed === "") return "";
    try {
      return JSON.parse(trimmed) as JsonValue;
    } catch {
      return raw;
    }
  }
  return raw;
}

function buildGuidedArgs(
  schema: JsonObject | undefined,
  args: JsonObject,
): Record<string, unknown> {
  const props = schemaProperties(schema);
  const required = schemaRequired(schema);
  const out: Record<string, unknown> = {};
  for (const [name, raw] of Object.entries(args)) {
    const prop = (props[name] as JsonObject) ?? {};
    const value = coerceArg(prop, raw);
    if ((value === "" || value === undefined) && !required.includes(name)) continue;
    if (typeof value === "number" && value === 0 && !required.includes(name) && typeof prop.minimum === "number" && prop.minimum > 0) {
      continue;
    }
    if (value !== undefined && value !== "") out[name] = value;
  }
  return out;
}

function buildGuidedArgsSchema(
  schema: JsonObject | undefined,
): z.ZodType<Record<string, unknown>> {
  const props = schemaProperties(schema);
  const required = new Set(schemaRequired(schema));
  const shape: Record<string, z.ZodTypeAny> = {};
  for (const [name, rawProp] of Object.entries(props)) {
    const prop = rawProp as JsonObject;
    const type = typeof prop.type === "string" ? prop.type : "string";
    const isRequired = required.has(name);
    let fieldSchema: z.ZodTypeAny;
    if (type === "boolean") {
      fieldSchema = z.boolean();
    } else if (type === "integer" || type === "number") {
      let numSchema = z.string().trim();
      if (isRequired) {
        numSchema = numSchema.min(1, "Required");
      }
      fieldSchema = numSchema
        .optional()
        .refine(
          (v) => {
            if (!v || v.trim() === "") return true;
            const n = Number(v);
            if (Number.isNaN(n)) return false;
            if (typeof prop.minimum === "number" && n < prop.minimum) return false;
            if (typeof prop.maximum === "number" && n > prop.maximum) return false;
            return true;
          },
          (v) => {
            if (v && Number.isNaN(Number(v))) return { message: "Must be a valid number" };
            if (typeof prop.minimum === "number" && Number(v) < prop.minimum) {
              return { message: `Must be >= ${prop.minimum}` };
            }
            if (typeof prop.maximum === "number" && Number(v) > prop.maximum) {
              return { message: `Must be <= ${prop.maximum}` };
            }
            return { message: "Invalid number" };
          },
        );
    } else if (type === "array" || type === "object") {
      fieldSchema = isRequired
        ? z
            .string()
            .trim()
            .min(1, "Required")
            .refine(
              (v) => {
                try {
                  JSON.parse(v);
                  return true;
                } catch {
                  return false;
                }
              },
              "Must be valid JSON",
            )
        : z.string().optional();
    } else {
      fieldSchema = isRequired
        ? z.string().trim().min(1, "Required")
        : z.string().optional();
    }
    shape[name] = fieldSchema;
  }
  return z.object(shape);
}

function samplePlaceholder(name: string, prop: JsonObject): string {
  const examples = Array.isArray(prop.examples) ? prop.examples : [];
  if (typeof examples[0] === "string" && examples[0].trim() !== "") {
    return examples[0];
  }
  if (typeof prop.example === "string" && prop.example.trim() !== "") {
    return prop.example;
  }
  const description =
    typeof prop.description === "string" ? prop.description.trim() : "";
  if (description !== "") return description;
  const lower = name.toLowerCase();
  if (/(repo|library|owner|path|slug|id|url|reference|package)/.test(lower)) {
    return "/owner/repo";
  }
  if (
    /(query|prompt|question|message|ask|describe|explain|search|input)/.test(
      lower,
    )
  ) {
    return "e.g. How to cache data with Next.js?";
  }
  return `Sample ${name}`;
}

function SchemaField({
  name,
  prop,
  value,
  onChange,
  disabled,
  required,
  error,
}: {
  name: string;
  prop: JsonObject;
  value: JsonValue;
  onChange: (value: JsonValue) => void;
  disabled: boolean;
  required: boolean;
  error?: string;
}) {
  const type = typeof prop.type === "string" ? prop.type : "string";
  const description =
    typeof prop.description === "string" ? prop.description : undefined;
  const enums = Array.isArray(prop.enum) ? prop.enum : undefined;
  const placeholder = samplePlaceholder(name, prop);

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between gap-2">
        <Label required={required} className="text-xs font-medium">
          {name}
        </Label>
        <span className="font-mono text-[10px] uppercase text-muted-foreground">
          {type}
        </span>
      </div>

      {type === "boolean" ? (
        <Switch
          checked={value === true}
          onCheckedChange={(checked) => onChange(checked)}
          disabled={disabled}
          aria-label={name}
        />
      ) : enums ? (
        <Select
          value={typeof value === "string" ? value : ""}
          onValueChange={(v) => onChange(v)}
          disabled={disabled}
        >
          <SelectTrigger>
            <SelectValue placeholder={`Select ${name}`} />
          </SelectTrigger>
          <SelectContent>
            {enums.map((opt, idx) => (
              <SelectItem key={idx} value={String(opt)}>
                {String(opt)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      ) : type === "array" || type === "object" ? (
        <Textarea
          value={typeof value === "string" ? value : JSON.stringify(value)}
          onChange={(e) => onChange(e.target.value)}
          placeholder={`${type} as JSON`}
          disabled={disabled}
          className="min-h-[72px] font-mono text-xs"
        />
      ) : (
        <Input
          type={type === "integer" || type === "number" ? "number" : "text"}
          value={
            typeof value === "string" || typeof value === "number"
              ? String(value)
              : ""
          }
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          disabled={disabled}
          className="h-8"
        />
      )}

      {description && (
        <p className="text-[11px] text-muted-foreground">{description}</p>
      )}

      {error && (
        <p className="text-xs font-medium text-destructive">{error}</p>
      )}
    </div>
  );
}

export function MCPTestModal({
  open,
  onOpenChange,
  server,
}: MCPTestModalProps) {
  const testMCPToolMutation = useTestMCPToolMutation();
  const { data: tools = [], isLoading: toolsLoading } = useMCPServerToolsQuery(
    open ? server?.id : null,
  );
  const [testResult, setTestResult] = useState<ApiMCPToolExecutionResult | null>(
    null,
  );
  const [mode, setMode] = useState<InputMode>("guided");
  const [guidedArgs, setGuidedArgs] = useState<JsonObject>({});
  const [guidedErrors, setGuidedErrors] = useState<Record<string, string>>({});

  const form = useForm<TestFormValues>({
    resolver: zodResolver(testSchema),
    defaultValues: { toolName: "", argsJson: "{}" },
  });

  const { reset, watch } = form;
  const formToolName = watch("toolName");

  const selectedTool: ApiMCPTool | undefined = useMemo(
    () => tools.find((t) => t.name === formToolName),
    [tools, formToolName],
  );

  const schema = (selectedTool?.inputSchema as JsonObject | undefined) ?? {};

  useEffect(() => {
    if (!selectedTool || mode !== "guided") return;
    const props = schemaProperties(schema);
    const next: JsonObject = {};
    for (const [name, prop] of Object.entries(props)) {
      next[name] = defaultForProp(prop as JsonObject);
    }
    setGuidedArgs(next);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedTool?.id, mode]);

  const handleToolSelect = (toolName: string) => {
    form.setValue("toolName", toolName, { shouldValidate: true });
    setGuidedErrors({});
    if (mode === "guided") {
      const tool = tools.find((t) => t.name === toolName);
      const props = schemaProperties(
        tool?.inputSchema as JsonObject | undefined,
      );
      const next: JsonObject = {};
      for (const [name, prop] of Object.entries(props)) {
        next[name] = defaultForProp(prop as JsonObject);
      }
      setGuidedArgs(next);
    }
  };

  const onSubmit = async (values: TestFormValues) => {
    if (!server) return;
    setTestResult(null);
    let parsedArgs: Record<string, unknown>;
    if (mode === "guided") {
      const parsed = buildGuidedArgsSchema(schema).safeParse(guidedArgs);
      if (!parsed.success) {
        const fieldErrors: Record<string, string> = {};
        let first = true;
        for (const issue of parsed.error.issues) {
          const key = String(issue.path[0] ?? "");
          if (key && !fieldErrors[key]) {
            fieldErrors[key] = issue.message;
            if (first) {
              toast.error(`Missing/invalid required argument: ${key}`);
              first = false;
            }
          }
        }
        setGuidedErrors(fieldErrors);
        return;
      }
      setGuidedErrors({});
      parsedArgs = buildGuidedArgs(schema, guidedArgs);
    } else {
      try {
        parsedArgs = JSON.parse(values.argsJson) as Record<string, unknown>;
      } catch (_parseError) {
        toast.error("Invalid JSON in args");
        return;
      }
    }
    try {
      const res = await testMCPToolMutation.mutateAsync({
        serverId: server.id,
        toolName: values.toolName.trim(),
        args: parsedArgs,
      });
      setTestResult(res);
      if (res.statusCode === 200) {
        toast.success(`MCP Tool executed successfully (${res.latencyMs}ms)`);
      } else {
        toast.error(`MCP Tool error status: ${res.statusCode}`);
      }
    } catch (err: unknown) {
      toast.error(`Execution error: ${getErrorMessage(err)}`);
    }
  };

  const syncArgsJson = () => {
    form.setValue("argsJson", JSON.stringify(buildGuidedArgs(schema, guidedArgs), null, 2));
  };

  const handleOpenChange = (openState: boolean) => {
    if (openState) {
      reset({ toolName: "", argsJson: "{}" });
      setTestResult(null);
      setMode("guided");
      setGuidedArgs({});
      setGuidedErrors({});
    } else {
      setTestResult(null);
    }
    onOpenChange(openState);
  };

  const props = schemaProperties(schema);
  const required = schemaRequired(schema);
  const hasTools = tools.length > 0;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-lg max-h-[85vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Terminal className="h-4 w-4 text-muted-foreground" />
            <span>Test MCP Server: {server?.displayName || server?.name}</span>
          </DialogTitle>
          <DialogDescription>
            Select a synced tool and its arguments to execute on the remote MCP
            endpoint.
          </DialogDescription>
        </DialogHeader>
        <Form {...form}>
          <form
            onSubmit={form.handleSubmit(onSubmit)}
            className="space-y-4 py-4"
          >
            <FormField
              control={form.control}
              name="toolName"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Tool</FormLabel>
                  <FormControl>
                    {toolsLoading ? (
                      <div className="flex h-9 items-center gap-2 px-3 border border-input rounded-none text-xs text-muted-foreground">
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        <span>Loading synced tools…</span>
                      </div>
                    ) : hasTools ? (
                      <Select
                        value={field.value}
                        onValueChange={(v) => handleToolSelect(v)}
                        disabled={testMCPToolMutation.isPending}
                      >
                        <SelectTrigger>
                          <SelectValue placeholder="Select a tool" />
                        </SelectTrigger>
                        <SelectContent>
                          {tools.map((t) => (
                            <SelectItem key={t.name} value={t.name}>
                              {t.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    ) : (
                      <Input
                        {...field}
                        placeholder="tool name (e.g. search, crawl, tools/list)"
                        disabled={testMCPToolMutation.isPending}
                      />
                    )}
                  </FormControl>
                  {!hasTools && !toolsLoading && (
                    <p className="text-[11px] text-muted-foreground">
                      No tools found for this server. Run{" "}
                      <span className="font-medium">Sync</span> first to load the
                      tool catalog, or type a tool name manually.
                    </p>
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />

            {formToolName.trim() !== "" && (
              <div>
                <div className="flex items-center justify-between gap-2 pb-2">
                  <Label className="text-xs font-medium">Arguments</Label>
                  <div className="flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => setMode("guided")}
                      className={
                        mode === "guided"
                          ? "rounded-none border border-border bg-muted px-2 py-1 text-[11px] font-medium"
                          : "rounded-none px-2 py-1 text-[11px] text-muted-foreground hover:text-foreground"
                      }
                    >
                      Guided
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        if (mode === "guided") syncArgsJson();
                        setMode("raw");
                      }}
                      className={
                        mode === "raw"
                          ? "rounded-none border border-border bg-muted px-2 py-1 text-[11px] font-medium"
                          : "rounded-none px-2 py-1 text-[11px] text-muted-foreground hover:text-foreground"
                      }
                    >
                      Raw JSON
                    </button>
                  </div>
                </div>
                {mode === "guided" && selectedTool ? (
                  Object.keys(props).length > 0 ? (
                    <div className="space-y-3 border border-border p-3 rounded-none">
                      {Object.entries(props).map(([name, prop]) => (
                        <SchemaField
                          key={name}
                          name={name}
                          prop={prop as JsonObject}
                          value={guidedArgs[name]}
                          onChange={(value) => {
                            setGuidedArgs((prev) => ({
                              ...prev,
                              [name]: value,
                            }));
                            if (guidedErrors[name]) {
                              setGuidedErrors((prev) => {
                                const next = { ...prev };
                                delete next[name];
                                return next;
                              });
                            }
                          }}
                          disabled={testMCPToolMutation.isPending}
                          required={required.includes(name)}
                          error={guidedErrors[name]}
                        />
                      ))}
                    </div>
                  ) : (
                    <p className="text-xs text-muted-foreground">
                      This tool declares no input schema, so it will be called with
                      no arguments. Switch to Raw JSON to pass arbitrary
                      arguments.
                    </p>
                  )
                ) : (
                  <FormField
                    control={form.control}
                    name="argsJson"
                    render={({ field }) => (
                      <FormItem className="space-y-2">
                        <FormControl>
                          <Textarea
                            {...field}
                            placeholder="{}"
                            disabled={testMCPToolMutation.isPending}
                            className="min-h-[96px] font-mono text-xs"
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
              </div>
            )}

            <Button
              type="submit"
              variant="prismViolet"
              className="w-full gap-2"
              disabled={testMCPToolMutation.isPending}
            >
              {testMCPToolMutation.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Play className="h-4 w-4" />
              )}
              <span>
                {testMCPToolMutation.isPending
                  ? "Executing MCP Tool…"
                  : "Execute MCP Tool Test"}
              </span>
            </Button>

            {testResult && (
              <div className="space-y-2 pt-2">
                <div className="flex items-center justify-between">
                  <Label className="text-xs font-semibold">
                    Execution Output
                  </Label>
                  <Badge
                    variant={
                      testResult.statusCode === 200 ? "success" : "destructive"
                    }
                    className="font-mono text-[10px]"
                  >
                    Status {testResult.statusCode} ({testResult.latencyMs}ms)
                  </Badge>
                </div>
                <div className="p-3 rounded-none border border-border bg-muted/40 font-mono text-xs overflow-y-auto max-h-48 whitespace-pre-wrap">
                  {typeof testResult.result === "object"
                    ? JSON.stringify(testResult.result, null, 2)
                    : String(testResult.result)}
                </div>
              </div>
            )}
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
