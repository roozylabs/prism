'use client';

import { useState, useEffect, Suspense } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { useForm } from 'react-hook-form';
import { useTheme } from 'next-themes';
import { Turnstile } from '@marsidev/react-turnstile';
import { AppRoutes } from '@/constants/routes';
import { zodResolver } from '@hookform/resolvers/zod';
import { AuthLayout } from '@/components/layouts/AuthLayout';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/molecules/Card';
import { Form, FormField, FormItem, FormLabel, FormControl, FormMessage } from '@/components/molecules/Form';
import { Input } from '@/components/atoms/Input';
import { Button } from '@/components/atoms/Button';
import { GoogleIcon, GitHubIcon } from '@/components/icons';
import { toast } from 'sonner';
import { ArrowRight, Loader2 } from 'lucide-react';
import { useAuth } from '@/context/AuthContext';
import { useTurnstile } from '@/hooks/useTurnstile';
import { loginSchema, LoginFormValues } from '@/features/auth/schemas/login.schema';

function SignInForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const oauthError = searchParams?.get('error');
  const { resolvedTheme } = useTheme();
  const { login, isAuthenticated } = useAuth();
  const [loading, setLoading] = useState(false);
  const [oauthLoading, setOauthLoading] = useState<'google' | 'github' | null>(null);

  const {
    siteKey,
    showTurnstile,
    token: turnstileToken,
    hasError,
    turnstileRef,
    onSuccess,
    onError,
    onExpire,
    reset: resetTurnstile,
    getTokenOrWait,
  } = useTurnstile();

  useEffect(() => {
    if (isAuthenticated) {
      router.replace(AppRoutes.HOME);
    }
  }, [isAuthenticated, router]);

  useEffect(() => {
    if (oauthError) {
      if (oauthError === 'session_creation_failed') {
        toast.error('OAuth sign-in failed. Please try again or sign in with your password.');
      } else if (oauthError === 'missing_code') {
        toast.error('OAuth authorization code was missing.');
      } else if (oauthError === 'invalid_state') {
        toast.error('This sign-in link expired or was not started here. Please sign in again.');
      } else if (oauthError === 'email_unverified') {
        toast.error('Your account has no verified email address. Verify an email with the provider, then try again.');
      } else if (oauthError === 'oauth_not_configured') {
        toast.error('This sign-in method is not available. Sign in with your email and password.');
      } else if (oauthError === 'oauth_failed' || oauthError === 'oauth_denied') {
        toast.error('Sign-in with the provider did not complete. Please try again.');
      } else {
        toast.error(`Authentication error: ${oauthError}`);
      }
    }
  }, [oauthError]);

  const form = useForm<LoginFormValues>({
    resolver: zodResolver(loginSchema),
    defaultValues: {
      email: '',
      password: '',
    },
  });

  const onSubmit = async (values: LoginFormValues) => {
    try {
      setLoading(true);

      let activeToken = turnstileToken;
      if (showTurnstile && !activeToken) {
        if (hasError) {
          toast.error('Security verification failed to load. Please check your connection or ad-blocker.');
          return;
        }
        activeToken = (await getTokenOrWait(3500)) || '';
        if (!activeToken) {
          toast.error('Please complete the security verification challenge before signing in.');
          return;
        }
      }

      await login({
        email: values.email,
        password: values.password,
        turnstileToken: activeToken && activeToken !== 'bypass' ? activeToken : undefined,
      });
      toast.success('Authentication successful! Welcome to RoozyLabs Prism.');
      router.replace(AppRoutes.HOME);
      router.refresh();
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'Invalid credentials. Please try again.';
      toast.error(message);
      resetTurnstile();
    } finally {
      setLoading(false);
    }
  };

  const handleOAuthLogin = async (provider: 'google' | 'github') => {
    try {
      setOauthLoading(provider);
      toast.info(`Connecting to ${provider === 'google' ? 'Google' : 'GitHub'} OAuth...`);
      if (provider === 'google') {
        window.location.href = '/api/auth/oauth/google';
      } else {
        window.location.href = '/api/auth/oauth/github';
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : 'OAuth redirection failed.';
      toast.error(message);
      setOauthLoading(null);
    }
  };

  return (
    <AuthLayout>
      <Card className="border-border shadow-xl">
        <CardHeader className="space-y-1 text-center">
          <CardTitle className="text-xl font-bold tracking-tight">Sign in to Prism Console</CardTitle>
          <CardDescription>
            Enter your credentials to access your universal AI control plane
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel required>Email Address</FormLabel>
                    <FormControl>
                      <Input
                        placeholder="Enter your email"
                        disabled={loading}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name="password"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel required>Password</FormLabel>
                    <FormControl>
                      <Input
                        type="password"
                        placeholder="Enter your password"
                        disabled={loading}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {showTurnstile && (
                <div className="flex flex-col items-start justify-center my-3 min-h-[65px] gap-1.5">
                  <Turnstile
                    ref={turnstileRef}
                    siteKey={siteKey}
                    onSuccess={onSuccess}
                    onError={onError}
                    onExpire={onExpire}
                    options={{
                      theme: resolvedTheme === 'dark' ? 'dark' : 'light',
                      size: 'normal',
                    }}
                  />
                </div>
              )}

              <Button
                type="submit"
                variant="prismViolet"
                className="w-full gap-2 mt-2 cursor-pointer"
                disabled={loading}
              >
                {loading ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>Authenticating...</span>
                  </>
                ) : (
                  <>
                    <span>Sign In to Console</span>
                    <ArrowRight className="h-4 w-4" />
                  </>
                )}
              </Button>
            </form>
          </Form>

          <div className="relative my-6">
            <div className="absolute inset-0 flex items-center">
              <span className="w-full border-t border-border" />
            </div>
            <div className="relative flex justify-center text-xs uppercase">
              <span className="bg-card px-2 text-muted-foreground font-mono">Or continue with</span>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="w-full text-xs gap-2 cursor-pointer hover:bg-muted/60 transition-colors"
              disabled={loading || oauthLoading !== null}
              onClick={() => handleOAuthLogin('google')}
            >
              {oauthLoading === 'google' ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <GoogleIcon className="h-3.5 w-3.5" />
              )}
              <span>Google</span>
            </Button>

            <Button
              type="button"
              variant="outline"
              size="sm"
              className="w-full text-xs gap-2 cursor-pointer hover:bg-muted/60 transition-colors"
              disabled={loading || oauthLoading !== null}
              onClick={() => handleOAuthLogin('github')}
            >
              {oauthLoading === 'github' ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <GitHubIcon className="h-3.5 w-3.5" />
              )}
              <span>GitHub</span>
            </Button>
          </div>

          <div className="mt-6 text-center text-xs text-muted-foreground">
            Don&apos;t have an account?{' '}
            <Link
              href={AppRoutes.SIGNUP}
              className="font-medium text-foreground hover:text-foreground underline underline-offset-4 transition-colors"
            >
              Sign up
            </Link>
          </div>
        </CardContent>
      </Card>
    </AuthLayout>
  );
}

export default function SignInPage() {
  return (
    <Suspense fallback={null}>
      <SignInForm />
    </Suspense>
  );
}
