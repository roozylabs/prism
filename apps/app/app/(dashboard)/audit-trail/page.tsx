'use client';

import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/molecules/Tabs';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { DataTable, Column } from '@/components/organisms/DataTable';
import { useAuditLogsQuery } from '@/hooks/queries/useAuditLogsQuery';
import { useAuditTrailsQuery, useVerifyAuditIntegrity } from '@/hooks/queries/useAuditTrailsQuery';
import { ApiAuditLogItem, ApiAIAuditTrail } from '@/lib/api';
import { useExportAuditLogsMutation } from '@/hooks/mutations/useAuditLogMutations';
import { getErrorMessage } from '@/types/ui';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import {
  DropdownMenu,
  DropdownMenuTrigger,
  DropdownMenuContent,
  DropdownMenuItem,
} from '@/components/molecules/DropdownMenu';
import { ScrollText, Download, FileSpreadsheet, FileCode, ShieldCheck } from 'lucide-react';
import { toast } from 'sonner';

export default function AuditTrailPage() {
  const { data: logsData, isLoading: logsLoading, isError: logsError, refetch: refetchLogs } = useAuditLogsQuery();
  const { data: trailsData, isLoading: trailsLoading, isError: trailsError, refetch: refetchTrails } = useAuditTrailsQuery();
  const verifyMutation = useVerifyAuditIntegrity();
  const exportMutation = useExportAuditLogsMutation();

  const auditLogsList: ApiAuditLogItem[] = logsData?.data ?? [];
  const aiAuditTrailsList: ApiAIAuditTrail[] = trailsData?.data ?? [];

  const handleVerify = (id: string) => {
    verifyMutation.mutate(id, {
      onSuccess: (res) => {
        if (res.valid) {
          toast.success(`Signature Verified! SHA-256 hash intact. (Signature: ${res.signatureHash ? res.signatureHash.substring(0, 10) : 'OK'}...)`);
        } else {
          toast.error(`Verification Failed! ${res.message || 'Hash mismatch detected.'}`);
        }
      },
      onError: (err: Error) => toast.error(`Verification error: ${err.message}`),
    });
  };

  const handleExportCsv = async () => {
    try {
      const blob = await exportMutation.mutateAsync({ format: 'csv' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'audit-logs.csv';
      a.click();
      URL.revokeObjectURL(url);
      toast.success('Exported audit logs as CSV');
    } catch (err: unknown) {
      toast.error(`Failed to export audit logs: ${getErrorMessage(err)}`);
    }
  };

  const handleExportJson = async () => {
    try {
      const blob = await exportMutation.mutateAsync({ format: 'json' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = 'audit-logs.json';
      a.click();
      URL.revokeObjectURL(url);
      toast.success('Exported audit logs as JSON');
    } catch (err: unknown) {
      toast.error(`Failed to export audit logs: ${getErrorMessage(err)}`);
    }
  };

  const aiTrailColumns: Column<ApiAIAuditTrail>[] = [
    {
      title: 'Timestamp',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (val) => <span className="font-mono text-muted-foreground">{new Date(String(val)).toLocaleString()}</span>,
    },
    {
      title: 'Request ID',
      dataIndex: 'requestId',
      key: 'requestId',
      render: (val) => <span className="font-mono text-xs text-foreground truncate max-w-[120px] block">{String(val)}</span>,
    },
    {
      title: 'User / Role',
      dataIndex: 'userRole',
      key: 'userRole',
      render: (role, record) => (
        <span className="text-xs">
          <span className="font-semibold text-foreground">{record.agentName || record.userId}</span>
          <span className="text-muted-foreground font-mono block text-[10px]">({role})</span>
        </span>
      ),
    },
    {
      title: 'Model',
      dataIndex: 'modelSlug',
      key: 'modelSlug',
      render: (model) => <Badge variant="outline">{model}</Badge>,
    },
    {
      title: 'Tokens & Cost',
      key: 'tokensCost',
      render: (_, record) => (
        <span className="font-mono text-xs">
          {record.totalTokens.toLocaleString()} tok / <span className="text-emerald-500">${record.totalCostUsd.toFixed(4)}</span>
        </span>
      ),
    },
    {
      title: 'Compliance',
      dataIndex: 'complianceStatus',
      key: 'compliance',
      render: (status) => (
        <Badge variant={status === 'compliant' ? 'success' : 'destructive'} className="text-[10px]">
          {status}
        </Badge>
      ),
    },
    {
      title: 'Integrity Signature',
      key: 'signature',
      render: (_, record) => (
        <Button
          variant="outline"
          size="sm"
          className="h-7 text-xs gap-1 font-mono"
          disabled={verifyMutation.isPending}
          onClick={() => handleVerify(record.id)}
        >
          <ShieldCheck className="h-3.5 w-3.5 text-muted-foreground" />
          Verify Hash
        </Button>
      ),
    },
  ];

  const adminLogColumns: Column<ApiAuditLogItem>[] = [
    {
      title: 'Timestamp',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (val) => <span className="font-mono text-muted-foreground">{String(val)}</span>,
    },
    {
      title: 'Actor',
      dataIndex: 'actorEmail',
      key: 'actorEmail',
      render: (_val, item) => <span className="font-semibold text-foreground">{item.actorEmail || item.actorId}</span>,
    },
    {
      title: 'Action',
      dataIndex: 'action',
      key: 'action',
      render: (act) => <Badge variant="violet" className="font-mono text-[10px]">{String(act)}</Badge>,
    },
    {
      title: 'Target Resource',
      dataIndex: 'resource',
      key: 'resource',
      render: (_val, item) => <span className="text-muted-foreground">{item.resource || item.resourceId}</span>,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (status) => {
        const s = String(status ?? '');
        const variant: 'success' | 'destructive' | 'warning' | 'info' =
          s === 'success' ? 'success' : s === 'denied' ? 'destructive' : s === 'flagged' ? 'warning' : 'info';
        return <Badge variant={variant} className="text-[10px]">{s}</Badge>;
      },
    },
  ];

  return (
    <AppLayout>
      <PageHeader
        title="Audit Trail & Compliance Ledger"
        description="End-to-end cryptographic AI request audit trails with SHA-256 verification and administrative action logs."
        extra={
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="prismViolet" size="sm" className="gap-1.5">
                <Download className="h-4 w-4" /> Export
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={handleExportCsv}>
                <FileSpreadsheet className="h-4 w-4 mr-2" /> Export CSV
              </DropdownMenuItem>
              <DropdownMenuItem onClick={handleExportJson}>
                <FileCode className="h-4 w-4 mr-2" /> Export JSON
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        }
      />

      <Tabs defaultValue="ai-audit" className="space-y-4">
        <TabsList>
          <TabsTrigger value="ai-audit">AI Cryptographic Audit Trails</TabsTrigger>
          <TabsTrigger value="admin-logs">System Admin Audit Logs</TabsTrigger>
        </TabsList>

        <TabsContent value="ai-audit">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <ShieldCheck className="h-4 w-4 text-muted-foreground" />
                <span>End-to-End Signed AI Request Audit Ledger</span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {trailsError ? (
                <ErrorState
                  title="Failed to load AI audit trails"
                  description="Could not connect to Cryptographic Audit Ledger."
                  onRetry={refetchTrails}
                />
              ) : !trailsLoading && aiAuditTrailsList.length === 0 ? (
                <EmptyState
                  title="No AI Audit Trails Recorded"
                  description="No cryptographic AI request audit trails have been recorded yet."
                />
              ) : (
                <DataTable
                  dataSource={aiAuditTrailsList}
                  columns={aiTrailColumns}
                  rowKey="id"
                  loading={trailsLoading}
                  pageSize={10}
                  searchPlaceholder="Search by request ID, model, or user..."
                  onRefresh={refetchTrails}
                />
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="admin-logs">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <ScrollText className="h-4 w-4 text-muted-foreground" />
                <span>System Administration Action Logs</span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {logsError ? (
                <ErrorState
                  title="Failed to load system audit logs"
                  description="Could not connect to System Audit Log database."
                  onRetry={refetchLogs}
                />
              ) : !logsLoading && auditLogsList.length === 0 ? (
                <EmptyState
                  title="No Admin Audit Logs Found"
                  description="There are no system administration action logs recorded yet. Administrative events (credential changes, policy updates, API key management) will appear here in real-time."
                />
              ) : (
                <DataTable
                  dataSource={auditLogsList}
                  columns={adminLogColumns}
                  rowKey="id"
                  loading={logsLoading}
                  pageSize={10}
                  searchPlaceholder="Search by actor, action, or resource..."
                  onRefresh={refetchLogs}
                />
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </AppLayout>
  );
}

