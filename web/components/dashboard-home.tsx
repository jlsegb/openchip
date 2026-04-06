"use client";

import Link from "next/link";
import { useEffect, useState } from "react";

import { DashboardShell } from "@/components/dashboard-shell";
import { apiFetch } from "@/lib/api";

type Pet = {
  id: string;
  pet_name: string;
  species: string;
  chip_id_normalized: string;
  manufacturer_hint: string;
};

export function DashboardHome() {
  const [pets, setPets] = useState<Pet[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    apiFetch<Pet[]>("/pets").then(setPets).catch((err) => {
      const message = err instanceof Error ? err.message : "Unable to load pets";
      if (message.toLowerCase().includes("unauthorized")) {
        window.location.href = "/auth";
        return;
      }
      setError(message);
    });
  }, []);

  return (
    <DashboardShell>
      <section className="grid gap-8 lg:grid-cols-[1.4fr_0.8fr]">
        <div className="rounded-[2rem] bg-white/8 p-8">
          <div className="mb-6 flex items-center justify-between">
            <div>
              <p className="text-sm uppercase tracking-[0.2em] text-white/60">Owner Portal</p>
              <h1 className="font-display text-4xl">Your registered pets</h1>
            </div>
            <Link href="/dashboard/pets/new" className="rounded-full bg-sand px-4 py-3 font-semibold text-dusk">
              Add pet
            </Link>
          </div>

          <div className="space-y-4">
            {pets.map((pet) => (
              <Link key={pet.id} href={`/dashboard/pets/${pet.id}`} className="block rounded-3xl bg-white/10 p-5 transition hover:bg-white/15">
                <p className="text-xl font-semibold">{pet.pet_name}</p>
                <p className="text-sm text-white/70">
                  {pet.species} • {pet.chip_id_normalized} • {pet.manufacturer_hint}
                </p>
              </Link>
            ))}
            {pets.length === 0 ? <p className="text-white/70">No active pets yet.</p> : null}
            {error ? <p className="text-sm text-[#ffc4b7]">{error}</p> : null}
          </div>
        </div>

        <aside className="space-y-4 rounded-[2rem] bg-[#213328] p-8">
          <h2 className="font-display text-3xl">Why OpenChip exists</h2>
          <p className="text-white/75">
            Reunification data should be available, self-hostable, and free for the public good.
          </p>
          <Link href="/lookup" className="rounded-full border border-white/15 px-4 py-3 text-center">
            Public lookup
          </Link>
        </aside>
      </section>
    </DashboardShell>
  );
}
