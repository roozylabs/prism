"use client";

import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { zodResolver } from "@hookform/resolvers/zod";

import { Button } from "@/components/atoms/Button";
import { Input } from "@/components/atoms/Input";
import { Select, SelectTrigger, SelectValue, SelectContent, SelectItem } from "@/components/molecules/Select";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogFooter,
  DialogTitle,
  DialogDescription,
} from "@/components/molecules/Dialog";
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormMessage,
} from "@/components/molecules/Form";
import { useCreateGatewayKey } from "@/hooks/queries/useGatewayKeysQuery";
import type { ApiProvider } from "@/lib/api";
import { toast } from "sonner";
import { SecretReveal } from "@/components/molecules/SecretReveal";
import { getErrorMessage } from "@/types/ui";

import { gatewayKeySchema, GatewayKeyFormValues } from "@/features/gateway-keys/schemas/create-gateway-key.schema";

const defaultValues: GatewayKeyFormValues = {
  name: "",
  providerId: "",
  rateLimit: "100",
};

interface GatewayKeyFormDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  providers?: ApiProvider[];
}

export function GatewayKeyFormDialog({
  open,
  onOpenChange,
  providers,
}: GatewayKeyFormDialogProps) {
  const createMutation = useCreateGatewayKey();
  // The API returns the secret only in the create response; it is shown here
  // once and dropped when the dialog closes.
  const [createdKey, setCreatedKey] = useState<string | null>(null);

  const form = useForm<GatewayKeyFormValues>({
    resolver: zodResolver(gatewayKeySchema),
    defaultValues,
  });

  const { reset } = form;

  useEffect(() => {
    if (open) {
      reset(defaultValues);
      setCreatedKey(null);
    }
  }, [open, reset]);

  const onSubmit = (values: GatewayKeyFormValues) => {
    createMutation.mutate(
      {
        name: values.name.trim(),
        providerId: values.providerId,
        rateLimit: Number(values.rateLimit) || undefined,
      },
      {
        onSuccess: (created) => {
          toast.success("Gateway key created");
          setCreatedKey(created.key);
        },
        onError: (err) => {
          toast.error(`Failed to create gateway key: ${getErrorMessage(err)}`);
        },
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>{createdKey ? "Gateway Key Created" : "Create Gateway Key"}</DialogTitle>
          <DialogDescription>
            {createdKey
              ? "Use this key as the bearer token for requests to the gateway."
              : "Generate a new API key to authenticate requests through the gateway."}
          </DialogDescription>
        </DialogHeader>
        {createdKey ? (
          <div className="space-y-4 py-4">
            <SecretReveal label="Gateway Secret Key" secret={createdKey} />
            <DialogFooter>
              <Button type="button" variant="prismViolet" onClick={() => onOpenChange(false)}>
                Done
              </Button>
            </DialogFooter>
          </div>
        ) : (
        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-4 py-4">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel required>Key Name</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      placeholder="e.g. production-key"
                      disabled={createMutation.isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="providerId"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Provider</FormLabel>
                  <FormControl>
                    <Select
                      value={field.value}
                      onValueChange={field.onChange}
                      disabled={createMutation.isPending}
                    >
                      <SelectTrigger>
                        <SelectValue placeholder="Select a provider" />
                      </SelectTrigger>
                      <SelectContent>
                        {providers?.map((p) => (
                          <SelectItem key={p.id} value={p.id}>
                            {p.name}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name="rateLimit"
              render={({ field }) => (
                <FormItem className="space-y-2">
                  <FormLabel>Rate Limit (req/min)</FormLabel>
                  <FormControl>
                    <Input
                      {...field}
                      type="number"
                      placeholder="100"
                      disabled={createMutation.isPending}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <DialogFooter>
              <Button
                type="button"
                variant="outline"
                onClick={() => onOpenChange(false)}
                disabled={createMutation.isPending}
              >
                Cancel
              </Button>
              <Button
                type="submit"
                variant="prismViolet"
                disabled={createMutation.isPending}
              >
                {createMutation.isPending ? "Creating..." : "Create Key"}
              </Button>
            </DialogFooter>
          </form>
        </Form>
        )}
      </DialogContent>
    </Dialog>
  );
}
