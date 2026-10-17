'use client';

import { useMemo, useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { MetricCard } from '@/components/molecules/MetricCard';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { StatusDot, Badge, StatusType } from '@/components/atoms/Badge';
import { DataTable, Column } from '@/components/organisms/DataTable';
import { LazyTrafficChart, TrafficChartData } from '@/components/organisms/ChartContainer';
import { ErrorState } from '@/components/molecules/StateAlerts';
import {
  useDashboardStatsQuery,
  useUsageChartQuery,
  useDashboardHealthQuery,
} from '@/hooks/queries/useDashboardQuery';
import { useLogsQuery } from '@/hooks/queries/useLogsQuery';
import { ApiRequestLog, ApiProviderHealth } from '@/lib/api';
import {
  Activity,
  Zap,
  DollarSign,
  CheckCircle2,
  Server
} from 'lucide-react';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectItem,
} from '@/components/molecules/Select';

function providerStatusToDot(status: ApiProviderHealth['status']): StatusType {
  if (status === 'down') return 'disabled';
  return status;
}

interface RecentActivity {
  id: string;
  time: string;
  route: string;
  model: string;
  latency: number;
  cost: number;
  status: 'success' | 'error';
}

const DATE_RANGE_OPTIONS = [
  { value: '24h', label: 'Last 24 Hours' },
  { value: '7d', label: 'Last 7 Days' },
  { value: '30d', label: 'Last 30 Days (Default)' },
  { value: '90d', label: 'Last 90 Days' },
  { value: 'all', label: 'All Time' },
];

export default function DashboardPage() {
  const [dateRange, setDateRange] = useState<string>('30d');
  const { data: stats, isLoading, isError: isStatsError, refetch: refetchStats } = useDashboardStatsQuery(dateRange);
  const { data: usageData, refetch: refetchUsage } = useUsageChartQuery(dateRange);
  const { data: logsData } = useLogsQuery({ limit: 10 });
  const { data: healthData } = useDashboardHealthQuery();

  const rangeLabelMap: Record<string, string> = {
    '24h': '24h',
    '7d': '7d',
    '30d': '30d',
    '90d': '90d',
    'all': 'All Time',
  };
  const currentRangeLabel = rangeLabelMap[dateRange] || '30d';

  const { trafficData, modelKeys } = useMemo(() => {
    if (!usageData || usageData.length === 0) return { trafficData: [] as TrafficChartData[], modelKeys: [] as string[] };

    const dateMap = new Map<string, TrafficChartData>();
    const modelsSet = new Set<string>();

    for (const p of usageData) {
      const model = p.model || 'default';
      modelsSet.add(model);

      const existing = dateMap.get(p.date) || { time: p.date, requests: 0 };
      existing.requests += p.requests;
      existing[model] = (Number(existing[model]) || 0) + p.requests;
      dateMap.set(p.date, existing);
    }

    return {
      trafficData: Array.from(dateMap.values()),
      modelKeys: Array.from(modelsSet),
    };
  }, [usageData]);

  const activities: RecentActivity[] = useMemo(() => {
    if (!logsData?.data) return [];
    return logsData.data.slice(0, 10).map((log: ApiRequestLog) => {
      const d = new Date(log.createdAt);
      const formattedTime = d.toLocaleString('en-US', {
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: false,
      });
      return {
        id: log.id,
        time: formattedTime,
        route: log.clientApp || log.model,
        model: log.model,
        latency: log.latencyMs,
        cost: log.estimatedCost ?? 0,
        status: (log.statusCode < 400 ? 'success' : 'error') as 'success' | 'error',
      };
    });
  }, [logsData]);

  const activityColumns: Column<RecentActivity>[] = [
    {
      title: 'Time & Date',
      dataIndex: 'time',
      key: 'time',
      render: (time) => <span className="font-mono text-muted-foreground whitespace-nowrap text-xs">{String(time ?? '')}</span>,
    },
    {
      title: 'Route Policy / App',
      dataIndex: 'route',
      key: 'route',
      render: (route) => (
        <span className="font-mono text-muted-foreground font-semibold text-xs truncate max-w-[140px] block">{String(route ?? '')}</span>
      ),
    },
    {
      title: 'Resolved Model',
      dataIndex: 'model',
      key: 'model',
      render: (model) => <Badge variant="outline" className="font-mono text-[11px]">{String(model ?? '')}</Badge>,
    },
    {
      title: 'Latency',
      dataIndex: 'latency',
      key: 'latency',
      render: (lat) => <span className="font-mono text-xs whitespace-nowrap">{String(lat ?? 0)} ms</span>,
    },
    {
      title: 'Cost',
      dataIndex: 'cost',
      key: 'cost',
      render: (cost) => <span className="font-mono text-emerald-400 text-xs whitespace-nowrap">${Number(cost || 0).toFixed(4)}</span>,
    },
    {
      title: 'Status',
      dataIndex: 'status',
      key: 'status',
      render: (status) => (
        <Badge variant={status === 'success' ? 'success' : 'destructive'} className="text-[10px] uppercase font-bold">
          {String(status ?? '').toUpperCase()}
        </Badge>
      ),
    },
  ];

  const tokenSubtitle = stats ? (
    stats.inputTokens != null && stats.outputTokens != null && (stats.inputTokens > 0 || stats.outputTokens > 0)
      ? `${(stats.inputTokens / 1_000_000).toFixed(1)}M Input / ${(stats.outputTokens / 1_000_000).toFixed(1)}M Output`
      : `${(stats.totalTokens / 1_000_000).toFixed(1)}M Total Tokens`
  ) : undefined;

  return (
    <AppLayout>
      {/* Top Header Controls: Title & Global Time Filter */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 mb-6">
        <div>
          <h1 className="text-xl font-bold tracking-tight text-foreground">API Gateway Overview</h1>
          <p className="text-xs text-muted-foreground mt-0.5">Real-time metrics, provider health, and operational statistics</p>
        </div>
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <Select value={dateRange} onValueChange={setDateRange}>
              <SelectTrigger className="w-[180px] h-8 text-xs bg-card border-border">
                <SelectValue placeholder="Select period" />
              </SelectTrigger>
              <SelectContent>
                {DATE_RANGE_OPTIONS.map((opt) => (
                  <SelectItem key={opt.value} value={opt.value} className="text-xs">
                    {opt.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </div>
      </div>

      {isStatsError && !stats && (
        <div className="mb-6">
          <ErrorState
            title="Failed to load dashboard metrics"
            description="Could not fetch real-time gateway statistics. Please check your network connection or API service status."
            onRetry={() => {
              refetchStats();
              refetchUsage();
            }}
          />
        </div>
      )}

      {/* Summary KPI Cards Grid */}
      <div className="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 mb-6">
        <MetricCard
          title={`Total Requests (${currentRangeLabel})`}
          value={stats?.totalRequests ? stats.totalRequests.toLocaleString() : (stats?.totalRequests === 0 ? '0' : '-')}
          subtitle={`selected ${currentRangeLabel} period`}
          icon={<Activity className="h-4 w-4 text-muted-foreground" />}
          loading={isLoading}
        />
        <MetricCard
          title="Tokens Processed"
          value={stats?.totalTokens ? `${(stats.totalTokens / 1000000).toFixed(1)}M` : (stats?.totalTokens === 0 ? '0 Tokens' : '-')}
          subtitle={tokenSubtitle}
          icon={<Zap className="h-4 w-4 text-cyan-500" />}
          loading={isLoading}
        />
        <MetricCard
          title="Total Expenditure"
          value={stats?.totalEstimatedCost != null ? `$${stats.totalEstimatedCost.toFixed(2)}` : '-'}
          subtitle="via Smart Routing"
          icon={<DollarSign className="h-4 w-4 text-emerald-500" />}
          loading={isLoading}
        />
        <MetricCard
          title="Gateway Success Rate"
          value={stats && stats.totalRequests > 0 ? `${(100 - stats.errorRate).toFixed(2)}%` : '-'}
          subtitle={stats && stats.totalRequests > 0 ? `${stats.errorRate.toFixed(2)}% failover auto-rerouted` : 'No requests recorded yet'}
          icon={<CheckCircle2 className="h-4 w-4 text-emerald-500" />}
          loading={isLoading}
        />
      </div>

      {/* Main Grid: Traffic Chart + Gateway Provider Health (Responsive: 1 col on tablet/mobile, 3 cols on laptop/desktop) */}
      <div className="grid gap-6 grid-cols-1 lg:grid-cols-3 mb-6">
        <Card className="lg:col-span-2">
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-semibold">Real-Time Request Traffic</CardTitle>
              <span className="font-mono text-xs text-muted-foreground">
                {dateRange === '24h' ? 'Requests / Hour' : 'Requests / Day'}
              </span>
            </div>
          </CardHeader>
          <CardContent>
            <LazyTrafficChart data={trafficData} modelKeys={modelKeys} height={260} />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <Server className="h-4 w-4 text-muted-foreground" />
                <span>Provider Availability</span>
              </CardTitle>
            </div>
          </CardHeader>
          <CardContent className="space-y-3">
            {healthData && healthData.length > 0 ? (
              healthData.map((provider: ApiProviderHealth) => (
                <div key={provider.name} className="flex flex-col sm:flex-row sm:items-center justify-between gap-1 p-2.5 rounded-none border border-border bg-muted/20">
                  <div className="flex items-center gap-2 truncate">
                    <StatusDot status={providerStatusToDot(provider.status)} />
                    <span className="font-semibold text-xs truncate">{provider.name}</span>
                  </div>
                  <span className={`font-mono text-[11px] shrink-0 ${provider.status === 'degraded' ? 'text-amber-500' : 'text-muted-foreground'}`}>
                    {provider.type} ({provider.credCount} cred{provider.credCount !== 1 ? 's' : ''})
                  </span>
                </div>
              ))
            ) : (
              <div className="text-sm text-muted-foreground text-center py-4">No provider data available</div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Recent AI Activity Table (Responsive overflow-x-auto) */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <CardTitle className="text-sm font-semibold flex items-center gap-2">
              <Activity className="h-4 w-4 text-muted-foreground" />
              <span>Recent AI Gateway Activity Logs</span>
            </CardTitle>
          </div>
        </CardHeader>
        <CardContent className="overflow-x-auto">
          <DataTable
            dataSource={activities}
            columns={activityColumns}
            rowKey="id"
            pageSize={10}
            searchPlaceholder="Filter activity logs..."
          />
        </CardContent>
      </Card>
    </AppLayout>
  );
}
