'use client';

import { useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/molecules/Tabs';
import { Progress } from '@/components/atoms/Progress';
import { Badge } from '@/components/atoms/Badge';
import { Button } from '@/components/atoms/Button';
import { DataTable, Column } from '@/components/organisms/DataTable';
import { useBudgetsQuery, useBudgetStatusQuery, useDeleteBudget } from '@/hooks/queries/useBudgetsQuery';
import { useQuotasQuery } from '@/hooks/queries/useQuotasQuery';
import { ApiBudget, ApiTenantQuota, ApiBudgetStatus } from '@/lib/api';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { Wallet, Plus, Trash2, Pencil } from 'lucide-react';
import { toast } from 'sonner';
import { ConfirmDialog } from '@/components/molecules/ConfirmDialog';
import { getErrorMessage } from '@/types/ui';
import { BudgetFormSheet } from './_components/BudgetFormSheet';
import { QuotaFormSheet } from './_components/QuotaFormSheet';

function toMoney(val: unknown): string {
  const n = Number(val ?? 0);
  return `$${(Number.isFinite(n) ? n : 0).toFixed(2)}`;
}

export default function BudgetsPage() {
  const { data: budgets, isLoading: budgetsLoading, isError: budgetsError, refetch: refetchBudgets } = useBudgetsQuery();
  const { data: budgetStatus, isLoading: statusLoading, isError: statusError, refetch: refetchStatus } = useBudgetStatusQuery();
  const { data: quotasData, isLoading: quotasLoading, isError: quotasError, refetch: refetchQuotas } = useQuotasQuery();
  const deleteMutation = useDeleteBudget();

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [quotaSheetOpen, setQuotaSheetOpen] = useState(false);
  const [editingQuota, setEditingQuota] = useState<ApiTenantQuota | null>(null);

  const quotasList: ApiTenantQuota[] = quotasData ?? [];

  const openQuotaEdit = (q: ApiTenantQuota) => {
    setEditingQuota(q);
    setQuotaSheetOpen(true);
  };

  const quotaColumns: Column<ApiTenantQuota>[] = [
    {
      title: 'Target Type',
      dataIndex: 'targetType',
      key: 'targetType',
      render: (type) => <Badge variant="violet">{String(type).toUpperCase()}</Badge>,
    },
    {
      title: 'Target ID',
      dataIndex: 'targetId',
      key: 'targetId',
      render: (id) => <span className="font-mono text-xs text-foreground font-semibold">{String(id)}</span>,
    },
    {
      title: 'Monthly Limit ($)',
      dataIndex: 'monthlySpendLimitUsd',
      key: 'monthlyLimit',
      render: (val) => <span className="font-mono text-xs font-bold text-emerald-500">{toMoney(val)}</span>,
    },
    {
      title: 'Daily Limit ($)',
      dataIndex: 'dailySpendLimitUsd',
      key: 'dailyLimit',
      render: (val) => <span className="font-mono text-xs">{toMoney(val)}</span>,
    },
    {
      title: 'Daily Req Limit',
      dataIndex: 'dailyRequestLimit',
      key: 'dailyReq',
      render: (val) => <span className="font-mono text-xs">{Number(val ?? 0).toLocaleString()} reqs</span>,
    },
    {
      title: 'Max Streams',
      dataIndex: 'maxConcurrentStreams',
      key: 'maxStreams',
      render: (val) => <span className="font-mono text-xs">{String(val ?? 0)}</span>,
    },
    {
      title: 'Actions',
      key: 'actions',
      render: (_, record) => (
        <Button variant="ghost" size="sm" className="h-7 text-xs gap-1" onClick={() => openQuotaEdit(record)}>
          <Pencil className="h-3 w-3" /> Edit Quota
        </Button>
      ),
    },
  ];

  const firstStatus: ApiBudgetStatus | undefined = Array.isArray(budgetStatus)
    ? budgetStatus[0]
    : (budgetStatus as unknown as ApiBudgetStatus | undefined);

  const monthlySpent = firstStatus?.monthlySpent ?? firstStatus?.currentSpendUSD ?? 0;
  const monthlyLimit = firstStatus?.budget?.monthlyLimit ?? firstStatus?.amountUSD ?? 0;
  const usagePercent = firstStatus?.usagePercent ?? firstStatus?.spendPercentage ?? 0;
  const statusLabel = firstStatus?.status ?? 'normal';

  const handleDelete = (budget: ApiBudget) => {
    deleteMutation.mutate(budget.id, {
      onSuccess: () => toast.success(`Budget "${budget.name}" deleted`),
      onError: (err) => toast.error(`Failed to delete budget: ${getErrorMessage(err)}`),
    });
  };

  const budgetColumns: Column<ApiBudget>[] = [
    {
      title: 'Name',
      dataIndex: 'name',
      key: 'name',
      render: (name) => <span className="font-semibold text-foreground">{name}</span>,
    },
    {
      title: 'Monthly Limit',
      dataIndex: 'monthlyLimit',
      key: 'monthlyLimit',
      render: (val) => <span className="font-mono text-sm">{toMoney(val)}</span>,
    },
    {
      title: 'Daily Limit',
      dataIndex: 'dailyLimit',
      key: 'dailyLimit',
      render: (val) => {
        const n = Number(val ?? 0);
        return (
          <span className="font-mono text-sm">{Number.isFinite(n) && n > 0 ? `$${n.toFixed(2)}` : '-'}</span>
        );
      },
    },
    {
      title: 'Hard Limit',
      dataIndex: 'hardLimit',
      key: 'hardLimit',
      render: (val) => (
        <Badge variant={val ? 'destructive' : 'outline'}>
          {val ? 'Enforced' : 'Advisory'}
        </Badge>
      ),
    },
    {
      title: 'Thresholds',
      key: 'thresholds',
      render: (_, record) => (
        <span className="text-xs text-muted-foreground">
          {record.warningThreshold ?? record.alertThresholdPct ?? 80}% / {record.criticalThreshold ?? 100}%
        </span>
      ),
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
      title: '',
      key: 'actions',
      render: (_, record) => (
        <ConfirmDialog
          title="Delete Budget"
          description={`Delete budget "${record.name}"? This cannot be undone.`}
          confirmText="Delete"
          onConfirm={() => handleDelete(record)}
          trigger={
            <Button
              variant="destructive"
              size="sm"
              className="gap-1.5 text-xs"
              disabled={deleteMutation.isPending}
            >
              <Trash2 className="h-3.5 w-3.5" /> Delete
            </Button>
          }
        />
      ),
    },
  ];

  return (
    <AppLayout>
      <PageHeader
        title="Budgets & Multi-Tenant Quotas"
        description="Set spend caps, token rate limits, and threshold notifications across Organizations, Workspaces, Agents, and Users."
        extra={
          <Button variant="prismViolet" size="sm" className="gap-1.5" onClick={() => setDrawerOpen(true)}>
            <Plus className="h-4 w-4" /> Create Budget Limit
          </Button>
        }
      />

      <Tabs defaultValue="budgets" className="space-y-4">
        <TabsList>
          <TabsTrigger value="budgets">Global Budgets</TabsTrigger>
          <TabsTrigger value="quotas">Multi-Tenant Quotas</TabsTrigger>
        </TabsList>

        <TabsContent value="budgets" className="space-y-4">
          {budgetsError || statusError ? (
            <ErrorState
              title="Failed to load budget data"
              description="Could not connect to Prism Metering & Billing engine."
              onRetry={() => { refetchBudgets(); refetchStatus(); }}
            />
          ) : (
            <>
              <div className="grid gap-6 md:grid-cols-2">
                <Card>
                  <CardHeader>
                    <div className="flex items-center justify-between">
                      <CardTitle className="text-sm font-semibold flex items-center gap-2">
                        <Wallet className="h-4 w-4 text-[#8B5CF6]" />
                        <span>Monthly Organization Budget</span>
                      </CardTitle>
                      <Badge variant={usagePercent > 80 ? 'destructive' : 'success'}>
                        {100 - Math.round(usagePercent)}% Available
                      </Badge>
                    </div>
                  </CardHeader>
                  <CardContent className="space-y-4">
                    <div className="flex items-baseline justify-between">
                      <span className="font-mono text-2xl font-bold text-foreground">
                        {toMoney(monthlySpent)}
                      </span>
                      <span className="font-mono text-xs text-muted-foreground">
                        Limit: {toMoney(monthlyLimit)} / month
                      </span>
                    </div>
                    <Progress value={statusLoading ? 0 : Math.min(100, Math.round(usagePercent))} />
                    <p className="text-xs text-muted-foreground">
                      {statusLabel === 'exceeded'
                        ? 'Budget limit exceeded.'
                        : statusLabel === 'critical'
                          ? 'Approaching critical threshold.'
                          : statusLabel === 'warning'
                            ? 'Approaching warning threshold.'
                            : 'Spending is within normal range.'}
                    </p>
                  </CardContent>
                </Card>
              </div>

              <Card>
                <CardHeader>
                  <CardTitle className="text-sm font-semibold">Budget Limits</CardTitle>
                </CardHeader>
                <CardContent>
                  {!budgetsLoading && (!budgets || budgets.length === 0) ? (
                    <EmptyState
                      title="No Budgets Configured"
                      description="Create a budget limit to start tracking spending."
                      action={
                        <Button variant="prismViolet" size="sm" className="gap-1.5" onClick={() => setDrawerOpen(true)}>
                          <Plus className="h-4 w-4" /> Create Budget Limit
                        </Button>
                      }
                    />
                  ) : (
                    <DataTable
                      dataSource={budgets ?? []}
                      columns={budgetColumns}
                      rowKey="id"
                      loading={budgetsLoading}
                      pageSize={10}
                      searchPlaceholder="Search budgets..."
                    />
                  )}
                </CardContent>
              </Card>
            </>
          )}
        </TabsContent>

        <TabsContent value="quotas" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <Wallet className="h-4 w-4 text-[#8B5CF6]" />
                <span>Multi-Tenant Workspace & Agent Quotas</span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {quotasError ? (
                <ErrorState
                  title="Failed to load quotas"
                  description="Could not connect to Prism Multi-Tenant Quota engine."
                  onRetry={refetchQuotas}
                />
              ) : !quotasLoading && quotasList.length === 0 ? (
                <EmptyState
                  title="No Quotas Configured"
                  description="There are no specific multi-tenant quotas set yet."
                />
              ) : (
                <DataTable
                  dataSource={quotasList}
                  columns={quotaColumns}
                  rowKey="id"
                  loading={quotasLoading}
                  pageSize={10}
                  searchPlaceholder="Search quotas by target..."
                />
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>

      <BudgetFormSheet open={drawerOpen} onOpenChange={setDrawerOpen} />
      <QuotaFormSheet
        open={quotaSheetOpen}
        onOpenChange={setQuotaSheetOpen}
        editingQuota={editingQuota}
      />
    </AppLayout>
  );
}
