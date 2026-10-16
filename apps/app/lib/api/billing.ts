import { api } from './client';
import {
  ApiBillingPlan,
  ApiSubscriptionStatus,
  ApiBillingInvoice,
} from './types/billing';
import { listData } from './envelope';

export async function apiGetBillingPlans(): Promise<ApiBillingPlan[]> {
  const response = await api.get<ApiBillingPlan[] | { data?: ApiBillingPlan[] }>('/billing/plans');
  return listData(response.data);
}

export async function apiGetActiveSubscription(): Promise<ApiSubscriptionStatus> {
  const response = await api.get<ApiSubscriptionStatus>('/billing/subscription');
  return response.data;
}

export async function apiUpgradeSubscription(planId: string): Promise<ApiSubscriptionStatus> {
  const response = await api.post<ApiSubscriptionStatus>('/billing/subscription', { planId });
  return response.data;
}

export async function apiGetInvoices(): Promise<ApiBillingInvoice[]> {
  const response = await api.get<ApiBillingInvoice[] | { data?: ApiBillingInvoice[] }>('/billing/invoices');
  return listData(response.data);
}
