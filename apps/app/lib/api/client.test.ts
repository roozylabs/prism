import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import Cookies from 'js-cookie';
import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios';
import { AxiosError } from 'axios';
import { api } from './client';
import { ApiError } from '@/lib/http/errors';
import { useTenantStore } from '@/stores/useTenantStore';

// Captures the config the interceptors produce and answers with a canned
// response (or a canned failure).
function stubAdapter(status = 200) {
  const seen: InternalAxiosRequestConfig[] = [];
  const adapter: AxiosAdapter = async (config) => {
    seen.push(config);
    const response = { data: { ok: true }, status, statusText: 'x', headers: {}, config };
    if (status >= 400) {
      throw new AxiosError('failed', String(status), config, null, response);
    }
    return response;
  };
  api.defaults.adapter = adapter;
  return seen;
}

describe('api client', () => {
  beforeEach(() => {
    Cookies.remove('auth_token');
    localStorage.clear();
    useTenantStore.getState().resetTenant();
    window.history.replaceState({}, '', '/');
  });
  afterEach(() => {
    Cookies.remove('auth_token');
  });

  it('attaches the bearer token and the selected tenant to every request', async () => {
    const seen = stubAdapter();
    Cookies.set('auth_token', 'tok_123');
    useTenantStore.getState().setSelectedOrg('org_1', 'Org One');
    useTenantStore.getState().setSelectedWorkspaceId('ws_1');

    await api.get('/gateway-keys');

    expect(seen).toHaveLength(1);
    expect(seen[0].headers.Authorization).toBe('Bearer tok_123');
    expect(seen[0].headers['X-Prism-Org-ID']).toBe('org_1');
    expect(seen[0].headers['X-Prism-Workspace-ID']).toBe('ws_1');
    expect(seen[0].baseURL).toBe('/api');
  });

  it('sends no auth or tenant headers when nothing is selected', async () => {
    const seen = stubAdapter();

    await api.get('/health');

    expect(seen[0].headers.Authorization).toBeUndefined();
    expect(seen[0].headers['X-Prism-Org-ID']).toBeUndefined();
  });

  it('turns failures into ApiError and drops the session cookie on 401', async () => {
    stubAdapter(401);
    Cookies.set('auth_token', 'stale');
    // On the sign-in page a 401 is a wrong password, so no redirect happens.
    window.history.replaceState({}, '', '/signin');

    await expect(api.post('/auth/login', {})).rejects.toMatchObject({ code: 'AUTH_REQUIRED', status: 401 });
    await expect(api.post('/auth/login', {})).rejects.toBeInstanceOf(ApiError);
    expect(Cookies.get('auth_token')).toBeUndefined();
    expect(window.location.pathname).toBe('/signin');
  });

  it('keeps other errors as ApiError with their status', async () => {
    stubAdapter(429);
    await expect(api.get('/logs')).rejects.toMatchObject({ code: 'RATE_LIMITED', status: 429, retryable: true });
  });
});
