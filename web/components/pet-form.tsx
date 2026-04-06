"use client";

import { useEffect, useState } from "react";

import { DashboardShell } from "@/components/dashboard-shell";
import { apiFetch } from "@/lib/api";

type Mode = "create" | "edit";

export function PetForm({ mode, petId }: { mode: Mode; petId?: string }) {
  const [form, setForm] = useState({
    chip_id: "",
    pet_name: "",
    species: "dog",
    breed: "",
    color: "",
    date_of_birth: "",
    notes: ""
  });
  const [status, setStatus] = useState("");
  const [lookupHistory, setLookupHistory] = useState<Array<{ id: string; looked_up_by_agent?: string; created_at: string }>>([]);

  useEffect(() => {
    if (mode === "edit" && petId) {
      apiFetch<any>(`/pets/${petId}`).then((pet) => {
        setForm({
          chip_id: pet.chip_id_raw,
          pet_name: pet.pet_name,
          species: pet.species,
          breed: pet.breed ?? "",
          color: pet.color ?? "",
          date_of_birth: pet.date_of_birth?.slice(0, 10) ?? "",
          notes: pet.notes ?? ""
        });
      }).catch((err) => {
        const message = err instanceof Error ? err.message : "Could not load pet";
        if (message.toLowerCase().includes("unauthorized")) {
          window.location.href = "/auth";
        } else {
          setStatus(message);
        }
      });
      apiFetch<any[]>(`/pets/${petId}/lookups`).then(setLookupHistory).catch(() => undefined);
    } else {
      apiFetch<any[]>("/pets").catch((err) => {
        const message = err instanceof Error ? err.message : "Unauthorized";
        if (message.toLowerCase().includes("unauthorized")) {
          window.location.href = "/auth";
        }
      });
    }
  }, [mode, petId]);

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    try {
      const method = mode === "create" ? "POST" : "PUT";
      const path = mode === "create" ? "/pets" : `/pets/${petId}`;
      await apiFetch(path, {
        method,
        body: JSON.stringify({
          ...form,
          breed: form.breed || null,
          color: form.color || null,
          date_of_birth: form.date_of_birth || null,
          notes: form.notes || null
        })
      });
      setStatus(mode === "create" ? "Pet registered." : "Pet updated.");
      if (mode === "create") {
        window.location.href = "/dashboard";
      }
    } catch (err) {
      setStatus(err instanceof Error ? err.message : "Could not save pet");
    }
  }

  async function deletePet() {
    if (!petId) return;
    await apiFetch(`/pets/${petId}`, { method: "DELETE" });
    window.location.href = "/dashboard";
  }

  async function transferPet() {
    const email = window.prompt("New owner email");
    if (!petId || !email) return;
    await apiFetch(`/pets/${petId}/transfer`, {
      method: "POST",
      body: JSON.stringify({ to_email: email })
    });
    setStatus("Transfer initiated.");
  }

  return (
    <DashboardShell>
      <div className="grid gap-8 lg:grid-cols-[1.1fr_0.9fr]">
        <form onSubmit={onSubmit} className="space-y-4 rounded-[2rem] bg-white/8 p-8">
          <h1 className="font-display text-4xl">{mode === "create" ? "Register a pet" : "Edit pet"}</h1>
          {[
            ["chip_id", "Chip ID"],
            ["pet_name", "Pet name"],
            ["breed", "Breed"],
            ["color", "Color"],
            ["date_of_birth", "Date of birth"],
            ["notes", "Notes"]
          ].map(([key, label]) => (
            <input
              key={key}
              value={form[key as keyof typeof form]}
              onChange={(event) => setForm((current) => ({ ...current, [key]: event.target.value }))}
              placeholder={label}
              type={key === "date_of_birth" ? "date" : "text"}
              className="w-full rounded-2xl border border-white/10 bg-white/10 px-4 py-3"
            />
          ))}
          <select
            value={form.species}
            onChange={(event) => setForm((current) => ({ ...current, species: event.target.value }))}
            className="w-full rounded-2xl border border-white/10 bg-white/10 px-4 py-3"
          >
            <option value="dog">Dog</option>
            <option value="cat">Cat</option>
            <option value="other">Other</option>
          </select>
          <button className="rounded-full bg-sand px-5 py-3 font-semibold text-dusk">
            {mode === "create" ? "Save registration" : "Save changes"}
          </button>
          {status ? <p className="text-sm text-white/80">{status}</p> : null}
          {mode === "edit" ? (
            <div className="flex flex-wrap gap-3 pt-4">
              <button type="button" onClick={transferPet} className="rounded-full border border-white/20 px-4 py-3">
                Transfer pet
              </button>
              <button type="button" onClick={deletePet} className="rounded-full border border-[#ffc4b7]/30 px-4 py-3 text-[#ffc4b7]">
                Deactivate registration
              </button>
            </div>
          ) : null}
        </form>

        <aside className="rounded-[2rem] bg-[#213328] p-8">
          <h2 className="font-display text-3xl">Lookup history</h2>
          <div className="mt-6 space-y-3">
            {lookupHistory.map((entry) => (
              <div key={entry.id} className="rounded-3xl bg-white/10 p-4">
                <p className="font-medium">{entry.looked_up_by_agent || "Public lookup"}</p>
                <p className="text-sm text-white/70">{new Date(entry.created_at).toLocaleString()}</p>
              </div>
            ))}
            {lookupHistory.length === 0 ? <p className="text-white/70">No lookup history yet.</p> : null}
          </div>
        </aside>
      </div>
    </DashboardShell>
  );
}
