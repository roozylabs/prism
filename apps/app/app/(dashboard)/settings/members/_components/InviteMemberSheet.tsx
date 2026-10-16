"use client";

import { useEffect } from "react";
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

  const form = useForm<InviteFormValues>({
    resolver: zodResolver(inviteSchema),
    defaultValues,
  });

  const { reset } = form;

  useEffect(() => {
    if (open) reset(defaultValues);
  }, [open, reset]);

  const onSubmit = async (values: InviteFormValues) => {
    try {
      await inviteMember({ email: values.email, role: values.role });
      onOpenChange(false);
      if (onInviteSuccess) onInviteSuccess();
    } catch (err: unknown) {
      const apiErr = parseApiError(err, "Failed to send invitation");
      toast.error(apiErr.message);
    }
  };

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>Invite Organization Member</SheetTitle>
          <SheetDescription>
            Send an invitation with specified organization RBAC role.
          </SheetDescription>
        </SheetHeader>
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
                {isSending ? "Sending..." : "Send Invitation"}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  );
}
