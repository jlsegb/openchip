import { LookupForm } from "@/components/lookup-form";
import { SiteShell } from "@/components/site-shell";

export default function LookupPage() {
  return (
    <SiteShell compact>
      <section className="py-10">
        <h1 className="font-display text-5xl text-pine">Lookup a microchip</h1>
        <p className="mt-4 max-w-2xl text-dusk/70">
          Search by raw or normalized chip ID. OpenChip will still show the manufacturer hint even when no registry match is found.
        </p>
        <div className="mt-8">
          <LookupForm />
        </div>
      </section>
    </SiteShell>
  );
}
