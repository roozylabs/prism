import { api } from './client';
import { ApiTenantQuota } from './types/billing';
import { listData } from './envelope';

export async function apiGetQuotas(): Promise<ApiTenantQuota[]> {
  const response = await api.get<ApiTenantQuota[] | { data?: ApiTenantQuota[] }>('/quotas');
  return listData(response.data);
}

export async function apiUpdateQuota(targetType: string, targetId: string, data: Partial<ApiTenantQuota>): Promise<ApiTenantQuota> {
  const response = await api.put<ApiTenantQuota>(`/quotas/${targetType}/${targetId}`, data);
  return response.data;
}
