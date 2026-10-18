import { describe, expect, it } from 'vitest';
import { ApiError, parseApiError } from './errors';

describe('parseApiError', () => {
  it('maps the status to a code and keeps the gateway error payload', () => {
    const err = parseApiError({
      response: {
        status: 403,
        data: { error: { message: 'Denied by policy', type: 'governance_policy_denied', policy_id: 'pol_1', policy_name: 'Deny all', request_id: 'req_9' } },
      },
    });

    expect(err).toBeInstanceOf(ApiError);
    expect(err.code).toBe('FORBIDDEN');
    expect(err.status).toBe(403);
    expect(err.message).toBe('Denied by policy');
    expect(err.policyId).toBe('pol_1');
    expect(err.policyName).toBe('Deny all');
    expect(err.requestId).toBe('req_9');
    expect(err.retryable).toBe(false);
  });

  it.each([
    [401, 'AUTH_REQUIRED', false],
    [400, 'VALIDATION_ERROR', false],
    [422, 'VALIDATION_ERROR', false],
    [404, 'NOT_FOUND', false],
    [429, 'RATE_LIMITED', true],
    [502, 'INTERNAL_ERROR', true],
  ])('status %i becomes %s (retryable %s)', (status, code, retryable) => {
    const err = parseApiError({ response: { status, data: { message: 'x' } } });
    expect(err.code).toBe(code);
    expect(err.retryable).toBe(retryable);
    expect(err.message).toBe('x');
  });

  it('reads plain-string bodies, string error fields and the request id header', () => {
    expect(parseApiError({ response: { status: 500, data: '  upstream exploded  ' } }).message).toBe('upstream exploded');
    expect(parseApiError({ response: { status: 400, data: { error: 'bad field' } } }).message).toBe('bad field');
    const withHeader = parseApiError({ response: { status: 500, data: {}, headers: { 'x-request-id': 'hdr_1' } } });
    expect(withHeader.requestId).toBe('hdr_1');
    expect(withHeader.message).toBe('An unexpected API error occurred');
  });

  it('wraps plain errors and unknown values, and returns ApiError instances untouched', () => {
    const plain = parseApiError(new Error('offline'));
    expect(plain.message).toBe('offline');
    expect(plain.status).toBe(500);

    const unknown = parseApiError(42, 'fallback');
    expect(unknown.message).toBe('fallback');
    expect(unknown.code).toBe('INTERNAL_ERROR');

    const original = new ApiError({ message: 'keep me', code: 'BUDGET_EXCEEDED', status: 402 });
    expect(parseApiError(original)).toBe(original);
  });
});
