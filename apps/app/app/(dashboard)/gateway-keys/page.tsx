'use client';

import { useCallback, useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { DataTable, type Column } from '@/components/organisms/DataTable';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { useGatewayKeysQuery, useDeleteGatewayKey } from '@/hooks/queries/useGatewayKeysQuery';
import { useProvidersQuery } from '@/hooks/queries/useProvidersQuery';
import type { ApiGatewayKey } from '@/lib/api';
import { ConfirmDialog } from '@/components/molecules/ConfirmDialog';
import { KeyRound, Plus, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { GatewayKeyFormDialog } from './_components/GatewayKeyFormDialog';

const PAGE_SIZE = 20;

export default function GatewayKeysPage() {
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const { data, isLoading, isError, refetch } = useGatewayKeysQuery({
    page,
    limit: PAGE_SIZE,
    search: search || undefined,
  });
  const onSearchChange = useCallback((term: string) => {
    setSearch(term);
    setPage(1);
  }, []);
  const deleteMutation = useDeleteGatewayKey();
  const { data: providers } = useProvidersQuery();

  const [modalOpen, setModalOpen] = useState(false);

  const list = data?.data ?? [];
  const total = data?.total ?? 0;

  const handleDelete = (key: ApiGatewayKey) => {
    deleteMutation.mutate(key.id, {
      onSuccess: () => toast.success('Key deleted'),
      onError: () => toast.error('Failed to delete key'),
    });
  };

  const columns: Column<ApiGatewayKey>[] = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (val) => <span className="font-semibold">{String(val)}</span>,
    },
    {
      title: 'Key Prefix',
      dataIndex: 'keyPrefix',
      key: 'keyPrefix',
      render: (val) => <span className="font-mono text-muted-foreground">{String(val)}</span>,
    },
    {
      title: 'Allowed Models',
      key: 'allowedModels',
      render: (_, record) => (
        <div className="flex gap-1 flex-wrap">
          {record.allowedModels && record.allowedModels.length > 0 ? (
            record.allowedModels.map((m) => <Badge key={m} variant="outline" className="text-[10px] font-mono">{m}</Badge>)
          ) : (
            <Badge variant="violet" className="text-[10px]">All Models (*)</Badge>
          )}
        </div>
      ),
    },
    {
      title: 'Rate Limit',
      dataIndex: 'rateLimit',
      key: 'rateLimit',
      render: (val) => <span className="font-mono">{String(val)} req/min</span>,
    },
    {
      title: 'Requests',
      dataIndex: 'requestCount',
      key: 'requestCount',
      render: (val) => <span className="font-mono">{String(val)}</span>,
    },
    {
      title: 'Status',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (val) => (
        <Badge variant={val ? 'success' : 'outline'}>
          {val ? 'Active' : 'Disabled'}
        </Badge>
      ),
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_, record) => (
        <ConfirmDialog
          title="Delete Gateway Key"
          description={`Delete key "${record.name}"? This action cannot be undone.`}
          confirmText="Delete"
          onConfirm={() => handleDelete(record)}
          trigger={
            <Button
              variant="ghost"
              size="icon"
              aria-label="Delete key"
              className="h-7 w-7 text-muted-foreground hover:text-destructive"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          }
        />
      ),
    },
  ];

  return (
    <AppLayout>
      <PageHeader
        title="Gateway Keys"
        description="Create and manage API keys that authenticate requests through the Prism AI Gateway."
        extra={
          <Button
            variant="prismViolet"
            size="sm"
            className="gap-1.5"
            onClick={() => setModalOpen(true)}
          >
            <Plus className="h-4 w-4" /> Create Key
          </Button>
        }
      />

      {isError ? (
        <ErrorState onRetry={refetch} />
      ) : !isLoading && total === 0 && !search ? (
        <EmptyState
          title="No gateway keys"
          description="Create your first API key to start routing requests through the gateway."
          icon={<KeyRound className="h-6 w-6" />}
          action={
            <Button variant="prismViolet" size="sm" className="gap-1.5" onClick={() => setModalOpen(true)}>
              <Plus className="h-4 w-4" /> Create Key
            </Button>
          }
        />
      ) : (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-semibold flex items-center gap-2">
              <KeyRound className="h-4 w-4 text-[#8B5CF6]" />
              <span>Active Keys</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            <DataTable
              dataSource={list}
              columns={columns}
              rowKey="id"
              loading={isLoading}
              pageSize={PAGE_SIZE}
              searchPlaceholder="Search gateway keys..."
              onRefresh={refetch}
              server={{ total, page, onPageChange: setPage, onSearchChange }}
            />
          </CardContent>
        </Card>
      )}

      <GatewayKeyFormDialog
        open={modalOpen}
        onOpenChange={setModalOpen}
        providers={providers}
      />
    </AppLayout>
  );
}
