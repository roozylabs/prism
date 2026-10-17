'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { AuthBackgroundAnimation } from '@/components/molecules/AuthBackgroundAnimation';
import { ThemeToggle } from '@/components/molecules/ThemeToggle';
import { Card, CardHeader, CardTitle, CardDescription, CardContent, CardFooter } from '@/components/molecules/Card';
import { Button } from '@/components/atoms/Button';
import { Badge } from '@/components/atoms/Badge';
import { Shield, Home, Terminal, Key, ArrowLeft, HelpCircle } from 'lucide-react';

export default function NotFoundPage() {
  const pathname = usePathname();

  return (
    <div className="relative flex min-h-screen flex-col items-center justify-center bg-background p-4 sm:p-6 lg:p-8 overflow-hidden">
      {/* Dynamic AI & Mesh Animated Background */}
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

      {/* 404 Main Card */}
      <div className="relative z-10 w-full max-w-lg">
        <Card className="border-border shadow-2xl backdrop-blur-sm bg-card/95">
          <CardHeader className="text-center space-y-2 pb-4">
            <div className="flex justify-center mb-1">
              <Badge variant="violet" className="font-mono text-xs px-3 py-0.5">
                HTTP 404 • ROUTE UNMAPPED
              </Badge>
            </div>
            <div className="relative py-2">
              <span className="text-7xl font-extrabold font-mono tracking-widest text-foreground select-none">
                404
              </span>
            </div>
            <CardTitle className="text-xl font-bold tracking-tight">
              Gateway Endpoint Not Found
            </CardTitle>
            <CardDescription className="text-xs text-muted-foreground max-w-sm mx-auto">
              The requested routing destination does not exist on the RoozyLabs Prism gateway mesh or has been decommissioned.
            </CardDescription>
          </CardHeader>

          <CardContent className="space-y-4">
            {/* Target Path Diagnostic Box */}
            <div className="rounded-none border border-border bg-muted/40 p-3 font-mono text-xs">
              <div className="flex items-center justify-between text-[11px] text-muted-foreground mb-1">
                <span className="uppercase font-semibold">Unresolved Target Route:</span>
                <span className="text-muted-foreground">UNRESOLVED_URI</span>
              </div>
              <p className="font-semibold text-foreground break-all select-all bg-background/80 px-2 py-1 border border-border/60">
                {pathname || '/unknown-route'}
              </p>
            </div>

            {/* Quick Navigation Surface */}
            <div className="space-y-2">
              <p className="text-[11px] font-mono uppercase text-muted-foreground tracking-wider font-semibold">
                Available Gateway Surfaces:
              </p>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                <Button asChild variant="outline" size="sm" className="justify-start gap-2 h-9 text-xs">
                  <Link href="/logs">
                    <Terminal className="h-3.5 w-3.5 text-muted-foreground" />
                    <span>Real-Time Logs</span>
                  </Link>
                </Button>
                <Button asChild variant="outline" size="sm" className="justify-start gap-2 h-9 text-xs">
                  <Link href="/gateway-keys">
                    <Key className="h-3.5 w-3.5 text-muted-foreground" />
                    <span>Gateway Keys</span>
                  </Link>
                </Button>
                <Button asChild variant="outline" size="sm" className="justify-start gap-2 h-9 text-xs">
                  <Link href="/models">
                    <Shield className="h-3.5 w-3.5 text-muted-foreground" />
                    <span>Model Catalog</span>
                  </Link>
                </Button>
                <Button asChild variant="outline" size="sm" className="justify-start gap-2 h-9 text-xs">
                  <Link href="/playground">
                    <HelpCircle className="h-3.5 w-3.5 text-muted-foreground" />
                    <span>Prompt Playground</span>
                  </Link>
                </Button>
              </div>
            </div>
          </CardContent>

          <CardFooter className="flex flex-col sm:flex-row gap-2 pt-2 border-t border-border/50">
            <Button asChild variant="outline" className="w-full sm:w-1/2 gap-2 text-xs" onClick={() => window.history.back()}>
              <button type="button">
                <ArrowLeft className="h-4 w-4" />
                Go Back
              </button>
            </Button>
            <Button asChild variant="prismViolet" className="w-full sm:w-1/2 gap-2 text-xs">
              <Link href="/">
                <Home className="h-4 w-4" />
                Return to Dashboard
              </Link>
            </Button>
          </CardFooter>
        </Card>
      </div>

      {/* Footer System Info */}
      <footer className="relative z-10 mt-8 text-center text-xs text-muted-foreground font-mono">
        <span>© 2026 RoozyLabs Prism. Fault-Tolerant AI Infrastructure Engine.</span>
      </footer>
    </div>
  );
}
