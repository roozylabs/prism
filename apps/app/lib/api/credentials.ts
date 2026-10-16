import { api } from './client';
import { ApiCredential, ApiTestCredentialResult } from './types/provider';
import { PageParams, PaginatedResult } from './types/common';

export async function apiGetCredentials(
  providerId?: string,
  params?: PageParams,
): Promise<PaginatedResult<ApiCredential>> {
  const url = providerId && providerId !== 'all' ? `/providers/${providerId}/credentials` : '/credentials';
  const response = await api.get<PaginatedResult<ApiCredential>>(url, { params });
  return response.data;
}

export async function apiCreateCredential(providerId: string, data: Partial<ApiCredential>): Promise<ApiCredential> {
  const response = await api.post<ApiCredential>(`/providers/${providerId}/credentials`, data);
  return response.data;
}

export async function apiUpdateCredential(providerId: string, id: string, data: Partial<ApiCredential>): Promise<ApiCredential> {
  const response = await api.put<ApiCredential>(`/providers/${providerId}/credentials/${id}`, data);
  return response.data;
}

export async function apiDeleteCredential(id: string, providerId?: string): Promise<{ message: string }> {
  const url = providerId ? `/providers/${providerId}/credentials/${id}` : `/credentials/${id}`;
  const response = await api.delete<{ message: string }>(url);
  return response.data;
}

export async function apiResetCredentialCooldown(id: string, providerId?: string): Promise<{ message: string }> {
  const url = providerId ? `/providers/${providerId}/credentials/${id}/reset-cooldown` : `/credentials/${id}/reset-cooldown`;
  const response = await api.post<{ message: string }>(url);
  return response.data;
}

export async function apiTestCredential(providerId: string, id: string): Promise<ApiTestCredentialResult> {
  const response = await api.post<ApiTestCredentialResult>(`/providers/${providerId}/credentials/${id}/test`);
  return response.data;
}

export async function apiRevealCredential(providerId: string, id: string): Promise<{ apiKey: string }> {
  const response = await api.post<{ apiKey: string }>(`/providers/${providerId}/credentials/${id}/reveal`);
  return response.data;
}
