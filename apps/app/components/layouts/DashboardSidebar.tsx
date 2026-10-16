'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import {
  LayoutDashboard,
  Box,
  Server,
  Key,
  Layers,
  Workflow,
  KeyRound,
  Wrench,
  Database,
  Globe,
  Bot,
  Users,
  ScrollText,
  Activity,
  Wallet,
  Play,
  Settings,
  CreditCard,
  ChevronLeft,
  ChevronRight,
  Shield,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { DEFAULT_FEATURE_FLAGS, type FeatureFlagKey } from '@/lib/features/features';
import { useFeatureFlagsQuery } from '@/hooks/queries/useFeatureFlags';
import { Button } from '@/components/atoms/Button';
import { ModelActivityWidget } from '@/components/molecules/ModelActivityWidget';
import { useSidebarStore } from '@/stores/useSidebarStore';
import { AppRoutes } from '@/constants/routes';

interface NavItem {
  key: string;
  label: string;
  href: AppRoutes;
  icon: ReactNode;
  /** Shown only while this feature flag is on. */
  flag?: FeatureFlagKey;
}

interface NavGroup {
  title: string;
  items: NavItem[];
}

const navGroups: NavGroup[] = [
  {
    title: 'OVERVIEW',
    items: [
      { key: AppRoutes.HOME, label: 'Dashboard', href: AppRoutes.HOME, icon: <LayoutDashboard className="h-4 w-4" /> },
      { key: AppRoutes.SANDBOX, label: 'AI Sandbox', href: AppRoutes.SANDBOX, icon: <Box className="h-4 w-4" /> },
    ],
  },
  {
    title: 'OBSERVE',
    items: [
      { key: AppRoutes.LOGS, label: 'Request Logs', href: AppRoutes.LOGS, icon: <Activity className="h-4 w-4" /> },
      { key: AppRoutes.AUDIT_TRAIL, label: 'Audit Trail', href: AppRoutes.AUDIT_TRAIL, icon: <ScrollText className="h-4 w-4" /> },
    ],
  },
  {
    title: 'AI INFRASTRUCTURE',
    items: [
      { key: AppRoutes.PROVIDERS, label: 'Providers', href: AppRoutes.PROVIDERS, icon: <Server className="h-4 w-4" /> },
      { key: AppRoutes.CREDENTIALS, label: 'Credentials', href: AppRoutes.CREDENTIALS, icon: <Key className="h-4 w-4" /> },
      { key: AppRoutes.MODELS, label: 'Models', href: AppRoutes.MODELS, icon: <Layers className="h-4 w-4" /> },
      { key: AppRoutes.POLICIES, label: 'Routing Policies', href: AppRoutes.POLICIES, icon: <Workflow className="h-4 w-4" /> },
      { key: AppRoutes.GATEWAY_KEYS, label: 'Gateway Keys', href: AppRoutes.GATEWAY_KEYS, icon: <KeyRound className="h-4 w-4" /> },
    ],
  },
  {
    title: 'AI RUNTIME',
    items: [
      { key: AppRoutes.AGENTS, label: 'Agent Gateway', href: AppRoutes.AGENTS, icon: <Bot className="h-4 w-4" /> },
      { key: AppRoutes.TOOLS, label: 'Tool Gateway', href: AppRoutes.TOOLS, icon: <Wrench className="h-4 w-4" /> },
      { key: AppRoutes.RESOURCES, label: 'Resource Gateway', href: AppRoutes.RESOURCES, icon: <Database className="h-4 w-4" /> },
      { key: AppRoutes.MCP, label: 'MCP Gateway', href: AppRoutes.MCP, icon: <Globe className="h-4 w-4" /> },
      { key: AppRoutes.PLAYGROUND, label: 'Routing Simulator', href: AppRoutes.PLAYGROUND, icon: <Play className="h-4 w-4" /> },
    ],
  },
  {
    title: 'CONTROL',
    items: [
      { key: AppRoutes.GOVERNANCE, label: 'Governance & RBAC', href: AppRoutes.GOVERNANCE, icon: <Users className="h-4 w-4" /> },
      { key: AppRoutes.BUDGETS, label: 'Budgets & Quotas', href: AppRoutes.BUDGETS, icon: <Wallet className="h-4 w-4" /> },
    ],
  },
  {
    title: 'ORGANIZATION',
    items: [
      { key: AppRoutes.SETTINGS_MEMBERS, label: 'Team Members', href: AppRoutes.SETTINGS_MEMBERS, icon: <Users className="h-4 w-4" /> },
      { key: AppRoutes.BILLING, label: 'Billing & Plans', href: AppRoutes.BILLING, icon: <CreditCard className="h-4 w-4" />, flag: 'billing' },
      { key: AppRoutes.SETTINGS_ORGANIZATION, label: 'Settings', href: AppRoutes.SETTINGS_ORGANIZATION, icon: <Settings className="h-4 w-4" /> },
    ],
  },
];

export function DashboardSidebar() {
  const pathname = usePathname();
  const { collapsed, toggleCollapsed } = useSidebarStore();
  const { data: features } = useFeatureFlagsQuery();
  const isOn = (flag: FeatureFlagKey) => features?.flags?.[flag] ?? DEFAULT_FEATURE_FLAGS[flag] ?? false;

  return (
    <aside
      className={cn(
        'fixed bottom-0 top-0 z-40 flex flex-col border-r border-border bg-card transition-all duration-200',
        collapsed ? 'w-[72px]' : 'w-[248px]'
      )}
    >
      {/* Sidebar Header Brand */}
      <div className="flex h-14 items-center justify-between border-b border-border px-4">
        <Link href={AppRoutes.HOME} className="flex items-center gap-2.5 overflow-hidden">
          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-none bg-[#8B5CF6] text-white shadow-sm">
            <Shield className="h-4 w-4" />
          </div>
          {!collapsed && (
            <span className="font-mono text-base font-bold tracking-tight text-foreground truncate">
              PRISM <span className="text-xs font-normal text-muted-foreground">v0.4.0</span>
            </span>
          )}
        </Link>

        <Button
          variant="ghost"
          size="icon"
          onClick={toggleCollapsed}
          className="h-6 w-6 rounded-none text-muted-foreground hover:text-foreground cursor-pointer"
          aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
        </Button>
      </div>

      {/* Navigation Groups */}
      <div className="flex-1 overflow-y-auto px-2 py-3 space-y-4">
        {navGroups.map((group) => (
          <div key={group.title} className="space-y-1">
            {!collapsed && (
              <h4 className="px-3 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground/70">
                {group.title}
              </h4>
            )}
            {group.items.filter((item) => !item.flag || isOn(item.flag)).map((item) => {
              const isActive = pathname === item.href || (item.href !== AppRoutes.HOME && pathname.startsWith(item.href));
              return (
                <Link
                  key={item.key}
                  href={item.href}
                  className={cn(
                    'group relative flex items-center gap-3 rounded-none px-3 py-2 text-xs font-medium transition-colors',
                    isActive
                      ? 'bg-[#8B5CF6]/10 text-[#7C3AED] font-semibold'
                      : 'text-muted-foreground hover:bg-accent hover:text-foreground'
                  )}
                >
                  {/* Left Active Line Indicator */}
                  {isActive && (
                    <span className="absolute left-0 top-1/2 h-4 w-1 -translate-y-1/2 rounded-none bg-[#8B5CF6]" />
                  )}
                  <span className={cn('shrink-0', isActive ? 'text-[#7C3AED]' : 'text-muted-foreground group-hover:text-foreground')}>
                    {item.icon}
                  </span>
                  {!collapsed && <span className="truncate">{item.label}</span>}
                </Link>
              );
            })}
          </div>
        ))}
      </div>

      {/* Active Model Activity Widget */}
      <div className="px-3 py-2 border-t border-border/60">
        <ModelActivityWidget collapsed={collapsed} />
      </div>
    </aside>
  );
}
