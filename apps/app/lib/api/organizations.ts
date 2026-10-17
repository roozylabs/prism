import { api } from './client';
import { ApiOrganization, ApiOrganizationMember } from './types/common';

export interface ApiMemberInvite {
  id: string;
  organizationId: string;
  roleId: string;
  role?: string;
  email: string;
  status: 'pending' | 'accepted' | 'revoked' | 'expired';
  invitedBy?: string;
  expiresAt: string;
  createdAt: string;
}

export interface ApiInviteMemberResponse {
  message: string;
  invite: ApiMemberInvite;
  /** True when the invitation went out by email. */
  emailSent: boolean;
  /** Present only when no email was sent: hand this link to the invitee. */
  inviteUrl?: string;
}

export interface ApiInvitePreview {
  email: string;
  role: string;
  organizationName: string;
  expiresAt: string;
}

export async function apiGetOrganizations(): Promise<ApiOrganization[]> {
  const response = await api.get<ApiOrganization[] | { data: ApiOrganization[] }>('/user/organizations');
  if (response.data && typeof response.data === 'object' && 'data' in response.data && Array.isArray(response.data.data)) {
    return response.data.data;
  }
  return (response.data as ApiOrganization[]) || [];
}

export async function apiGetOrganizationMembers(): Promise<ApiOrganizationMember[]> {
  const response = await api.get<{ data: ApiOrganizationMember[] } | ApiOrganizationMember[]>('/organizations/members');
  if (response.data && typeof response.data === 'object' && 'data' in response.data && Array.isArray(response.data.data)) {
    return response.data.data;
  }
  return (response.data as ApiOrganizationMember[]) || [];
}

export async function apiInviteMember(data: { email: string; role: string }): Promise<ApiInviteMemberResponse> {
  const response = await api.post<ApiInviteMemberResponse>('/organizations/invites', data);
  return response.data;
}

export async function apiGetPendingInvites(): Promise<ApiMemberInvite[]> {
  const response = await api.get<{ data: ApiMemberInvite[] }>('/organizations/invites');
  return response.data?.data || [];
}

export async function apiRevokeInvite(inviteId: string): Promise<void> {
  await api.delete(`/organizations/invites/${inviteId}`);
}

/** Public: describes an invitation link before the holder signs in. */
export async function apiGetInvitePreview(token: string): Promise<ApiInvitePreview> {
  const response = await api.get<ApiInvitePreview>(`/auth/invites/${encodeURIComponent(token)}`);
  return response.data;
}

export async function apiAcceptInvite(token: string): Promise<{ message: string; organizationId: string; role: string }> {
  const response = await api.post<{ message: string; organizationId: string; role: string }>('/invites/accept', { token });
  return response.data;
}

export async function apiUpdateMemberRole(userId: string, role: string): Promise<{ message: string }> {
  const response = await api.put<{ message: string }>(`/organizations/members/${userId}`, { role });
  return response.data;
}

export async function apiRemoveMember(userId: string): Promise<{ message: string }> {
  const response = await api.delete<{ message: string }>(`/organizations/members/${userId}`);
  return response.data;
}
