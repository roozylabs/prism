import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { apiGetAllGatewayKeys, apiGetGatewayKeys } from '@/lib/api';
export { useCreateGatewayKeyMutation as useCreateGatewayKey, useDeleteGatewayKeyMutation as useDeleteGatewayKey } from '@/hooks/mutations/useGatewayKeyMutations';

export function useGatewayKeysQuery(params?: Parameters<typeof apiGetGatewayKeys>[0]) {
  return useQuery({
    queryKey: ['gateway-keys', params],
    queryFn: () => apiGetGatewayKeys(params),
    placeholderData: keepPreviousData,
  });
}

/** Every gateway key, for pickers. */
export function useAllGatewayKeysQuery() {
  return useQuery({
    queryKey: ['gateway-keys', 'all'],
    queryFn: apiGetAllGatewayKeys,
  });
}
