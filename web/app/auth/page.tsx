import { AuthForm } from "@/components/auth-form";
import { SiteShell } from "@/components/site-shell";

export default function AuthPage() {
  return (
    <SiteShell compact>
      <section className="py-10">
        <h1 className="font-display text-5xl text-pine">Sign in or register</h1>
        <p className="mt-4 max-w-2xl text-dusk/70">
          OpenChip uses single-use magic links instead of passwords. New owners are created automatically on first sign-in.
        </p>
        <div className="mt-8">
          <AuthForm />
        </div>
      </section>
    </SiteShell>
  );
}
