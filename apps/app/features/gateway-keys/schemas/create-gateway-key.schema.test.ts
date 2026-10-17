import { describe, expect, it } from 'vitest';
import { gatewayKeySchema } from './create-gateway-key.schema';

describe('gatewayKeySchema', () => {
  it('needs a name and a provider and defaults the rate limit', () => {
    const parsed = gatewayKeySchema.safeParse({ name: 'ci', providerId: 'prov_1' });
    expect(parsed.success).toBe(true);
    if (parsed.success) {
      expect(parsed.data).toEqual({ name: 'ci', providerId: 'prov_1', rateLimit: '100' });
    }
  });

  it('reports both missing fields', () => {
    const parsed = gatewayKeySchema.safeParse({ name: '', providerId: '', rateLimit: '50' });
    expect(parsed.success).toBe(false);
    if (!parsed.success) {
      expect(parsed.error.issues.map((i) => i.message)).toEqual(['Key name is required', 'Please select a provider']);
    }
  });
});
