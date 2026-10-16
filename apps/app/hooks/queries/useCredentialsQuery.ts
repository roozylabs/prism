import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { apiGetCredentials, PageParams } from '@/lib/api';
export { useDeleteCredentialMutation as useDeleteCredential } from '@/hooks/mutations/useCredentialMutations';

export function useCredentialsQuery(providerId: string = 'openai', params?: PageParams) {
  return useQuery({
    queryKey: ['credentials', providerId, params],
    queryFn: () => apiGetCredentials(providerId, params),
    placeholderData: keepPreviousData,
  });
}
