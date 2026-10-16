import { keepPreviousData, useQuery } from '@tanstack/react-query';
import { apiGetLogs } from '@/lib/api';

export function useLogsQuery(params?: {
  page?: number;
  limit?: number;
  provider?: string;
  model?: string;
  status?: number;
  search?: string;
  agentId?: string;
}) {
  return useQuery({
    queryKey: ['request-logs', params],
    queryFn: () => apiGetLogs(params),
    placeholderData: keepPreviousData,
  });
}
