/** One admin audit log entry, as GET /audit-trail/logs returns it. */
export interface ApiAuditLogItem {
  id: string;
  organizationId: string;
  createdAt: string;
  actorId: string;
  actorEmail?: string;
  action: string;
  resource?: string;
  resourceId?: string;
  /** JSON string with identifiers and names of what changed; never secrets. */
  detailsJson?: string;
  actorIp?: string;
  actorUserAgent?: string;
  status: 'success' | 'denied' | 'flagged' | string;
}

export interface ApiAIAuditTrail {
  id: string;
  requestId: string;
  organizationId: string;
  actorId: string;
  userId?: string;
  userRole?: string;
  agentName?: string;
  actorEmail?: string;
  action: string;
  model: string;
  modelSlug?: string;
  provider: string;
  complianceStatus?: string;
  promptSanitized?: string;
  responseSummary?: string;
  totalTokens: number;
  totalCostUsd: number;
  ipAddress?: string;
  userAgent?: string;
  checksum: string;
  verified: boolean;
  createdAt: string;
}

export interface ApiAuditVerificationResult {
  id: string;
  requestId: string;
  verified: boolean;
  valid?: boolean;
  tampered: boolean;
  signatureHash?: string;
  message?: string;
  calculatedChecksum: string;
  storedChecksum: string;
  checkedAt: string;
}
