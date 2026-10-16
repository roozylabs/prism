import { api } from './client';
import { ApiModel } from './types/model';
import { prependSmartRouterModel } from './smart-router';
import { PageParams, PaginatedResult } from './types/common';
import { fetchAllPages } from './pagination';

/** One page of models, for the models table. */
export async function apiGetModelsPage(
  params: PageParams & { providerId?: string },
): Promise<PaginatedResult<ApiModel>> {
  const { providerId, ...query } = params;
  const url = providerId && providerId !== 'all' ? `/providers/${providerId}/models` : '/models';
  const response = await api.get<PaginatedResult<ApiModel>>(url, { params: query });
  return response.data;
}

/** Every model of a provider, with the smart router first, for pickers. */
export async function apiGetModels(providerId?: string): Promise<ApiModel[]> {
  const models = await fetchAllPages((page, limit) => apiGetModelsPage({ providerId, page, limit }));
  return prependSmartRouterModel(models);
}

/** Every model, with the smart router first, for pickers. */
export async function apiGetAllModels(): Promise<ApiModel[]> {
  return apiGetModels();
}

export async function apiCreateModel(providerId: string, data: Partial<ApiModel>): Promise<ApiModel> {
  const response = await api.post<ApiModel>(`/providers/${providerId}/models`, data);
  return response.data;
}

export async function apiUpdateModel(providerId: string, id: string, data: Partial<ApiModel>): Promise<ApiModel> {
  const response = await api.put<ApiModel>(`/providers/${providerId}/models/${id}`, data);
  return response.data;
}

export async function apiDeleteModel(providerId: string, id: string): Promise<{ message: string }> {
  const response = await api.delete<{ message: string }>(`/providers/${providerId}/models/${id}`);
  return response.data;
}
