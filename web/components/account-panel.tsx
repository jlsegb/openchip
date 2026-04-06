"use client";

import { useEffect, useState } from "react";

import { DashboardShell } from "@/components/dashboard-shell";
import { apiFetch } from "@/lib/api";

export function AccountPanel() {
  const [profile, setProfile] = useState<any>(null);
  const [status, setStatus] = useState("");

  useEffect(() => {
    apiFetch("/auth/me").then(setProfile).catch((err) => {
      const message = err instanceof Error ? err.message : "Could not load profile";
      if (message.toLowerCase().includes("unauthorized")) {
        window.location.href = "/auth";
        return;
      }
      setStatus(message);
    });
  }, []);

  async function deleteAccount() {
    await apiFetch("/account", { method: "DELETE" });
    await apiFetch("/auth/logout", { method: "POST" });
    window.location.href = "/";
  }

  async function saveProfile(event: React.FormEvent) {
    event.preventDefault();
    const updated = await apiFetch(
      "/auth/me",
      {
        method: "PUT",
        body: JSON.stringify({
          name: profile?.name ?? "",
          phone: profile?.phone || null
        })
      }
    );
    setProfile(updated);
    setStatus("Profile updated.");
  }

  return (
    <DashboardShell>
      <div className="grid gap-6 lg:grid-cols-[1fr_0.8fr]">
        <form onSubmit={saveProfile} className="rounded-[2rem] bg-white/8 p-8">
          <h1 className="font-display text-4xl">Account</h1>
          {profile ? (
            <div className="mt-6 space-y-4 text-white/80">
              <div>
                <p className="mb-2 text-sm uppercase tracking-[0.18em] text-white/50">Name</p>
                <input
                  value={profile.name ?? ""}
                  onChange={(event) => setProfile((current: any) => ({ ...current, name: event.target.value }))}
                  className="w-full rounded-2xl border border-white/10 bg-white/10 px-4 py-3"
                />
              </div>
              <div>
                <p className="mb-2 text-sm uppercase tracking-[0.18em] text-white/50">Email</p>
                <input value={profile.email ?? ""} disabled className="w-full rounded-2xl border border-white/10 bg-white/5 px-4 py-3 text-white/55" />
              </div>
              <div>
                <p className="mb-2 text-sm uppercase tracking-[0.18em] text-white/50">Phone</p>
                <input
                  value={profile.phone ?? ""}
                  onChange={(event) => setProfile((current: any) => ({ ...current, phone: event.target.value }))}
                  className="w-full rounded-2xl border border-white/10 bg-white/10 px-4 py-3"
                />
              </div>
            </div>
          ) : null}
          <button className="mt-6 rounded-full bg-sand px-5 py-3 font-semibold text-dusk">
            Save profile
          </button>
          {status ? <p className="mt-3 text-sm text-white/80">{status}</p> : null}
        </form>
        <div className="rounded-[2rem] bg-[#33211f] p-8">
          <h2 className="font-display text-3xl">Delete account</h2>
          <p className="mt-4 text-white/70">
            This anonymizes your owner record and deactivates your registrations while preserving registry integrity.
          </p>
          <button onClick={deleteAccount} className="mt-6 rounded-full bg-[#ffc4b7] px-5 py-3 font-semibold text-[#331814]">
            Delete and anonymize
          </button>
        </div>
      </div>
    </DashboardShell>
  );
}
