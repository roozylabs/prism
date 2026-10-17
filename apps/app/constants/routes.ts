export enum AppRoutes {
  HOME = '/',
  SIGNIN = '/signin',
  SIGNUP = '/signup',
  ACCEPT_INVITE = '/accept-invite',
  ONBOARDING = '/onboarding',
  SANDBOX = '/sandbox',
  PROVIDERS = '/providers',
  CREDENTIALS = '/credentials',
  MODELS = '/models',
  POLICIES = '/policies',
  GATEWAY_KEYS = '/gateway-keys',
  TOOLS = '/tools',
  RESOURCES = '/resources',
  MCP = '/mcp',
  AGENTS = '/agents',
  GOVERNANCE = '/governance',
  AUDIT_TRAIL = '/audit-trail',
  LOGS = '/logs',
  BUDGETS = '/budgets',
  PLAYGROUND = '/playground',
  BILLING = '/settings/billing',
  SETTINGS = '/settings',
  SETTINGS_ORGANIZATION = '/settings/organization',
  SETTINGS_MEMBERS = '/settings/members',
}

export enum ApiEndpoints {
  SSE = '/api/sse',
  HEALTH = '/api/health',
  LOGIN = '/api/auth/login',
  SIGNUP = '/api/auth/signup',
  LOGOUT = '/api/auth/logout',
}

export enum CookieKeys {
  AUTH_TOKEN = 'auth_token',
}

export function mcpDetailRoute(id: string): string {
  return `${AppRoutes.MCP}/${id}`;
}

export function agentDetailRoute(id: string): string {
  return `${AppRoutes.AGENTS}/${id}`;
}
