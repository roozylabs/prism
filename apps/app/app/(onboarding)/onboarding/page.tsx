'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useQueryClient } from '@tanstack/react-query';
import { AuthLayout } from '@/components/layouts/AuthLayout';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from '@/components/molecules/Card';
import { Form, FormField, FormItem, FormLabel, FormControl, FormMessage, FormDescription } from '@/components/molecules/Form';
import { Input } from '@/components/atoms/Input';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { CheckCircle2, ArrowRight, LogOut, Zap, User as UserIcon } from 'lucide-react';
import { SecretReveal } from '@/components/molecules/SecretReveal';
import { onboardingSchema, OnboardingValues } from '@/features/onboarding/schemas/onboarding.schema';
import { useAuth } from '@/context/AuthContext';
import { useUserPermissionsQuery } from '@/hooks/queries/useUserPermissionsQuery';
import { useOnboardingMutation } from '@/hooks/mutations/useOnboardingMutation';

import { parseApiError } from '@/lib/http/errors';
import { toast } from 'sonner';

export default function OnboardingPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const { user, logout } = useAuth();
  const [step, setStep] = useState(1);
  const [createdKey, setCreatedKey] = useState<string | null>(null);
  const [justCompleted, setJustCompleted] = useState(false);

  const { data: permissions, isLoading: permissionsLoading } = useUserPermissionsQuery();
  const { mutateAsync: completeOnboarding, isPending: loading } = useOnboardingMutation();

  useEffect(() => {
    // Only redirect if user was already onboarded before visiting and not in the middle of completing setup
    if (!justCompleted && step !== 3 && !permissionsLoading && permissions?.isOnboarded === true) {
      router.replace('/');
    }
  }, [permissions, permissionsLoading, router, step, justCompleted]);

  const form = useForm<OnboardingValues>({
    resolver: zodResolver(onboardingSchema),
    mode: 'onTouched',
    defaultValues: {
      organizationName: '',
      workspaceName: '',
      gatewayKeyName: '',
    },
  });

  // Calculate smart suggestions based on user profile
  const suggestedOrgName = (() => {
    if (user?.name && user.name.toLowerCase() !== 'developer' && !user.name.includes('Developer')) {
      return `${user.name}'s Org`;
    }
    if (user?.email && user.email.includes('@')) {
      const domain = user.email.split('@')[1];
      if (domain && !['gmail.com', 'yahoo.com', 'hotmail.com', 'outlook.com'].includes(domain)) {
        const company = domain.split('.')[0];
        return company.charAt(0).toUpperCase() + company.slice(1) + ' Enterprise';
      }
    }
    return 'RoozyLabs Enterprise';
  })();

  const handleFillDefaults = () => {
    form.setValue('organizationName', suggestedOrgName, { shouldValidate: true });
    form.setValue('workspaceName', 'Production Environment', { shouldValidate: true });
    form.setValue('gatewayKeyName', 'Primary Control Key', { shouldValidate: true });
    toast.info('Suggested organization values applied.');
  };

  const handleNextStep = async () => {
    const isValid = await form.trigger(['organizationName', 'workspaceName']);
    if (isValid) {
      setStep(2);
    }
  };

  const onSubmit = async (values: OnboardingValues) => {
    try {
      const res = await completeOnboarding(values);
      setJustCompleted(true);
      setCreatedKey(res.apiKey || null);
      setStep(3);
      toast.success('Onboarding complete! Your workspace is ready.');
    } catch (err: unknown) {
      const apiErr = parseApiError(err, 'Failed to complete workspace onboarding. Please try again.');
      toast.error(apiErr.message);
    }
  };

  return (
    <AuthLayout>
      {/* Top User Context & Sign Out Bar */}
      <div className="flex items-center justify-between bg-card/70 border border-border px-3.5 py-2 mb-3 shadow-sm backdrop-blur-sm">
        <div className="flex items-center gap-2 min-w-0">
          <div className="h-6 w-6 rounded-full bg-secondary text-foreground flex items-center justify-center font-mono text-[10px] font-bold shrink-0">
            {user?.name ? user.name.charAt(0).toUpperCase() : <UserIcon className="h-3 w-3" />}
          </div>
          <span className="text-xs text-muted-foreground truncate font-mono">
            {user?.email || 'Logged in user'}
          </span>
        </div>

        <Button
          type="button"
          variant="ghost"
          size="sm"
          onClick={() => logout()}
          className="h-7 px-2 text-xs text-muted-foreground hover:text-destructive hover:bg-destructive/10 gap-1.5 cursor-pointer shrink-0"
        >
          <LogOut className="h-3.5 w-3.5" />
          <span>Sign Out</span>
        </Button>
      </div>

      <Card className="border-border shadow-xl">
        <CardHeader className="space-y-1 text-center">
          <Badge variant="violet" className="mx-auto mb-2">Step {step} of 3</Badge>
          <CardTitle className="text-xl font-bold tracking-tight">
            {step === 1 ? 'Configure Organization' : step === 2 ? 'Create Gateway Key' : 'Setup Complete!'}
          </CardTitle>
          <CardDescription>
            {step === 1 ? 'Set up your primary enterprise organization & workspace' : step === 2 ? 'Generate your first gateway key for LLM requests' : 'Your workspace and API key are configured and active'}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {step < 3 ? (
            <Form {...form}>
              <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4">
                {step === 1 && (
                  <>
                    <FormField
                      control={form.control}
                      name="organizationName"
                      render={({ field }) => (
                        <FormItem>
                          <div className="flex items-center justify-between">
                            <FormLabel required>Organization Name</FormLabel>
                            <button
                              type="button"
                              onClick={handleFillDefaults}
                              className="text-[11px] font-mono text-muted-foreground hover:text-foreground flex items-center gap-1 cursor-pointer"
                            >
                              <Zap className="h-3 w-3" />
                              <span>Auto-fill</span>
                            </button>
                          </div>
                          <FormControl>
                            <Input placeholder={`e.g. ${suggestedOrgName}`} required {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name="workspaceName"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel required>Default Workspace Name</FormLabel>
                          <FormControl>
                            <Input placeholder="e.g. Production Environment" required {...field} />
                          </FormControl>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <Button type="button" variant="prismViolet" className="w-full gap-2 mt-4" onClick={handleNextStep}>
                      Continue to Key Setup <ArrowRight className="h-4 w-4" />
                    </Button>
                  </>
                )}

                {step === 2 && (
                  <>
                    <FormField
                      control={form.control}
                      name="gatewayKeyName"
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel required>First Gateway Key Label</FormLabel>
                          <FormControl>
                            <Input placeholder="e.g. Primary Control Key" required {...field} />
                          </FormControl>
                          <FormDescription>Bound to default workspace for OpenAI/Gemini requests</FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <div className="flex gap-2 mt-4">
                      <Button type="button" variant="outline" onClick={() => setStep(1)} disabled={loading}>
                        Back
                      </Button>
                      <Button type="submit" variant="prismViolet" className="flex-1 gap-2" disabled={loading}>
                        {loading ? 'Initializing Workspace...' : 'Complete Setup'}
                      </Button>
                    </div>
                  </>
                )}
              </form>
            </Form>
          ) : (
            <div className="space-y-4 text-center">
              <CheckCircle2 className="mx-auto h-12 w-12 text-emerald-500" />
              
              <SecretReveal label="Generated Gateway Secret Key" secret={createdKey ?? ''} />

              <Button
                variant="prismViolet"
                className="w-full gap-2 mt-4"
                onClick={async () => {
                  await queryClient.invalidateQueries({ queryKey: ['user-permissions'] });
                  await queryClient.invalidateQueries({ queryKey: ['auth', 'me'] });
                  router.push('/');
                  router.refresh();
                }}
              >
                Go to Control Plane Console <ArrowRight className="h-4 w-4" />
              </Button>
            </div>
          )}
        </CardContent>
      </Card>
    </AuthLayout>
  );
}
