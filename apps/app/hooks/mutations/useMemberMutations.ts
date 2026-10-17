import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  apiAcceptInvite,
  apiInviteMember,
  apiRemoveMember,
  apiRevokeInvite,
  apiUpdateMemberRole,
} from '@/lib/api/organizations';
import { PENDING_INVITES_QUERY_KEY } from '@/hooks/queries/useInvitesQuery';
import { toast } from 'sonner';

export function useInviteMemberMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ email, role }: { email: string; role: string }) =>
      apiInviteMember({ email, role }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: PENDING_INVITES_QUERY_KEY });
      if (data.emailSent) {
        toast.success(`Invitation emailed to ${data.invite.email}.`);
      } else {
        toast.success('Invitation created. Share the link with the invitee.');
      }
    },
  });
}

export function useRevokeInviteMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (inviteId: string) => apiRevokeInvite(inviteId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PENDING_INVITES_QUERY_KEY });
      toast.success('Invitation revoked.');
    },
  });
}

export function useAcceptInviteMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (token: string) => apiAcceptInvite(token),
    onSuccess: () => {
      // The user now belongs to another organization and may have been onboarded.
      queryClient.invalidateQueries({ queryKey: ['auth', 'me'] });
      queryClient.invalidateQueries({ queryKey: ['organizations'] });
      queryClient.invalidateQueries({ queryKey: ['user', 'organizations'] });
    },
  });
}

export function useRemoveMemberMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (memberId: string) => apiRemoveMember(memberId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organization-members'] });
      toast.success('Member removed successfully.');
    },
  });
}

export function useUpdateMemberRoleMutation() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ memberId, role }: { memberId: string; role: string }) =>
      apiUpdateMemberRole(memberId, role),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['organization-members'] });
      toast.success('Member role updated successfully.');
    },
  });
}
