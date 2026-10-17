"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";

import { Button } from "@/components/atoms/Button";
import { Input } from "@/components/atoms/Input";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/molecules/Select";
import { Sheet, SheetContent, SheetHeader, SheetFooter, SheetTitle, SheetDescription } from "@/components/molecules/Sheet";
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from "@/components/molecules/Form";
import { SecretReveal } from "@/components/molecules/SecretReveal";
import { toast } from "sonner";
import { useInviteMemberMutation } from "@/hooks/mutations/useMemberMutations";
import { parseApiError } from "@/lib/http/errors";

const inviteSchema = z.object({
  email: z.string().min(1, "Email address is required").email("Enter a valid email address"),
  role: z.string().default("developer"),
});

type InviteFormValues = z.infer<typeof inviteSchema>;

const defaultValues: InviteFormValues = {
  email: "",
  role: "developer",
};

interface InviteMemberSheetProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onInviteSuccess?: () => void;
}

export function InviteMemberSheet({ open, onOpenChange, onInviteSuccess }: InviteMemberSheetProps) {
  const { mutateAsync: inviteMember, isPending: isSending } = useInviteMemberMutation();
  // Shown when the server could not email the invitation: the link must be
  // passed on by hand and is displayed only once.
  const [inviteLink, setInviteLink] = useState<{ email: string; url: string } | null>(null);

  const form = useForm<InviteFormValues>({
    resolver: zodResolver(inviteSchema),
    defaultValues,
  });

  const { reset } = form;

  useEffect(() => {
    if (open) {
      reset(defaultValues);
      setInviteLink(null);
    }
  }, [open, reset]);

  const onSubmit = async (values: InviteFormValues) => {
    try {
      const result = await inviteMember({ email: values.email, role: values.role });
      if (onInviteSuccess) onInviteSuccess();
      if (!result.emailSent && result.inviteUrl) {
        setInviteLink({ email: result.invite.email, url: result.inviteUrl });
        return;
      }
      onOpenChange(false);
    } catch (err: unknown) {
      const apiErr = parseApiError(err, "Failed to create invitation");
      toast.error(apiErr.message);
    }
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Invite Organization Member</SheetTitle>
          <SheetDescription>
            The invitee gets a link to join with the role you choose. It expires in 7 days.
          </SheetDescription>
        </SheetHeader>
        {inviteLink ? (
          <div className="space-y-4 py-4">
            <p className="text-xs text-muted-foreground">
              Email delivery is not configured on this server, so the link was not sent. Pass it to{" "}
              <span className="font-medium text-foreground">{inviteLink.email}</span> yourself; it is shown only now.
            </p>
            <SecretReveal label="Invitation link" secret={inviteLink.url} />
            <SheetFooter>
              <Button type="button" variant="prismViolet" onClick={() => onOpenChange(false)}>
                Done
              </Button>
            </SheetFooter>
          </div>
        ) : (
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4 py-4">
              <FormField
                control={form.control}
                name="email"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel required>Email Address</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type="email"
                        placeholder="colleague@company.com"
                        disabled={isSending}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="role"
                render={({ field }) => (
                  <FormItem className="space-y-2">
                    <FormLabel required>Organization Role</FormLabel>
                    <FormControl>
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                        disabled={isSending}
                      >
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectItem value="owner">Owner</SelectItem>
                          <SelectItem value="developer">Developer</SelectItem>
                          <SelectItem value="agent_manager">Agent Manager</SelectItem>
                          <SelectItem value="finops_manager">FinOps Manager</SelectItem>
                          <SelectItem value="auditor">Security Auditor</SelectItem>
                          <SelectItem value="viewer">Read-Only Viewer</SelectItem>
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <SheetFooter>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => onOpenChange(false)}
                  disabled={isSending}
                >
                  Cancel
                </Button>
                <Button type="submit" variant="prismViolet" disabled={isSending}>
                  {isSending ? "Inviting..." : "Send Invitation"}
                </Button>
              </SheetFooter>
            </form>
          </Form>
        )}
      </SheetContent>
    </Sheet>
  );
}
