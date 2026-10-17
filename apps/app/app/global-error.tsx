'use client';

import { useEffect } from 'react';
import { Shield, AlertOctagon, RefreshCw } from 'lucide-react';

export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error('[Prism Global Layout Crash]', error);
  }, [error]);

  return (
    <html lang="en">
      <body className="min-h-screen bg-[#090D16] text-[#F3F4F6] font-sans antialiased flex flex-col items-center justify-center p-4">
        <div className="relative z-10 w-full max-w-md border border-red-500/30 bg-[#0F172A] p-6 shadow-2xl space-y-6">
          {/* Brand Header */}
          <div className="flex items-center gap-3 border-b border-slate-800 pb-4">
            <div className="flex h-10 w-10 items-center justify-center bg-red-600 text-white shadow-md">
              <Shield className="h-5 w-5" />
            </div>
            <div className="flex flex-col">
              <span className="font-mono text-lg font-bold tracking-tight text-white">
                RoozyLabs <span className="text-red-400">Prism</span>
              </span>
              <span className="text-[10px] font-semibold uppercase tracking-wider text-slate-400 font-mono">
                Critical System Kernel Fallback
              </span>
            </div>
          </div>

          {/* Crash Notification */}
          <div className="space-y-2 text-center py-2">
            <div className="inline-flex items-center gap-1.5 px-2.5 py-1 text-xs font-mono font-medium text-red-400 bg-red-950/60 border border-red-800/60">
              <AlertOctagon className="h-3.5 w-3.5" />
              <span>FATAL ROOT LAYOUT FAILURE</span>
            </div>
            <h2 className="text-xl font-bold tracking-tight text-white mt-2">
              System Boundary Interrupted
            </h2>
            <p className="text-xs text-slate-400 max-w-xs mx-auto">
              A critical layout exception interrupted the root interface rendering cycle.
            </p>
          </div>

          {/* Error Details */}
          <div className="border border-slate-800 bg-black/60 p-3 font-mono text-xs text-slate-300">
            <p className="text-[10px] text-slate-500 uppercase mb-1">Error Signature:</p>
            <p className="font-semibold text-red-400 select-all break-all">
              {error.message || 'Fatal root initialization fault'}
            </p>
            {error.digest && (
              <p className="text-[10px] text-slate-500 mt-1">Digest: {error.digest}</p>
            )}
          </div>

          {/* Action Button */}
          <div className="pt-2 flex flex-col gap-2">
            <button
              type="button"
              onClick={() => reset()}
              className="w-full flex items-center justify-center gap-2 bg-primary hover:bg-primary/90 text-primary-foreground py-2.5 px-4 text-xs font-semibold transition-colors cursor-pointer"
            >
              <RefreshCw className="h-4 w-4" />
              Re-initialize Kernel UI
            </button>
            <button
              type="button"
              onClick={() => window.location.assign('/')}
              className="w-full flex items-center justify-center gap-2 border border-slate-700 hover:bg-slate-800 text-slate-300 py-2.5 px-4 text-xs font-semibold transition-colors cursor-pointer"
            >
              Reload Application Root
            </button>
          </div>

          <footer className="text-center text-[10px] font-mono text-slate-500 pt-2 border-t border-slate-800/80">
            © 2026 RoozyLabs Prism Control Plane.
          </footer>
        </div>
      </body>
    </html>
  );
}
