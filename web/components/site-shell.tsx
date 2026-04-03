import Link from "next/link";
import { ReactNode } from "react";

import { messages } from "@/lib/messages";

export function SiteShell({
  children,
  compact = false
}: {
  children: ReactNode;
  compact?: boolean;
}) {
  return (
    <div className="min-h-screen bg-[radial-gradient(circle_at_top,_rgba(244,239,230,0.95),_rgba(232,226,214,0.88),_rgba(210,220,206,0.84))] text-dusk">
      <header className="mx-auto flex max-w-6xl items-center justify-between px-6 py-6">
        <Link href="/" className="font-display text-2xl font-semibold text-pine">
          {messages.brand}
        </Link>
        <nav className="flex items-center gap-3 text-sm font-medium">
          <Link href="/lookup" className="rounded-full border border-pine/20 px-4 py-2 hover:bg-white/70">
            Lookup
          </Link>
          <Link href="/auth" className="rounded-full bg-pine px-4 py-2 text-white">
            Register
          </Link>
        </nav>
      </header>
      <main className={compact ? "mx-auto max-w-3xl px-6 pb-16" : "mx-auto max-w-6xl px-6 pb-16"}>
        {children}
      </main>
    </div>
  );
}
