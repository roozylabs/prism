import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { apiGetModelsPage, PageParams } from '@/lib/api';

/** One page of models for the models table. */
export function useModelsPageQuery(params: PageParams & { providerId?: string }) {
  return useQuery({
    queryKey: ['models-page', params],
    queryFn: () => apiGetModelsPage(params),
    placeholderData: keepPreviousData,
  });
}
