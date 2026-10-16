'use client';

import { AppLayout } from '@/components/AppLayout';
import { PageHeader } from '@/components/molecules/PageHeader';
import { Card, CardHeader, CardTitle, CardContent, CardDescription, CardFooter } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { useBillingPlansQuery, useBillingSubscriptionQuery, useBillingInvoicesQuery, useUpgradeSubscription } from '@/hooks/queries/useBillingQuery';
import { ApiBillingPlan, ApiBillingInvoice } from '@/lib/api';
import { ErrorState, EmptyState } from '@/components/molecules/StateAlerts';
import { CreditCard, Check } from 'lucide-react';
import { toast } from 'sonner';

import { ConfirmDialog } from '@/components/molecules/ConfirmDialog';
import { FeatureGate } from '@/components/molecules/FeatureGate';

// Billing is a placeholder (nothing is stored or charged) and its API routes
// exist only with FEATURE_FLAG_BILLING, so the page stays behind the flag.
export default function BillingPage() {
  return (
    <FeatureGate
      flag="billing"
      fallback={
        <AppLayout>
          <PageHeader title="Billing & Subscription Plans" />
          <EmptyState
            title="Billing is not available yet"
            description="Plans and invoices will appear here once billing is enabled for this deployment."
          />
        </AppLayout>
      }
    >
      <BillingContent />
    </FeatureGate>
  );
}

function BillingContent() {
  const { data: plansData, isLoading: plansLoading, isError: plansError, refetch: refetchPlans } = useBillingPlansQuery();
  const { data: subData, isLoading: subLoading, isError: subError, refetch: refetchSub } = useBillingSubscriptionQuery();
  const { data: invoicesData, isLoading: invoicesLoading, isError: invoicesError, refetch: refetchInvoices } = useBillingInvoicesQuery();
  const upgradeMutation = useUpgradeSubscription();

  const plans: ApiBillingPlan[] = Array.isArray(plansData) ? plansData : (plansData as unknown as { data?: ApiBillingPlan[] })?.data ?? [];
  const invoices: ApiBillingInvoice[] = Array.isArray(invoicesData) ? invoicesData : (invoicesData as unknown as { data?: ApiBillingInvoice[] })?.data ?? [];
  const currentPlanSlug = subData?.planSlug ?? null;

  const handleUpgrade = (slug: string) => {
    upgradeMutation.mutate(slug, {
      onSuccess: () => toast.success(`Upgraded to plan!`),
      onError: (err) => toast.error(`Upgrade failed: ${err.message}`),
    });
  };

  return (
    <AppLayout>
      <PageHeader
        title="Billing & Subscription Plans"
        description="Manage multi-tier gateway subscriptions, payment methods, and billing invoice history."
      />

      {(plansError || subError) && <ErrorState onRetry={() => { refetchPlans(); refetchSub(); }} />}

      {!plansError && !subError && (
        <>
          {plansLoading || subLoading ? (
            <p className="text-xs text-muted-foreground mb-6">Loading plans...</p>
          ) : plans.length === 0 ? (
            <EmptyState description="No billing plans available." />
          ) : (
            <div className="grid gap-6 md:grid-cols-3 mb-6">
              {plans.map((plan) => {
                const isCurrent = currentPlanSlug === plan.slug;
                const priceMonthlyUsd = (plan.priceMonthlyCents ?? 0) / 100;
                return (
                  <Card
                    key={plan.id}
                    className={`flex flex-col justify-between ${isCurrent ? 'border-primary shadow-md relative' : 'border-border'}`}
                  >
                    {isCurrent && <Badge variant="violet" className="absolute -top-2.5 right-4">CURRENT</Badge>}
                    <CardHeader>
                      <CardTitle className="text-lg font-bold">{plan.name}</CardTitle>
                      <CardDescription>{plan.features?.[0] ?? 'Plan feature'}</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                      <div className="font-mono text-3xl font-bold">
                        {priceMonthlyUsd === 0 ? 'Free' : `$${priceMonthlyUsd}`}
                        {priceMonthlyUsd > 0 && (
                          <span className="text-xs text-muted-foreground font-normal"> / month</span>
                        )}
                      </div>
                      <ul className="space-y-2 text-xs text-muted-foreground">
                        {plan.features?.map((feat, i) => (
                          <li key={i} className="flex items-center gap-2">
                            <Check className={`h-3.5 w-3.5 ${isCurrent ? 'text-primary' : 'text-emerald-500'}`} />
                            {feat}
                          </li>
                        ))}
                        <li className="flex items-center gap-2">
                          <Check className={`h-3.5 w-3.5 ${isCurrent ? 'text-primary' : 'text-emerald-500'}`} />
                          {((plan.includedTokensMonthly ?? 0) / 1000).toLocaleString()}K tokens/mo
                        </li>
                      </ul>
                    </CardContent>
                    <CardFooter className="pt-4">
                      {isCurrent ? (
                        <Button variant="outline" className="w-full text-xs" disabled>Current Plan</Button>
                      ) : (
                        <ConfirmDialog
                          title="Upgrade Plan"
                          description={`Upgrade your organization's subscription tier to "${plan.name}" for $${priceMonthlyUsd}/month?`}
                          confirmText="Confirm Upgrade"
                          onConfirm={() => handleUpgrade(plan.slug)}
                          trigger={
                            <Button
                              variant="prismViolet"
                              className="w-full text-xs"
                              disabled={upgradeMutation.isPending}
                            >
                              {upgradeMutation.isPending ? 'Upgrading...' : `Upgrade to ${plan.name}`}
                            </Button>
                          }
                        />
                      )}
                    </CardFooter>
                  </Card>
                );
              })}
            </div>
          )}

          <Card>
            <CardHeader>
              <CardTitle className="text-sm font-semibold flex items-center gap-2">
                <CreditCard className="h-4 w-4 text-primary" />
                <span>Invoice History</span>
              </CardTitle>
            </CardHeader>
            <CardContent>
              {invoicesLoading ? (
                <p className="text-xs text-muted-foreground">Loading invoices...</p>
              ) : invoicesError ? (
                <ErrorState onRetry={refetchInvoices} />
              ) : invoices.length === 0 ? (
                <EmptyState description="No invoices yet." />
              ) : (
                <div className="space-y-2">
                  <div className="grid grid-cols-5 gap-4 text-[11px] font-semibold text-muted-foreground uppercase tracking-wide px-3">
                    <span>Invoice</span>
                    <span>Period</span>
                    <span>Due</span>
                    <span>Paid</span>
                    <span>Status</span>
                  </div>
                  {invoices.map((inv) => (
                    <div key={inv.id} className="grid grid-cols-5 gap-4 items-center p-3 rounded-md border border-border bg-muted/20 text-xs">
                      <span className="font-mono font-semibold text-foreground">{inv.invoiceNumber}</span>
                      <span className="text-muted-foreground">
                        {new Date(inv.periodStart).toLocaleDateString()} - {new Date(inv.periodEnd).toLocaleDateString()}
                      </span>
                      <span className="font-mono text-foreground">${inv.amountDueUsd.toFixed(2)}</span>
                      <span className="font-mono text-foreground">${inv.amountPaidUsd.toFixed(2)}</span>
                      <Badge variant={inv.status === 'paid' ? 'success' : inv.status === 'open' ? 'warning' : 'default'}>
                        {inv.status}
                      </Badge>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </>
      )}
    </AppLayout>
  );
}
