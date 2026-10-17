'use client';

import { useMemo } from 'react';
import dynamic from 'next/dynamic';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { Activity } from 'lucide-react';

export interface TrafficChartData {
  time: string;
  requests: number;
  [key: string]: string | number | undefined;
}

export interface TrafficChartProps {
  data: TrafficChartData[];
  modelKeys?: string[];
  height?: number;
}

const MODEL_PALETTE = [
  '#8B5CF6', // Violet
  '#06B6D4', // Cyan
  '#10B981', // Emerald
  '#F59E0B', // Amber
  '#EC4899', // Pink
  '#3B82F6', // Blue
  '#A855F7', // Purple
  '#14B8A6', // Teal
];

interface CustomTooltipProps {
  active?: boolean;
  payload?: Array<{
    name?: string;
    dataKey?: string;
    value?: number | string;
    color?: string;
    payload?: TrafficChartData;
  }>;
  label?: string;
  modelKeys?: string[];
}

const CustomTooltip = ({ active, payload, label, modelKeys = [] }: CustomTooltipProps) => {
  if (active && payload && payload.length) {
    const rawData = payload[0].payload;
    const totalRequests = rawData?.requests ?? 0;

    return (
      <div className="rounded-md border border-slate-800 bg-slate-950/95 px-3 py-2.5 shadow-2xl backdrop-blur-md min-w-[180px]">
        <div className="flex items-center justify-between border-b border-white/10 pb-1.5 mb-2 text-[11px] font-mono text-slate-400">
          <span className="font-semibold text-slate-200">{label}</span>
          <span className="font-bold text-slate-100">{Number(totalRequests).toLocaleString()} total req</span>
        </div>

        {modelKeys.length > 0 ? (
          <div className="space-y-1.5 text-xs font-mono">
            {modelKeys.map((model, idx) => {
              const reqCount = Number(rawData?.[model]) || 0;
              if (reqCount === 0 && modelKeys.length > 1) return null;
              const color = MODEL_PALETTE[idx % MODEL_PALETTE.length];
              return (
                <div key={model} className="flex items-center justify-between gap-4">
                  <div className="flex items-center gap-1.5 truncate max-w-[140px]">
                    <span className="h-2 w-2 rounded-full inline-block shrink-0" style={{ backgroundColor: color }} />
                    <span className="text-slate-300 truncate">{model}</span>
                  </div>
                  <span className="font-semibold text-foreground">{reqCount.toLocaleString()} req</span>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="flex items-center justify-between gap-4 text-xs font-mono">
            <span className="text-slate-300">Requests:</span>
            <span className="font-bold text-slate-100">{Number(totalRequests).toLocaleString()} req</span>
          </div>
        )}
      </div>
    );
  }
  return null;
};

function TrafficChartInner({ data, modelKeys = [], height = 240 }: TrafficChartProps) {
  const hasData = Array.isArray(data) && data.length > 0 && data.some((d) => d.requests > 0);

  // Auto-detect model keys if not explicitly provided
  const activeModelKeys = useMemo(() => {
    if (modelKeys && modelKeys.length > 0) return modelKeys;
    if (!data || data.length === 0) return [];
    const keys = new Set<string>();
    for (const d of data) {
      for (const k of Object.keys(d)) {
        if (k !== 'time' && k !== 'requests' && typeof d[k] === 'number') {
          keys.add(k);
        }
      }
    }
    return Array.from(keys);
  }, [data, modelKeys]);

  if (!hasData) {
    return (
      <div
        style={{ width: '100%', height }}
        className="flex flex-col items-center justify-center border border-dashed border-border/60 bg-muted/5 p-6 text-center rounded-sm"
      >
        <Activity className="h-7 w-7 text-muted-foreground/40 mb-2.5" />
        <p className="text-xs font-semibold text-foreground">No Request Traffic Recorded</p>
        <p className="text-[11px] text-muted-foreground mt-0.5 max-w-xs leading-relaxed">
          Real-time metrics will appear here once requests are dispatched through your Gateway API Keys.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-3" style={{ width: '100%' }}>
      {/* Model Legend Pills */}
      {activeModelKeys.length > 0 && (
        <div className="flex flex-wrap items-center gap-2 pt-1 px-1">
          {activeModelKeys.map((model, idx) => {
            const color = MODEL_PALETTE[idx % MODEL_PALETTE.length];
            const totalForModel = data.reduce((acc, curr) => acc + (Number(curr[model]) || 0), 0);
            return (
              <div
                key={model}
                className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-secondary/50 border border-border/70 text-xs font-mono"
              >
                <span className="h-2 w-2 rounded-full shrink-0" style={{ backgroundColor: color }} />
                <span className="font-medium text-foreground">{model}</span>
                <span className="text-muted-foreground text-[11px]">({totalForModel.toLocaleString()} req)</span>
              </div>
            );
          })}
        </div>
      )}

      {/* Chart Canvas */}
      <div style={{ width: '100%', height }}>
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data} margin={{ top: 10, right: 10, left: -20, bottom: 0 }}>
            <defs>
              {activeModelKeys.length > 0 ? (
                activeModelKeys.map((model, idx) => {
                  const color = MODEL_PALETTE[idx % MODEL_PALETTE.length];
                  return (
                    <linearGradient key={`grad-${model}`} id={`grad-${model}`} x1="0" y1="0" x2="0" y2="1">
                      <stop offset="5%" stopColor={color} stopOpacity={0.45} />
                      <stop offset="95%" stopColor={color} stopOpacity={0.0} />
                    </linearGradient>
                  );
                })
              ) : (
                <linearGradient id="violetGradient" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#8B5CF6" stopOpacity={0.4} />
                  <stop offset="95%" stopColor="#8B5CF6" stopOpacity={0} />
                </linearGradient>
              )}
            </defs>

            <XAxis
              dataKey="time"
              stroke="currentColor"
              className="text-[10px] text-muted-foreground font-mono"
              tickLine={false}
            />
            <YAxis
              stroke="currentColor"
              className="text-[10px] text-muted-foreground font-mono"
              tickLine={false}
              allowDecimals={false}
            />
            <Tooltip
              content={<CustomTooltip modelKeys={activeModelKeys} />}
              cursor={{ stroke: 'rgba(139, 92, 246, 0.4)', strokeWidth: 1, strokeDasharray: '4 4' }}
            />

            {activeModelKeys.length > 0 ? (
              activeModelKeys.map((model, idx) => {
                const color = MODEL_PALETTE[idx % MODEL_PALETTE.length];
                return (
                  <Area
                    key={model}
                    type="monotone"
                    dataKey={model}
                    name={model}
                    stroke={color}
                    strokeWidth={2}
                    fillOpacity={1}
                    fill={`url(#grad-${model})`}
                    activeDot={{ r: 4, stroke: color, strokeWidth: 2, fill: '#090D16' }}
                  />
                );
              })
            ) : (
              <Area
                type="monotone"
                dataKey="requests"
                stroke="#8B5CF6"
                strokeWidth={2}
                fillOpacity={1}
                fill="url(#violetGradient)"
                activeDot={{ r: 4, stroke: '#8B5CF6', strokeWidth: 2, fill: '#090D16' }}
              />
            )}
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

export const LazyTrafficChart = dynamic(() => Promise.resolve(TrafficChartInner), {
  ssr: false,
  loading: () => <div className="h-[240px] w-full animate-pulse rounded bg-muted/40" />,
});
