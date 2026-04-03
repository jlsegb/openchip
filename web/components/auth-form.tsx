"use client";

import { useState } from "react";

import { apiFetch } from "@/lib/api";

export function AuthForm() {
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [status, setStatus] = useState("");
  const [error, setError] = useState("");

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    setStatus("");
    try {
      await apiFetch("/auth/magic-link", {
        method: "POST",
        body: JSON.stringify({ email, name })
      });
      setStatus("Check your inbox for a sign-in link.");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Unable to send magic link");
    }
  }

  return (
    <form onSubmit={onSubmit} className="space-y-4 rounded-[2rem] border border-black/5 bg-white/80 p-8 shadow-lg">
      <input
        value={name}
        onChange={(event) => setName(event.target.value)}
        placeholder="Your name"
        className="w-full rounded-2xl border border-moss/20 bg-sand px-4 py-3"
      />
      <input
        value={email}
        onChange={(event) => setEmail(event.target.value)}
        placeholder="Email address"
        type="email"
        className="w-full rounded-2xl border border-moss/20 bg-sand px-4 py-3"
      />
      <button className="w-full rounded-full bg-pine px-5 py-3 font-semibold text-white">
        Email me a magic link
      </button>
      {status ? <p className="text-sm text-moss">{status}</p> : null}
      {error ? <p className="text-sm text-ember">{error}</p> : null}
    </form>
  );
}
