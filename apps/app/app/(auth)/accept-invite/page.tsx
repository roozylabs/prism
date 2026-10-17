'use client';

import { Suspense, useEffect, useState } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { AppRoutes } from '@/constants/routes';
import { AuthLayout } from '@/components/layouts/AuthLayout';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { toast } from 'sonner';
import { Loader2, ArrowRight, Building2 } from 'lucide-react';
import { useAuth } from '@/context/AuthContext';
import { apiGetInvitePreview } from '@/lib/api/organizations';
import { useAcceptInviteMutation } from '@/hooks/mutations/useMemberMutations';
import { parseApiError } from '@/lib/http/errors';

function AcceptInviteForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams?.get('token') || '';
  const { user, isAuthenticated, isLoading: authLoading } = useAuth();
  const { mutateAsync: acceptInvite, isPending: accepting } = useAcceptInviteMutation();
  const [done, setDone] = useState(false);

  const {
    data: preview,
    isLoading: previewLoading,
    isError: previewError,
  } = useQuery({
    queryKey: ['invite-preview', token],
    queryFn: () => apiGetInvitePreview(token),
    enabled: token.length > 0,
    retry: false,
  });

  useEffect(() => {
    if (done) {
      router.replace(AppRoutes.HOME);
      router.refresh();
    }
  }, [done, router]);

  const handleAccept = async () => {
    try {
      const result = await acceptInvite(token);
      toast.success(`You joined ${preview?.organizationName || 'the organization'} as ${result.role}.`);
      setDone(true);
    } catch (err: unknown) {
      toast.error(parseApiError(err, 'Could not accept the invitation').message);
    }
  };

  const invitePath = `${AppRoutes.ACCEPT_INVITE}?token=${encodeURIComponent(token)}`;
  const signInHref = `${AppRoutes.SIGNIN}?invite=${encodeURIComponent(token)}`;
  const signUpHref = `${AppRoutes.SIGNUP}?invite=${encodeURIComponent(token)}`;
  const emailMismatch = isAuthenticated && preview && user?.email && user.email.toLowerCase() !== preview.email.toLowerCase();

  return (
    <AuthLayout>
      <Card className="border-border shadow-xl">
        <CardHeader className="space-y-1 text-center">
          <CardTitle className="text-xl font-bold tracking-tight">Organization Invitation</CardTitle>
          <CardDescription>
            {!token
              ? 'This link is missing its invitation token.'
              : previewLoading
                ? 'Checking the invitation...'
                : previewError || !preview
                  ? 'This invitation is not valid any more. Ask the organization owner for a new one.'
                  : `You have been invited to join as ${preview.role}.`}
          </CardDescription>
        </CardHeader>
        {preview && !previewError && (
          <CardContent className="space-y-4">
            <div className="rounded-none border border-border bg-card/80 p-3.5 text-sm space-y-1">
              <div className="flex items-center gap-2 font-semibold">
                <Building2 className="h-4 w-4 text-primary" /> {preview.organizationName || 'Prism organization'}
              </div>
              <div className="text-xs text-muted-foreground">
                Invitation for <span className="font-medium text-foreground">{preview.email}</span>, expires{' '}
                {new Date(preview.expiresAt).toLocaleDateString()}.
              </div>
            </div>

            {authLoading ? (
              <p className="text-xs text-muted-foreground">Checking your session...</p>
            ) : isAuthenticated ? (
              emailMismatch ? (
                <div className="space-y-3">
                  <p className="text-xs text-muted-foreground">
                    You are signed in as <span className="font-medium text-foreground">{user?.email}</span>, but this
                    invitation was sent to {preview.email}. Sign in with that account to accept it.
                  </p>
                  <Button asChild variant="outline" className="w-full">
                    <Link href={signInHref}>Sign in with {preview.email}</Link>
                  </Button>
                </div>
              ) : (
                <Button variant="prismViolet" className="w-full" onClick={handleAccept} disabled={accepting || done}>
                  {accepting ? <Loader2 className="h-4 w-4 animate-spin" /> : <ArrowRight className="h-4 w-4" />}
                  Accept invitation
                </Button>
              )
            ) : (
              <div className="space-y-2">
                <p className="text-xs text-muted-foreground">
                  Sign in or create an account with <span className="font-medium text-foreground">{preview.email}</span>{' '}
                  to accept. You will be brought back here.
                </p>
                <Button asChild variant="prismViolet" className="w-full">
                  <Link href={signUpHref}>Create account</Link>
                </Button>
                <Button asChild variant="outline" className="w-full">
                  <Link href={signInHref}>I already have an account</Link>
                </Button>
                <p className="sr-only">{invitePath}</p>
              </div>
            )}
          </CardContent>
        )}
      </Card>
    </AuthLayout>
  );
}

export default function AcceptInvitePage() {
  return (
    <Suspense fallback={null}>
      <AcceptInviteForm />
    </Suspense>
  );
}
