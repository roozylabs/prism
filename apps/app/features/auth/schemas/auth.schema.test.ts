import { describe, expect, it } from 'vitest';
import { loginSchema } from './login.schema';
import { signupSchema } from './signup.schema';

// First message per field, which is what the form shows.
function messages(result: { success: boolean; error?: { issues: { path: PropertyKey[]; message: string }[] } }) {
  const out: Record<string, string> = {};
  for (const issue of result.success ? [] : result.error!.issues) {
    const key = issue.path.join('.');
    if (!(key in out)) out[key] = issue.message;
  }
  return out;
}

describe('loginSchema', () => {
  it('accepts an e-mail and a password of six characters or more', () => {
    expect(loginSchema.safeParse({ email: 'ada@example.com', password: 'secret1' }).success).toBe(true);
  });

  it('names the field that is wrong', () => {
    expect(messages(loginSchema.safeParse({ email: '', password: '' }))).toEqual({
      email: 'Email is required',
      password: 'Password must be at least 6 characters',
    });
    expect(messages(loginSchema.safeParse({ email: 'not-an-email', password: 'secret1' }))).toEqual({
      email: 'Please enter a valid email address',
    });
  });
});

describe('signupSchema', () => {
  const valid = { name: 'Ada Lovelace', email: 'ada@example.com', password: 'correct horse', confirmPassword: 'correct horse' };

  it('accepts matching passwords of eight characters or more', () => {
    expect(signupSchema.safeParse(valid).success).toBe(true);
  });

  it('rejects short names and passwords, overlong passwords, and mismatches', () => {
    expect(messages(signupSchema.safeParse({ ...valid, name: 'A' }))).toEqual({ name: 'Full name must be at least 2 characters' });
    expect(messages(signupSchema.safeParse({ ...valid, password: 'short', confirmPassword: 'short' }))).toMatchObject({
      password: 'Password must be at least 8 characters',
    });
    const long = 'x'.repeat(73);
    expect(messages(signupSchema.safeParse({ ...valid, password: long, confirmPassword: long }))).toEqual({
      password: 'Password must be at most 72 characters',
    });
    expect(messages(signupSchema.safeParse({ ...valid, confirmPassword: 'correct horse!' }))).toEqual({
      confirmPassword: 'Passwords do not match',
    });
  });
});
