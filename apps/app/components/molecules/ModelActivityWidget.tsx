'use client';

import { useState, useEffect } from 'react';
import { Layers, Activity, KeyRound } from 'lucide-react';
import { useLogsQuery } from '@/hooks/queries/useLogsQuery';
import { ApiRequestLog } from '@/lib/api';
import { useSSE } from '@/context/SSEContext';

interface SSEEventData {
  byCredential?: Record<string, number>;
  byModel?: Record<string, number>;
  selectedModel?: string;
  model?: string;
  latencyMs?: number;
  gatewayApiKeyId?: string;
  gatewayKeyId?: string;
  gatewayKeyName?: string;
  credentialId?: string;
  credentialName?: string;
  keyPrefix?: string;
}

interface SSEEventPayload {
  type?: string;
  data?: SSEEventData;
  model?: string;
  gatewayKeyName?: string;
  credentialName?: string;
  keyPrefix?: string;
  latencyMs?: number;
}

export function ModelActivityWidget({ collapsed }: { collapsed?: boolean }) {
  const { lastEvent } = useSSE();
  const { data: logsData } = useLogsQuery({ limit: 1 });

  const latestLog: ApiRequestLog | undefined = logsData?.data?.[0];

  const [activeModel, setActiveModel] = useState<string>('prism-auto');
  const [activeCred, setActiveCred] = useState<string>('Gateway Key');
  const [latencyMs, setLatencyMs] = useState<number | null>(null);
  const [lastActiveTime, setLastActiveTime] = useState<number>(0);
  const [isRecentlyActive, setIsRecentlyActive] = useState<boolean>(false);

  useEffect(() => {
    if (latestLog) {
      const logTime = latestLog.createdAt ? new Date(latestLog.createdAt).getTime() : Date.now();
      setActiveModel(latestLog.model || 'prism-auto');
      setActiveCred(
        latestLog.credentialName ||
          (latestLog.gatewayKeyId ? `gw_sk_${latestLog.gatewayKeyId.slice(0, 6)}` : 'Gateway Key')
      );
      setLatencyMs(latestLog.latencyMs ?? null);
      setLastActiveTime(logTime);
    }
  }, [latestLog]);

  // Consume global SSE stream
  useEffect(() => {
    if (!lastEvent || !lastEvent.payload) return;
    const payload = lastEvent.payload as SSEEventPayload;

    // 1. active_streams_update event (Real-time active credentials & models)
    if (payload.type === 'active_streams_update' && payload.data) {
      const { byCredential, byModel } = payload.data;
      if (byCredential) {
        const credKeys = Object.keys(byCredential);
        if (credKeys.length > 0) {
          setActiveCred(credKeys[0]);
          setLastActiveTime(Date.now());
        }
      }
      if (byModel) {
        const modelKeys = Object.keys(byModel);
        if (modelKeys.length > 0) {
          setActiveModel(modelKeys[0]);
          setLastActiveTime(Date.now());
        }
      }
    }

    // 2. ROUTING_DECISION event (Smart Router selection)
    if (payload.type === 'ROUTING_DECISION' && payload.data) {
      if (payload.data.selectedModel) {
        setActiveModel(payload.data.selectedModel);
        setLastActiveTime(Date.now());
      }
    }

    // 3. request_log_created event (Completed request summary)
    if (payload.type === 'request_log_created' && payload.data) {
      const d = payload.data;
      if (d.model) setActiveModel(d.model);
      const cred = d.gatewayKeyName || d.credentialName || d.keyPrefix;
      if (cred) setActiveCred(cred);
      if (typeof d.latencyMs === 'number') setLatencyMs(d.latencyMs);
      setLastActiveTime(Date.now());
    }

    // 4. Fallback / flat payload structure
    if (payload.model) {
      setActiveModel(payload.model);
      setLastActiveTime(Date.now());
    }
    const credName = payload.gatewayKeyName || payload.credentialName || payload.keyPrefix;
    if (credName) {
      setActiveCred(credName);
      setLastActiveTime(Date.now());
    }
    if (typeof payload.latencyMs === 'number') {
      setLatencyMs(payload.latencyMs);
      setLastActiveTime(Date.now());
    }
  }, [lastEvent]);

  // 15-second recency status ticker
  useEffect(() => {
    const timer = setInterval(() => {
      const now = Date.now();
      setIsRecentlyActive(now - lastActiveTime < 15000);
    }, 1000);

    return () => clearInterval(timer);
  }, [lastActiveTime]);

  if (collapsed) {
    return (
      <div
        className="flex items-center justify-center py-2 text-[#8B5CF6]"
        title={`Active Router (prism-auto) | ${isRecentlyActive ? 'ACTIVE' : 'STANDBY'} | Model: ${isRecentlyActive ? activeModel : 'prism-auto'} | Key: ${activeCred}`}
      >
        <Activity className={`h-4 w-4 ${isRecentlyActive ? 'animate-pulse text-emerald-400' : 'text-muted-foreground'}`} />
      </div>
    );
  }

  return (
    <div className="p-3 rounded-none border border-border bg-card/60 backdrop-blur space-y-2 text-xs">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-1.5 text-muted-foreground font-semibold text-[11px]">
          <Activity className={`h-3.5 w-3.5 ${isRecentlyActive ? 'text-emerald-400 animate-pulse' : 'text-muted-foreground'}`} />
        </div>
        <span
          className={`text-[9px] font-mono px-1.5 py-0.5 rounded font-bold uppercase tracking-wider ${
            isRecentlyActive
              ? 'text-emerald-400 bg-emerald-500/10 border border-emerald-500/20'
              : 'text-muted-foreground bg-muted/40 border border-border'
          }`}
        >
          {isRecentlyActive ? 'ACTIVE' : 'STANDBY'}
        </span>
      </div>

      <div className="space-y-1 font-mono text-xs pt-0.5">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-1.5 font-bold text-foreground">
            <Layers className="h-3.5 w-3.5 text-[#8B5CF6]" />
            <span className="truncate max-w-[110px]">{isRecentlyActive ? activeModel : 'prism-auto'}</span>
          </div>
          <span className="text-muted-foreground text-[10px]">
            {isRecentlyActive && latencyMs != null ? `${latencyMs}ms` : 'Ready'}
          </span>
        </div>

        {isRecentlyActive && activeCred && (
          <div className="flex items-center gap-1 text-[10px] text-muted-foreground pt-0.5 truncate">
            <KeyRound className="h-3 w-3 text-violet-400 shrink-0" />
            <span className="truncate">{activeCred}</span>
          </div>
        )}
      </div>
    </div>
  );
}
