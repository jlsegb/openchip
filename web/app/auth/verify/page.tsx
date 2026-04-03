"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";

import { SiteShell } from "@/components/site-shell";
import { apiFetch } from "@/lib/api";
import { setToken } from "@/lib/session";

export default function VerifyPage() {
  const params = useSearchParams();
  const [status, setStatus] = useState("Verifying your magic link...");

  useEffect(() => {
    const token = params.get("token");
    if (!token) {
      setStatus("Missing token.");
      return;
    }
    apiFetch<{ token: string }>("/auth/verify?token=" + encodeURIComponent(token))
      .then((data) => {
        setToken(data.token);
        setStatus("Signed in. Redirecting to dashboard...");
        window.location.href = "/dashboard";
      })
      .catch((err) => {
        setStatus(err instanceof Error ? err.message : "Verification failed.");
      });
  }, [params]);

  return (
    <SiteShell compact>
      <section className="py-20">
        <h1 className="font-display text-5xl text-pine">Verify sign-in</h1>
        <p className="mt-6 text-lg text-dusk/75">{status}</p>
      </section>
    </SiteShell>
  );
}
