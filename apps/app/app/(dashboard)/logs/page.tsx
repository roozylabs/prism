'use client';

import { useCallback, useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Badge } from '@/components/atoms/Badge';
import { Button } from '@/components/atoms/Button';
import { DataTable, Column } from '@/components/organisms/DataTable';
import { Sheet, SheetContent, SheetHeader, SheetTitle, SheetDescription } from '@/components/molecules/Sheet';
import { useLogsQuery } from '@/hooks/queries/useLogsQuery';
import { ApiRequestLog } from '@/lib/api';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { Activity, Check, Copy, Eye, RefreshCw, Server } from 'lucide-react';

const PAGE_SIZE = 25;

export default function LogsPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const { data, isLoading, isError, refetch } = useLogsQuery({
    page,
    limit: PAGE_SIZE,
    search: search || undefined,
  });
  const onSearchChange = useCallback((term: string) => {
    setSearch(term);
    setPage(1);
  }, []);
  const [selectedLog, setSelectedLog] = useState<ApiRequestLog | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const logsList = (data?.data ?? []) as ApiRequestLog[];
  const totalLogs = data?.total ?? 0;

  const [copiedError, setCopiedError] = useState(false);

  const handleCopyError = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedError(true);
    setTimeout(() => setCopiedError(false), 2000);
  };

  const openLogInspector = (log: ApiRequestLog) => {
    setSelectedLog(log);
    setCopiedError(false);
    setDrawerOpen(true);
  };

  const columns: Column<ApiRequestLog>[] = [
    {
      title: 'Timestamp',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (t) => <span className="font-mono text-muted-foreground text-xs">{t ? new Date(String(t)).toLocaleString() : '-'}</span>,
    },
    {
      title: 'Client App / IP',
      key: 'client',
      render: (_, record) => (
        <span className="text-xs">
          <span className="font-semibold text-foreground block">{record.clientApp || 'API Client'}</span>
          <span className="font-mono text-muted-foreground text-[10px]">{record.clientIp || '127.0.0.1'}</span>
        </span>
      ),
    },
    {
      title: 'Resolved Model',
      dataIndex: 'model',
      key: 'model',
      render: (mod) => <Badge variant="outline">{mod}</Badge>,
    },
    {
      title: 'Status',
      dataIndex: 'statusCode',
      key: 'statusCode',
      render: (code) => (
        <Badge variant={code === 200 ? 'success' : 'destructive'} className="font-mono text-[10px]">
          {code}
        </Badge>
      ),
    },
    {
      title: 'Latency (TTFT)',
      key: 'latency',
      render: (_, record) => (
        <span className="font-mono text-xs">
          {record.latencyMs} ms {record.ttftMs ? <span className="text-muted-foreground text-[10px]">({record.ttftMs}ms)</span> : null}
        </span>
      ),
    },
    {
      title: 'Tokens & Cost',
      key: 'tokensCost',
      render: (_, record) => (
        <span className="font-mono text-xs">
          {Number(record.totalTokens || 0).toLocaleString()} tok / <span className="text-emerald-500">${record.estimatedCost != null ? Number(record.estimatedCost).toFixed(4) : '0.0000'}</span>
        </span>
      ),
    },
    {
      title: 'Retries',
      dataIndex: 'retryCount',
      key: 'retryCount',
      render: (retries) => (
        <Badge variant={Number(retries || 0) > 0 ? 'warning' : 'outline'} className="font-mono text-[10px]">
          {Number(retries || 0)} failovers
        </Badge>
      ),
    },
    {
      title: 'Inspect',
      key: 'actions',
      render: (_, record) => (
        <Button variant="ghost" size="sm" className="h-7 text-xs gap-1" onClick={() => openLogInspector(record)}>
          <Eye className="h-3.5 w-3.5 text-muted-foreground" /> Details
        </Button>
      ),
    },
  ];

  return (
    <AppLayout>
      <PageHeader
        title="Live Request Logs & HTTP Inspector"
        description="Inspect all incoming proxy requests, TTFT latency breakdown, HTTP status codes, and failover execution chains."
      />

      <Card>
        <CardHeader>
          <CardTitle className="text-sm font-semibold flex items-center gap-2">
            <Activity className="h-4 w-4 text-muted-foreground" />
            <span>Proxy Request Stream</span>
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isError ? (
            <ErrorState
              title="Failed to load request logs"
              description="Could not connect to Prism Request Logs Inspector."
              onRetry={refetch}
            />
          ) : !isLoading && totalLogs === 0 && !search ? (
            <EmptyState
              title="No Request Logs"
              description="No incoming HTTP proxy requests logged yet."
            />
          ) : (
            <DataTable
              dataSource={logsList}
              columns={columns}
              rowKey="id"
              loading={isLoading}
              pageSize={PAGE_SIZE}
              searchPlaceholder="Search logs by path, model, or client app..."
              onRefresh={refetch}
              server={{ total: totalLogs, page, onPageChange: setPage, onSearchChange }}
            />
          )}
        </CardContent>
      </Card>

      <Sheet open={drawerOpen} onOpenChange={setDrawerOpen}>
        <SheetContent className="sm:max-w-lg overflow-y-auto">
          <SheetHeader>
            <SheetTitle className="flex items-center gap-2">
              <Server className="h-4 w-4 text-muted-foreground" />
              <span>Deep Request Trace & Diagnostics</span>
            </SheetTitle>
            <SheetDescription className="font-mono text-xs text-muted-foreground flex items-center justify-between">
              <span>Request ID: {selectedLog?.id}</span>
              {selectedLog?.id && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-5 px-1.5 text-[10px] gap-1 font-mono hover:bg-accent"
                  onClick={() => handleCopyError(selectedLog.id)}
                >
                  <Copy className="h-3 w-3" />
                  <span>Copy ID</span>
                </Button>
              )}
            </SheetDescription>
          </SheetHeader>

          {selectedLog && (
            <div className="space-y-4 py-4 text-xs">
              <div className="grid grid-cols-2 gap-3 p-3 rounded-md border border-border bg-muted/20">
                <div>
                  <span className="text-muted-foreground block text-[10px]">RESOLVED MODEL</span>
                  <span className="font-bold text-foreground font-mono">{selectedLog.model}</span>
                </div>
                <div>
                  <span className="text-muted-foreground block text-[10px]">PROVIDER</span>
                  <span className="font-bold text-foreground font-mono">{selectedLog.providerId || 'Prism Router'}</span>
                </div>
                <div>
                  <span className="text-muted-foreground block text-[10px]">STATUS CODE</span>
                  <Badge variant={selectedLog.statusCode === 200 ? 'success' : 'destructive'} className="font-mono">
                    {selectedLog.statusCode}
                  </Badge>
                </div>
                <div>
                  <span className="text-muted-foreground block text-[10px]">ROUTING POLICY</span>
                  <Badge variant="outline" className="font-mono text-[10px]">
                    {selectedLog.routingStrategy || 'auto-balanced'}
                  </Badge>
                </div>
                <div>
                  <span className="text-muted-foreground block text-[10px]">LATENCY (TTFT)</span>
                  <span className="font-mono font-bold text-foreground">{selectedLog.latencyMs} ms ({selectedLog.ttftMs ?? 0}ms TTFT)</span>
                </div>
                <div>
                  <span className="text-muted-foreground block text-[10px]">TOKENS (IN / OUT)</span>
                  <span className="font-mono font-bold text-foreground">{selectedLog.inputTokens} / {selectedLog.outputTokens} ({selectedLog.totalTokens} total)</span>
                </div>
                <div className="col-span-2">
                  <span className="text-muted-foreground block text-[10px]">ESTIMATED COST</span>
                  <span className="font-mono font-bold text-emerald-500 text-sm">${Number(selectedLog.estimatedCost || 0).toFixed(6)} USD</span>
                </div>
              </div>

              <div className="space-y-2">
                <span className="font-semibold text-foreground block">Execution Timeline</span>
                <div className="p-3 rounded-md border border-border bg-muted/30 space-y-2 font-mono text-[11px]">
                  <div className="flex items-center gap-2">
                    <span className="h-2 w-2 rounded-full bg-emerald-500 shrink-0" />
                    <span className="text-muted-foreground">1. Ingress & Admission:</span>
                    <span className="text-foreground font-semibold">RBAC & Budget OK</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="h-2 w-2 rounded-full bg-blue-500 shrink-0" />
                    <span className="text-muted-foreground">2. Smart Router:</span>
                    <span className="text-foreground font-semibold">{selectedLog.model} ({selectedLog.providerId || 'Upstream'})</span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className={`h-2 w-2 rounded-full ${selectedLog.statusCode === 200 ? 'bg-emerald-500' : 'bg-red-500'} shrink-0`} />
                    <span className="text-muted-foreground">3. Upstream Proxy:</span>
                    <span className={selectedLog.statusCode === 200 ? 'text-emerald-400 font-semibold' : 'text-red-400 font-semibold'}>
                      HTTP {selectedLog.statusCode} ({selectedLog.latencyMs}ms)
                    </span>
                  </div>
                </div>
              </div>

              <div className="space-y-2">
                <span className="font-semibold text-foreground block">Client Metadata</span>
                <div className="p-3 rounded-md border border-border bg-muted/40 space-y-1 font-mono text-[11px]">
                  <p><span className="text-muted-foreground">Client App:</span> {selectedLog.clientApp || 'API Client'}</p>
                  <p><span className="text-muted-foreground">Client IP:</span> {selectedLog.clientIp || '127.0.0.1'}</p>
                  <p><span className="text-muted-foreground">User Agent:</span> {selectedLog.userAgent || '-'}</p>
                  <p><span className="text-muted-foreground">Is Stream:</span> {selectedLog.isStream ? 'Yes (SSE)' : 'No (JSON)'}</p>
                </div>
              </div>

              {(() => {
                const raw = selectedLog.errorMessage;
                const msg = raw && typeof raw === 'object' ? (raw as { String?: string }).String : raw;
                if (!msg) return null;
                return (
                  <div className="space-y-1">
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-foreground block">Error Message</span>
                      <Button
                        variant="ghost"
                        size="sm"
                        className="h-6 px-2 text-[10px] text-foreground hover:text-black hover:bg-foreground gap-1 font-mono"
                        onClick={() => handleCopyError(String(msg))}
                      >
                        {copiedError ? (
                          <>
                            <Check className="h-3 w-3 text-emerald-500" />
                            <span className="text-emerald-500 font-sans">Copied</span>
                          </>
                        ) : (
                          <>
                            <Copy className="h-3 w-3" />
                            <span className="font-sans">Copy</span>
                          </>
                        )}
                      </Button>
                    </div>
                    <div className="p-3 rounded-md border border-destructive/30 bg-destructive/10 font-mono text-destructive text-[11px] whitespace-pre-wrap break-all">
                      {msg}
                    </div>
                  </div>
                );
              })()}

              <div className="space-y-2">
                <span className="font-semibold text-foreground flex items-center gap-1.5">
                  <RefreshCw className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>Failover Retries</span>
                </span>
                <div className="p-3 rounded-md border border-border bg-muted/40 font-mono text-[11px]">
                  {selectedLog.retryCount > 0 ? (
                    <p className="text-amber-500 font-semibold">{selectedLog.retryCount} failover attempt(s) executed automatically by Prism Gateway.</p>
                  ) : (
                    <p className="text-muted-foreground italic">Executed directly on first attempt without failover rerouting.</p>
                  )}
                </div>
              </div>
            </div>
          )}
        </SheetContent>
      </Sheet>
    </AppLayout>
  );
}


