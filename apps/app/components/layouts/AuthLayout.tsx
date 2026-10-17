'use client';

import type { ReactNode } from 'react';
import { ThemeToggle } from '@/components/molecules/ThemeToggle';
import { AuthBackgroundAnimation } from '@/components/molecules/AuthBackgroundAnimation';
import { Shield } from 'lucide-react';

export function AuthLayout({ children }: { children: ReactNode }) {
  return (
    <div className="relative flex min-h-screen flex-col items-center justify-center bg-background p-4 sm:p-6 lg:p-8 overflow-hidden">
      {/* Dynamic AI & Data Theme-Adaptive Animated Background */}
      <AuthBackgroundAnimation />

      {/* Top Header Controls */}
      <div className="absolute right-4 top-4 z-20 flex items-center gap-2">
        <ThemeToggle />
      </div>

      {/* Brand Header */}
      <div className="relative z-10 mb-6 flex items-center gap-2.5">
        <div className="flex h-10 w-10 items-center justify-center rounded-none bg-[#7C3AED] text-white shadow-md border border-white/20">
          <Shield className="h-5 w-5" />
        </div>
        <div className="flex flex-col">
          <span className="font-mono text-lg font-bold tracking-tight text-foreground">
            RoozyLabs <span className="text-[#8B5CF6]">Prism</span>
          </span>
          <span className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground font-mono">
            Universal AI Control Plane v0.4.0
          </span>
        </div>
      </div>

      {/* Auth Content Container */}
      <div className="relative z-10 w-full max-w-md">
        {children}
      </div>

      {/* Footer System Info */}
      <footer className="relative z-10 mt-8 text-center text-xs text-muted-foreground font-mono">
        <span>© 2026 RoozyLabs Prism. Enterprise Security & Encryption Active.</span>
      </footer>
    </div>
  );
}
