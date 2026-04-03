import Link from "next/link";

import { SiteShell } from "@/components/site-shell";
import { messages } from "@/lib/messages";

export default function HomePage() {
  return (
    <SiteShell>
      <section className="grid items-center gap-12 py-10 lg:grid-cols-[1.2fr_0.8fr]">
        <div>
          <p className="mb-4 text-sm uppercase tracking-[0.28em] text-moss">Public-Good Registry</p>
          <h1 className="font-display text-6xl leading-none text-pine">{messages.brand}</h1>
          <p className="mt-6 max-w-2xl text-lg text-dusk/75">{messages.tagline}</p>
          <div className="mt-8 flex flex-wrap gap-4">
            <Link href="/lookup" className="rounded-full bg-ember px-6 py-4 font-semibold text-white">
              Look up a chip
            </Link>
            <Link href="/auth" className="rounded-full border border-pine/20 bg-white/70 px-6 py-4 font-semibold text-pine">
              Register your pet
            </Link>
          </div>
        </div>
        <div className="rounded-[2.5rem] border border-black/5 bg-white/75 p-8 shadow-[0_24px_80px_rgba(18,53,36,0.12)]">
          <div className="rounded-[2rem] bg-pine p-8 text-white">
            <p className="text-sm uppercase tracking-[0.24em] text-white/60">Designed for reunification</p>
            <ul className="mt-6 space-y-4 text-lg">
              <li>Free public lookup</li>
              <li>Magic-link owner sign in</li>
              <li>Self-hostable open source stack</li>
              <li>Shelter and vet API support</li>
            </ul>
          </div>
        </div>
      </section>
    </SiteShell>
  );
}
