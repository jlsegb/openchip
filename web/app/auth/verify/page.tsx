"use client";

import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";

import { SiteShell } from "@/components/site-shell";
import { apiFetch } from "@/lib/api";

function VerifyContent() {
  const params = useSearchParams();
  const [status, setStatus] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const token = params.get("token");

  async function confirmSignIn() {
    if (!token) {
      setStatus("Missing token.");
      return;
    }
    setIsSubmitting(true);
    setStatus("");
    try {
      await apiFetch<{ role: string }>("/auth/verify", {
        method: "POST",
        body: JSON.stringify({ token })
      });
      setStatus("Signed in. Redirecting to dashboard...");
      window.location.href = "/dashboard";
    } catch (err) {
      setStatus(err instanceof Error ? err.message : "Verification failed.");
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <section className="py-20">
      <h1 className="font-display text-5xl text-pine">Verify sign-in</h1>
      <p className="mt-6 max-w-2xl text-lg text-dusk/75">
        Confirm in the browser to finish sign-in. This avoids one-time magic links being consumed by automated scanners or prefetchers.
      </p>
      <button
        onClick={confirmSignIn}
        disabled={!token || isSubmitting}
        className="mt-8 rounded-full bg-pine px-5 py-3 font-semibold text-white disabled:cursor-not-allowed disabled:opacity-60"
      >
        {isSubmitting ? "Signing in..." : "Complete sign-in"}
      </button>
      {status ? <p className="mt-4 text-lg text-dusk/75">{status}</p> : null}
      {!token ? <p className="mt-4 text-lg text-[#8b4b3d]">Missing token.</p> : null}
    </section>
  );
}

export default function VerifyPage() {
  return (
    <SiteShell compact>
      <Suspense
        fallback={
          <section className="py-20">
            <h1 className="font-display text-5xl text-pine">Verify sign-in</h1>
            <p className="mt-6 text-lg text-dusk/75">Verifying your magic link...</p>
          </section>
        }
      >
        <VerifyContent />
      </Suspense>
    </SiteShell>
  );
}
