import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, renderHook, waitFor } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

const apiGetTurnstileConfig = vi.fn();
vi.mock('@/lib/api', () => ({
  apiGetTurnstileConfig: () => apiGetTurnstileConfig(),
}));

import { useTurnstile } from './useTurnstile';

function wrapper({ children }: { children: React.ReactNode }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>;
}

describe('useTurnstile', () => {
  beforeEach(() => {
    vi.stubEnv('NEXT_PUBLIC_CLOUDFLARE_SITE_KEY', '');
  });
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.useRealTimers();
  });

  it('is ready at once and bypasses the challenge when the widget is disabled', async () => {
    apiGetTurnstileConfig.mockResolvedValue({ siteKey: 'disabled' });
    const { result } = renderHook(() => useTurnstile(), { wrapper });

    await waitFor(() => expect(result.current.showTurnstile).toBe(false));
    expect(result.current.isReady).toBe(true);
    await expect(result.current.getTokenOrWait()).resolves.toBe('bypass');
  });

  it('waits for the widget token and hands it to the caller', async () => {
    apiGetTurnstileConfig.mockResolvedValue({ siteKey: '1x00000000000000000000AA' });
    const { result } = renderHook(() => useTurnstile(), { wrapper });

    expect(result.current.showTurnstile).toBe(true);
    expect(result.current.isReady).toBe(false);

    const pending = result.current.getTokenOrWait(5000);
    act(() => result.current.onSuccess('tok_widget'));

    await expect(pending).resolves.toBe('tok_widget');
    expect(result.current.token).toBe('tok_widget');
    expect(result.current.isReady).toBe(true);
    // A second call returns the cached token without waiting.
    await expect(result.current.getTokenOrWait()).resolves.toBe('tok_widget');

    act(() => result.current.onExpire());
    expect(result.current.token).toBe('');
    expect(result.current.isReady).toBe(false);
  });

  it('resolves null when the widget errors and when the wait times out', async () => {
    vi.useFakeTimers();
    apiGetTurnstileConfig.mockResolvedValue({ siteKey: '1x00000000000000000000AA' });
    const { result } = renderHook(() => useTurnstile(), { wrapper });

    const failing = result.current.getTokenOrWait(1000);
    act(() => result.current.onError());
    await expect(failing).resolves.toBeNull();
    expect(result.current.hasError).toBe(true);

    const timingOut = result.current.getTokenOrWait(1000);
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1001);
    });
    await expect(timingOut).resolves.toBeNull();

    act(() => result.current.reset());
    expect(result.current.hasError).toBe(false);
  });
});
