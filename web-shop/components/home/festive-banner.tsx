import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { activeFestival } from "@/lib/festivals";

// Date-driven festival band. Renders only inside a festival's window, otherwise
// nothing (the page falls back to the seasonal hero). Pass `preview` (a festival
// id, e.g. from ?festival=diwali) to force a specific banner for previewing.
export function FestiveBanner({ preview }: { preview?: string }) {
  const f = activeFestival(new Date(), preview);
  if (!f) return null;

  return (
    <section
      aria-label={`${f.eyebrow} — festival offers`}
      className="relative isolate overflow-hidden rounded-2xl px-6 py-8 sm:px-10 sm:py-12 text-white"
      style={{ background: f.gradient }}
    >
      <div
        aria-hidden
        className="pointer-events-none absolute -right-12 -top-16 size-72 rounded-full blur-3xl opacity-45"
        style={{ background: f.glow }}
      />
      {/* festive twinkle: faint dotted overlay */}
      <div
        aria-hidden
        className="pointer-events-none absolute inset-0 opacity-25"
        style={{
          backgroundImage: "radial-gradient(circle, rgba(255,255,255,0.6) 1px, transparent 1.5px)",
          backgroundSize: "26px 26px",
        }}
      />

      <div className="relative grid gap-6 sm:grid-cols-[1fr_auto] sm:items-center">
        <div className="max-w-xl space-y-4">
          <span className="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3 py-1 text-xs font-medium tracking-wide uppercase backdrop-blur">
            <span aria-hidden>{f.emoji}</span> {f.eyebrow}
          </span>
          <h1 className="font-display text-3xl sm:text-4xl font-semibold tracking-tight text-balance">
            {f.title}
          </h1>
          <p className="text-sm sm:text-base text-white/80 text-pretty">{f.sub}</p>
          <Link
            href={f.cta.href}
            className="group inline-flex items-center gap-1.5 rounded-full bg-white/95 px-5 py-2.5 text-sm font-semibold text-neutral-900 shadow-sm transition hover:bg-white"
          >
            {f.cta.label}
            <ArrowRight className="size-4 transition-transform group-hover:translate-x-0.5" />
          </Link>
        </div>

        <span aria-hidden className="hidden sm:block text-7xl lg:text-8xl drop-shadow-lg self-center">
          {f.emoji}
        </span>
      </div>
    </section>
  );
}
