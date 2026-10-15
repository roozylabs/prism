export interface PaginatedResult<T> {
  data: T[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
}

export interface ApiHealthResponse {
  status: string;
  uptime?: string;
  version?: string;
  database?: string;
  redis?: string;
}

export interface ApiSetting {
  id?: string;
  key: string;
  value: string;
  category?: string;
  // "organization" when the organization set it, "platform" for a default.
  scope?: 'organization' | 'platform';
}

export interface ApiOrganization {
  id: string;
  name: string;
  slug?: string;
  planTier?: string;
  createdAt?: string;
  updatedAt?: string;
}

export interface ApiOrganizationMember {
  id: string;
  organizationId: string;
  userId: string;
  userEmail?: string;
  userName?: string;
  role: string;
  createdAt: string;
  updatedAt: string;
}

export interface ApiWorkspace {
  id: string;
  orgId: string;
  name: string;
  slug?: string;
}

export interface ApiResponse<T> {
  data: T;
  message?: string;
  status?: string;
}
