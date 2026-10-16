'use client';

import { useState } from 'react';
import { Check, Copy, ShieldAlert } from 'lucide-react';
import { toast } from 'sonner';
import { Button } from '@/components/atoms/Button';

interface SecretRevealProps {
  label: string;
  secret: string;
}

/** Shows a secret the API returns only once, with a copy button and a warning. */
export function SecretReveal({ label, secret }: SecretRevealProps) {
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(secret);
      setCopied(true);
      toast.success('Copied to clipboard.');
      setTimeout(() => setCopied(false), 2000);
    } catch {
      toast.error('Could not copy; select the key and copy it manually.');
    }
  };

  return (
    <div className="rounded-none border border-border bg-card/80 p-3.5 font-mono text-xs text-left space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-muted-foreground text-[10px] uppercase font-semibold">{label}</span>
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={handleCopy}
          className="h-6 px-2 text-[11px] gap-1 cursor-pointer font-sans"
        >
          {copied ? <Check className="h-3 w-3 text-emerald-400" /> : <Copy className="h-3 w-3" />}
          <span>{copied ? 'Copied' : 'Copy Key'}</span>
        </Button>
      </div>
      <p className="font-mono text-xs text-emerald-400 bg-background/60 p-2.5 rounded border border-border/60 break-all select-all">
        {secret}
      </p>
      <div className="flex items-start gap-1.5 text-[11px] text-amber-400/90 font-sans mt-2 bg-amber-500/10 p-2 rounded border border-amber-500/20">
        <ShieldAlert className="h-4 w-4 shrink-0 mt-0.5" />
        <span>
          Please copy your API key now and store it securely. For security reasons, you will not be able to view this
          full key again.
        </span>
      </div>
    </div>
  );
}
