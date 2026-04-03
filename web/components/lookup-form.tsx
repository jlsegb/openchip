"use client";

import { useState } from "react";

import { apiFetch } from "@/lib/api";

type LookupResponse = {
  found: boolean;
  manufacturer_hint: string;
  registrations: Array<{
    pet_name: string;
    species: string;
    owner_first_name: string;
    manufacturer_hint: string;
  }>;
};

export function LookupForm() {
  const [chipId, setChipId] = useState("");
  const [data, setData] = useState<LookupResponse | null>(null);
  const [error, setError] = useState("");
  const [contactStatus, setContactStatus] = useState("");

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    setContactStatus("");
    try {
      const result = await apiFetch<LookupResponse>(`/lookup/${encodeURIComponent(chipId)}`);
      setData(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Lookup failed");
    }
  }

  async function contactOwner() {
    try {
      await apiFetch(`/lookup/${encodeURIComponent(chipId)}/contact`, { method: "POST" });
      setContactStatus("Owner notification sent.");
    } catch (err) {
      setContactStatus(err instanceof Error ? err.message : "Could not notify owner.");
    }
  }

  return (
    <div className="rounded-[2rem] border border-black/5 bg-white/75 p-8 shadow-[0_24px_80px_rgba(18,53,36,0.12)] backdrop-blur">
      <form onSubmit={onSubmit} className="flex flex-col gap-4">
        <label className="text-sm font-semibold uppercase tracking-[0.2em] text-moss">
          Microchip ID
        </label>
        <div className="flex flex-col gap-3 md:flex-row">
          <input
            value={chipId}
            onChange={(event) => setChipId(event.target.value)}
            placeholder="Enter 9-digit, 10-hex, or 15-digit chip ID"
            className="flex-1 rounded-full border border-moss/20 bg-sand px-5 py-4 outline-none ring-0 transition focus:border-pine"
          />
          <button className="rounded-full bg-ember px-6 py-4 font-semibold text-white transition hover:opacity-90">
            Search registry
          </button>
        </div>
      </form>

      {error ? <p className="mt-4 text-sm text-ember">{error}</p> : null}

      {data ? (
        <div className="mt-8 space-y-4 rounded-[1.5rem] bg-sand/80 p-6">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="text-sm uppercase tracking-[0.2em] text-moss">Manufacturer</p>
              <p className="text-lg font-semibold text-pine">{data.manufacturer_hint}</p>
            </div>
            <p className="text-sm text-dusk/70">
              {data.found ? `${data.registrations.length} registration(s) found` : "No active registrations found"}
            </p>
          </div>

          {data.registrations.map((item) => (
            <div key={`${item.pet_name}-${item.owner_first_name}`} className="rounded-3xl border border-black/5 bg-white p-5">
              <p className="font-semibold text-pine">{item.pet_name}</p>
              <p className="text-sm text-dusk/70">
                {item.species} • Owner: {item.owner_first_name}
              </p>
            </div>
          ))}

          {data.found ? (
            <div className="flex flex-col gap-3 pt-2">
              <button
                onClick={contactOwner}
                className="w-full rounded-full border border-pine bg-pine px-5 py-3 font-semibold text-white md:w-auto"
              >
                Contact owner
              </button>
              {contactStatus ? <p className="text-sm text-moss">{contactStatus}</p> : null}
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
