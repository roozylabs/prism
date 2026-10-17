'use client';

import { useCallback, useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge, StatusDot, StatusType } from '@/components/atoms/Badge';
import { DataTable, Column } from '@/components/organisms/DataTable';
import { useCredentialsQuery } from '@/hooks/queries/useCredentialsQuery';
import { ApiCredential } from '@/lib/api';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { Key, Plus, Trash2, Activity, RefreshCw } from 'lucide-react';
import { toast } from 'sonner';
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from '@/components/molecules/Select';
import { useProvidersQuery } from '@/hooks/queries/useProvidersQuery';
import { ConfirmDialog } from '@/components/molecules/ConfirmDialog';
import { ProviderFilterId, getErrorMessage } from '@/types/ui';
import {
  useTestCredentialMutation,
  useResetCooldownMutation,
  useDeleteCredentialMutation,
} from '@/hooks/mutations/useCredentialMutations';
import { CredentialFormDialog } from './_components/CredentialFormDialog';

const PAGE_SIZE = 20;

export default function CredentialsPage() {
  const [selectedProviderId, setSelectedProviderId] = useState<ProviderFilterId>('all');
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const { data, isLoading, isError, refetch } = useCredentialsQuery(selectedProviderId, {
    page,
    limit: PAGE_SIZE,
    search: search || undefined,
  });
  const onSearchChange = useCallback((term: string) => {
    setSearch(term);
    setPage(1);
  }, []);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [testingId, setTestingId] = useState<string | null>(null);

  const { data: providersData } = useProvidersQuery();
  const providers = Array.isArray(providersData) ? providersData : [];

  const testMutation = useTestCredentialMutation();
  const resetCooldownMutation = useResetCooldownMutation();
  const deleteMutation = useDeleteCredentialMutation();

  const handleTestKey = async (providerId: string, credId: string) => {
    setTestingId(credId);
    try {
      const result = await testMutation.mutateAsync({ providerId, credId });
      if (result.success) {
        toast.success(`Key Health Check Passed (${result.latencyMs}ms latency)`);
      } else {
        toast.error(`Key Health Check Failed: ${result.error || result.message || 'Invalid key or quota exceeded'}`);
      }
      refetch();
    } catch (err: unknown) {
      toast.error(`Health check failed: ${getErrorMessage(err)}`);
    } finally {
      setTestingId(null);
    }
  };

  const handleResetCooldown = async (providerId: string, credId: string) => {
    try {
      await resetCooldownMutation.mutateAsync({ providerId, credId });
      toast.success('Credential cooldown reset');
      refetch();
    } catch (err: unknown) {
      toast.error(`Failed to reset cooldown: ${getErrorMessage(err)}`);
    }
  };

  const credentialsList: ApiCredential[] = data?.data ?? [];
  const totalCredentials = data?.total ?? 0;

  const columns: Column<ApiCredential>[] = [
    {
      title: 'Credential Label',
      dataIndex: 'name' as const,
      key: 'name',
      render: (val) => <span className="font-semibold text-foreground">{String(val ?? '')}</span>,
    },
    {
      title: 'Provider',
      dataIndex: 'providerName' as const,
      key: 'provider',
      render: (_val, record) => <Badge variant="outline">{record.providerName || record.providerId}</Badge>,
    },
    {
      title: 'Key Identifier',
      dataIndex: 'keyPrefix' as const,
      key: 'keyPrefix',
      render: (val, record) => (
        <span className="font-mono text-xs text-muted-foreground">
          {record.maskedKey || (val ? `${val}...` : '••••••••')}
        </span>
      ),
    },
    {
      title: 'Health Status',
      dataIndex: 'status' as const,
      key: 'status',
      render: (_val, record) => {
        let dotStatus: StatusType = 'healthy';
        if (record.status === 'degraded' || record.status === 'cooldown') dotStatus = 'degraded';
        if (record.status === 'exhausted' || record.status === 'disabled' || record.status === 'invalid') dotStatus = 'disabled';
        return (
          <div className="flex flex-col gap-0.5">
            <div className="flex items-center gap-2">
              <StatusDot status={dotStatus} className="font-mono" />
            </div>
            {record.quota?.statusText && (
              <span className="text-[10px] font-mono text-amber-500/90 leading-tight">
                {record.quota.statusText}
              </span>
            )}
          </div>
        );
      },
    },
    {
      title: 'Usage',
      dataIndex: 'requestCount' as const,
      key: 'requestCount',
      render: (val) => <span className="font-mono text-xs">{String(val ?? 0)} reqs</span>,
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_val, record) => (
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            className="h-7 px-2 text-[11px] gap-1"
            onClick={() => handleTestKey(record.providerId, record.id)}
            disabled={testingId === record.id}
          >
            {testingId === record.id ? <RefreshCw className="h-3 w-3 animate-spin" /> : <Activity className="h-3 w-3" />}
            <span>Test Health</span>
          </Button>
          {(record.status === 'cooldown' || record.status === 'degraded') && (
            <ConfirmDialog
              title="Reset Cooldown"
              description={`Reset the cooldown period for "${record.name}" and return it to the active routing pool?`}
              confirmText="Reset Cooldown"
              onConfirm={() => handleResetCooldown(record.providerId, record.id)}
              trigger={
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 px-2 text-[11px] text-amber-500 hover:text-amber-600"
                >
                  Reset Cooldown
                </Button>
              }
            />
          )}
          <ConfirmDialog
            title="Delete Credential"
            description={`Are you sure you want to delete "${record.name}"? Active failovers using this key will be updated.`}
            confirmText="Delete"
            variant="destructive"
            onConfirm={() => {
              deleteMutation.mutate(
                { providerId: record.providerId, credId: record.id },
                {
                  onSuccess: () => {
                    toast.success('Credential deleted successfully');
                    refetch();
                  },
                  onError: (err) => {
                    toast.error(`Delete failed: ${getErrorMessage(err)}`);
                  },
                }
              );
            }}
            trigger={
              <Button variant="ghost" size="sm" className="h-7 w-7 p-0 text-muted-foreground hover:text-destructive">
                <Trash2 className="h-3.5 w-3.5" />
              </Button>
            }
          />
        </div>
      ),
    },
  ];

  if (isLoading) {
    return (
      <AppLayout>
        <PageHeader title="Provider Credentials" description="Manage API keys, OAuth credentials, and key health rotation per provider." />
        <Card><CardContent className="py-12 text-center text-sm text-muted-foreground">Loading credentials...</CardContent></Card>
      </AppLayout>
    );
  }

  if (isError) {
    return (
      <AppLayout>
        <PageHeader title="Provider Credentials" description="Manage API keys, OAuth credentials, and key health rotation per provider." />
        <Card><CardContent className="py-8"><ErrorState title="Failed to load credentials" onRetry={() => refetch()} /></CardContent></Card>
      </AppLayout>
    );
  }

  return (
    <AppLayout>
      <PageHeader
        title="Provider Credentials"
        description="Manage API keys, OAuth credentials, and key health rotation per provider."
        extra={
          <Button variant="prismViolet" onClick={() => setDrawerOpen(true)} className="gap-2">
            <Plus className="h-4 w-4" />
            <span>Add Credential</span>
          </Button>
        }
      />

      <div className="space-y-6">
        <div className="flex items-center gap-2">
          <Select
            value={selectedProviderId}
            onValueChange={(val) => {
              setSelectedProviderId(val as ProviderFilterId);
              setPage(1);
            }}
          >
            <SelectTrigger className="w-[200px]">
              <SelectValue placeholder="Filter by Provider" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Providers</SelectItem>
              {providers.map((p) => (
                <SelectItem key={p.id} value={p.id}>{p.name}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-semibold flex items-center gap-2">
              <Key className="h-4 w-4 text-muted-foreground" />
              <span>Configured Provider Credentials</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {totalCredentials > 0 || search ? (
              <DataTable
                dataSource={credentialsList}
                columns={columns}
                rowKey="id"
                pageSize={PAGE_SIZE}
                searchPlaceholder="Search credentials by label..."
                server={{ total: totalCredentials, page, onPageChange: setPage, onSearchChange }}
              />
            ) : (
              <EmptyState
                title="No Credentials Found"
                description="Add API keys for your AI providers to enable gateway routing."
                action={
                  <Button variant="prismViolet" onClick={() => setDrawerOpen(true)}>
                    Add Credential
                  </Button>
                }
              />
            )}
          </CardContent>
        </Card>
      </div>

      <CredentialFormDialog open={drawerOpen} onOpenChange={setDrawerOpen} />
    </AppLayout>
  );
}
