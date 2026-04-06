"use client";

import Link from "next/link";
import { ReactNode } from "react";

import { apiFetch } from "@/lib/api";

export function DashboardShell({ children }: { children: ReactNode }) {
  return (
    <div className="min-h-screen bg-dusk text-white">
      <header className="mx-auto flex max-w-6xl items-center justify-between px-6 py-6">
        <Link href="/dashboard" className="font-display text-2xl">
          OpenChip
        </Link>
        <div className="flex items-center gap-3 text-sm">
          <Link href="/dashboard/export">Export</Link>
          <Link href="/dashboard/account">Account</Link>
          <button
            onClick={async () => {
              await apiFetch("/auth/logout", { method: "POST" });
              window.location.href = "/auth";
            }}
            className="rounded-full border border-white/20 px-3 py-2"
          >
            Sign out
          </button>
        </div>
      </header>
      <main className="mx-auto max-w-6xl px-6 pb-16">{children}</main>
    </div>
  );
}
