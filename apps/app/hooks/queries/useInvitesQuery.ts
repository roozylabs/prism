import { useQuery } from '@tanstack/react-query';
import { apiGetPendingInvites, ApiMemberInvite } from '@/lib/api/organizations';

export const PENDING_INVITES_QUERY_KEY = ['organization-invites'] as const;

export function usePendingInvitesQuery() {
  return useQuery<ApiMemberInvite[]>({
    queryKey: PENDING_INVITES_QUERY_KEY,
    queryFn: apiGetPendingInvites,
  });
}
