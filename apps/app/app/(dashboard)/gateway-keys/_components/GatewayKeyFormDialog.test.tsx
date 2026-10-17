import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ApiProvider } from '@/lib/api';

// Radix Select does not run under jsdom; a native select stands in for it.
vi.mock('@/components/molecules/Select', () => ({
  Select: ({
    value,
    onValueChange,
    disabled,
    children,
    ...rest
  }: {
    value?: string;
    onValueChange?: (v: string) => void;
    disabled?: boolean;
    children?: React.ReactNode;
  }) => (
    <select aria-label="Provider" value={value ?? ''} onChange={(e) => onValueChange?.(e.target.value)} disabled={disabled} {...rest}>
      <option value="">Select a provider</option>
      {children}
    </select>
  ),
  SelectTrigger: () => null,
  SelectValue: () => null,
  SelectContent: ({ children }: { children?: React.ReactNode }) => <>{children}</>,
  SelectItem: ({ value, children }: { value: string; children?: React.ReactNode }) => <option value={value}>{children}</option>,
}));
vi.mock('sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const mutate = vi.fn();
vi.mock('@/hooks/queries/useGatewayKeysQuery', () => ({
  useCreateGatewayKey: () => ({ mutate, isPending: false }),
}));

import { toast } from 'sonner';
import { GatewayKeyFormDialog } from './GatewayKeyFormDialog';

const providers = [{ id: 'prov_openai', name: 'OpenAI' }, { id: 'prov_anthropic', name: 'Anthropic' }] as ApiProvider[];

describe('GatewayKeyFormDialog', () => {
  beforeEach(() => {
    mutate.mockReset();
  });

  it('validates before creating and then shows the secret exactly once', async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    mutate.mockImplementation((_values, opts?: { onSuccess?: (created: { key: string }) => void }) => {
      opts?.onSuccess?.({ key: 'prism_sk_1234567890' });
    });
    render(<GatewayKeyFormDialog open onOpenChange={onOpenChange} providers={providers} />);

    expect(screen.getByRole('heading', { name: 'Create Gateway Key' })).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Create Key' }));
    expect(await screen.findByText('Key name is required')).toBeInTheDocument();
    expect(screen.getByText('Please select a provider')).toBeInTheDocument();
    expect(mutate).not.toHaveBeenCalled();

    await user.type(screen.getByPlaceholderText('e.g. production-key'), '  production  ');
    await user.selectOptions(screen.getByLabelText('Provider'), 'prov_anthropic');
    await user.clear(screen.getByPlaceholderText('100'));
    await user.type(screen.getByPlaceholderText('100'), '250');
    await user.click(screen.getByRole('button', { name: 'Create Key' }));

    await waitFor(() => expect(mutate).toHaveBeenCalledTimes(1));
    expect(mutate.mock.calls[0][0]).toEqual({ name: 'production', providerId: 'prov_anthropic', rateLimit: 250 });
    expect(await screen.findByRole('heading', { name: 'Gateway Key Created' })).toBeInTheDocument();
    expect(screen.getByText('prism_sk_1234567890')).toBeInTheDocument();
    expect(toast.success).toHaveBeenCalledWith('Gateway key created');
    expect(screen.queryByPlaceholderText('e.g. production-key')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Done' }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it('reports a failed creation and keeps the form open', async () => {
    const user = userEvent.setup();
    mutate.mockImplementation((_values, opts?: { onError?: (err: Error) => void }) => {
      opts?.onError?.(new Error('provider has no active credentials'));
    });
    render(<GatewayKeyFormDialog open onOpenChange={vi.fn()} providers={providers} />);

    await user.type(screen.getByPlaceholderText('e.g. production-key'), 'ci');
    await user.selectOptions(screen.getByLabelText('Provider'), 'prov_openai');
    await user.click(screen.getByRole('button', { name: 'Create Key' }));

    await waitFor(() => expect(toast.error).toHaveBeenCalledWith('Failed to create gateway key: provider has no active credentials'));
    expect(screen.getByRole('heading', { name: 'Create Gateway Key' })).toBeInTheDocument();
    expect(screen.queryByText(/prism_sk_/)).not.toBeInTheDocument();
  });
});
