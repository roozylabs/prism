import { api } from './client';
import { ApiGatewayKey } from './types/gateway-key';
import { PageParams, PaginatedResult } from './types/common';
import { fetchAllPages } from './pagination';

export async function apiGetGatewayKeys(params?: PageParams): Promise<PaginatedResult<ApiGatewayKey>> {
  const response = await api.get<PaginatedResult<ApiGatewayKey>>('/gateway-keys', { params });
  return response.data;
}

/** Every gateway key of the user, for pickers. */
export async function apiGetAllGatewayKeys(): Promise<ApiGatewayKey[]> {
  return fetchAllPages((page, limit) => apiGetGatewayKeys({ page, limit }));
}

/** A new key with its secret; the API returns the secret only this once. */
export type ApiCreatedGatewayKey = ApiGatewayKey & { key: string };

export async function apiCreateGatewayKey(data: {
  name: string;
  providerId?: string;
  rateLimit?: number | string;
  budgetLimitMonthly?: number;
  allowedModels?: string[];
  allowedProviders?: string[];
  expiresInDays?: number;
}): Promise<ApiCreatedGatewayKey> {
  const response = await api.post<ApiCreatedGatewayKey>('/gateway-keys', data);
  return response.data;
}

export async function apiDeleteGatewayKey(id: string): Promise<{ message: string }> {
  const response = await api.delete<{ message: string }>(`/gateway-keys/${id}`);
  return response.data;
}
