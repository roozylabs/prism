// @vitest-environment node
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { NextRequest } from 'next/server';
import { middleware } from './middleware';

function request(path: string, token?: string, headers: Record<string, string> = {}) {
  const req = new NextRequest(new URL(path, 'https://app.example'), { headers });
  if (token) {
    req.cookies.set('auth_token', token);
  }
  return req;
}

// A stand-in for the backend's /api/user/permissions answer.
function permissions(isOnboarded: boolean, ok = true) {
  return vi.fn(async () => ({ ok, json: async () => ({ isOnboarded }) }) as unknown as Response);
}

function redirectTarget(res: Response) {
  return res.status === 307 ? new URL(res.headers.get('location') ?? '').pathname : null;
}

describe('middleware', () => {
  const originalFetch = globalThis.fetch;
  beforeEach(() => {
    vi.stubEnv('API_URL', 'http://api.test');
  });
  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.unstubAllEnvs();
  });

  it('sends anonymous visitors of protected pages to sign in', async () => {
    globalThis.fetch = vi.fn();
    const res = await middleware(request('/gateway-keys'));
    expect(redirectTarget(res)).toBe('/signin');
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it('lets anonymous visitors reach the auth pages, assets and proxied routes', async () => {
    globalThis.fetch = vi.fn();
    for (const path of ['/signin', '/signup', '/_next/static/chunk.js', '/favicon.ico', '/api/auth/login', '/v1/chat/completions', '/health']) {
      const res = await middleware(request(path));
      expect(redirectTarget(res), path).toBeNull();
      expect(res.headers.get('x-middleware-next'), path).toBe('1');
    }
    expect(globalThis.fetch).not.toHaveBeenCalled();
  });

  it('asks the backend who the token belongs to and routes an un-onboarded user to onboarding', async () => {
    const fetch = permissions(false);
    globalThis.fetch = fetch;

    expect(redirectTarget(await middleware(request('/', 'tok')))).toBe('/onboarding');
    expect(redirectTarget(await middleware(request('/onboarding', 'tok')))).toBeNull();

    const [url, init] = fetch.mock.calls[0] as unknown as [string, RequestInit];
    expect(url).toBe('http://api.test/api/user/permissions');
    expect(new Headers(init.headers).get('authorization')).toBe('Bearer tok');
    expect(new Headers(init.headers).get('cookie')).toBe('auth_token=tok');
  });

  it('keeps an onboarded user out of onboarding and the auth pages', async () => {
    globalThis.fetch = permissions(true);
    expect(redirectTarget(await middleware(request('/onboarding', 'tok')))).toBe('/');
    expect(redirectTarget(await middleware(request('/signin', 'tok')))).toBe('/');
    expect(redirectTarget(await middleware(request('/logs', 'tok')))).toBeNull();
  });

  it('falls through when the backend cannot be reached, except on auth pages', async () => {
    globalThis.fetch = vi.fn(async () => {
      throw new Error('ECONNREFUSED');
    });
    expect(redirectTarget(await middleware(request('/logs', 'tok')))).toBeNull();
    expect(redirectTarget(await middleware(request('/signin', 'tok')))).toBe('/');

    globalThis.fetch = permissions(false, false);
    expect(redirectTarget(await middleware(request('/logs', 'tok')))).toBeNull();
  });

  it('does not challenge the build-time page collector', async () => {
    globalThis.fetch = vi.fn();
    const res = await middleware(request('/logs', undefined, { 'user-agent': 'Next.js collector' }));
    expect(redirectTarget(res)).toBeNull();
  });
});
