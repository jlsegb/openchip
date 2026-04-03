"use client";

import { useState } from "react";

import { DashboardShell } from "@/components/dashboard-shell";
import { apiFetch } from "@/lib/api";
import { getToken } from "@/lib/session";

export function ExportPanel() {
  const [status, setStatus] = useState("");

  async function exportData() {
    try {
      const token = getToken();
      const data = await apiFetch("/export", undefined, token);
      const blob = new Blob([JSON.stringify(data, null, 2)], { type: "application/json" });
      const url = URL.createObjectURL(blob);
      const anchor = document.createElement("a");
      anchor.href = url;
      anchor.download = "openchip-export.json";
      anchor.click();
      URL.revokeObjectURL(url);
      setStatus("Export downloaded.");
    } catch (err) {
      setStatus(err instanceof Error ? err.message : "Export failed");
    }
  }

  return (
    <DashboardShell>
      <div className="rounded-[2rem] bg-white/8 p-8">
        <h1 className="font-display text-4xl">Export your data</h1>
        <p className="mt-4 max-w-2xl text-white/70">
          Download your owner profile, pets, and associated lookup events as JSON.
        </p>
        <button onClick={exportData} className="mt-6 rounded-full bg-sand px-5 py-3 font-semibold text-dusk">
          Download export
        </button>
        {status ? <p className="mt-3 text-sm text-white/80">{status}</p> : null}
      </div>
    </DashboardShell>
  );
}
