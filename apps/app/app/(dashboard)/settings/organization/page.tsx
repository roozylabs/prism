'use client';

import { useState } from 'react';
import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { useOrganizationQuery } from '@/hooks/queries/useOrganizationQuery';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { Settings, Pencil } from 'lucide-react';
import { ApiSetting } from '@/lib/api';
import { SettingFormSheet } from './_components/SettingFormSheet';

export default function SettingsOrganizationPage() {
  const { data, isLoading, isError, refetch } = useOrganizationQuery();
  const settings: ApiSetting[] = data ?? [];

  const [drawerOpen, setDrawerOpen] = useState(false);
  const [editingSetting, setEditingSetting] = useState<ApiSetting | null>(null);

  const openEditDrawer = (s: ApiSetting) => {
    setEditingSetting(s);
    setDrawerOpen(true);
  };

  return (
    <AppLayout>
      <PageHeader
        title="Organization & System Settings"
        description="Configure organization details, default workspace parameters, and notification webhooks."
      />

      {isError && <ErrorState onRetry={refetch} />}

      {!isError && (
        <Card>
          <CardHeader>
            <CardTitle className="text-sm font-semibold flex items-center gap-2">
              <Settings className="h-4 w-4 text-primary" />
              <span>Organization Parameters</span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <p className="text-xs text-muted-foreground">Loading settings...</p>
            ) : settings.length === 0 ? (
              <EmptyState description="No organization settings configured yet." />
            ) : (
              <div className="space-y-2">
                {settings.map((s, i) => (
                  <div
                    key={s.id ?? `${s.key}-${i}`}
                    className="flex items-center justify-between p-3 rounded-md border border-border bg-muted/20"
                  >
                    <div className="flex flex-col gap-0.5 min-w-0 flex-1 mr-3">
                      <span className="text-xs font-semibold text-foreground">{s.key}</span>
                      <span className="text-xs text-muted-foreground font-mono truncate">{s.value}</span>
                    </div>
                    <div className="flex items-center gap-2">
                      {s.scope === 'platform' && <Badge variant="outline">Platform default</Badge>}
                      {s.category && <Badge variant="violet">{s.category}</Badge>}
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7 text-muted-foreground hover:text-foreground"
                        onClick={() => openEditDrawer(s)}
                      >
                        <Pencil className="h-3.5 w-3.5" />
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      <SettingFormSheet
        open={drawerOpen}
        onOpenChange={setDrawerOpen}
        editingSetting={editingSetting}
      />
    </AppLayout>
  );
}
