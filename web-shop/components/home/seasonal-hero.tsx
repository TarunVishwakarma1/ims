import Link from "next/link";
import { CloudRain, Sun, Sparkles, Snowflake, ArrowRight } from "lucide-react";
import { shopHref } from "@/lib/shop-path";

// A date-driven seasonal band — the home page's signature element. The season
// is derived from the month so the hero stays timely without a CMS edit, and
// every quick-pick chip routes to a real product search.
type Season = {
  id: string;
  eyebrow: string;
  title: string;
  sub: string;
  rain?: boolean;
  // gradient + glow tuned per season; text stays light on all of them.
  gradient: string;
  glow: string;
  Icon: typeof CloudRain;
  chips: { label: string; slug: string }[];
};

function seasonFor(month: number): Season {
  // Northern-India calendar. month is 1–12.
  if (month >= 6 && month <= 9) {
    return {
      id: "monsoon",
      eyebrow: "Monsoon kitchen",
      title: "Rainy-day cravings, delivered",
      sub: "Chai, crispy pakora kits and piping-hot soups — before the next downpour.",
      rain: true,
      gradient: "linear-gradient(135deg, oklch(0.30 0.06 175) 0%, oklch(0.34 0.09 160) 55%, oklch(0.42 0.13 150) 100%)",
      glow: "oklch(0.80 0.13 150)",
      Icon: CloudRain,
      chips: [
        { label: "Chai & coffee", slug: "beverages" },
        { label: "Hot snacks", slug: "snacks" },
        { label: "Cooking staples", slug: "staples" },
      ],
    };
  }
  if (month >= 3 && month <= 5) {
    return {
      id: "summer",
      eyebrow: "Beat the heat",
      title: "Cool down, stock up",
      sub: "Chilled drinks, ice cream and juicy seasonal fruit — kept cold to your door.",
      gradient: "linear-gradient(135deg, oklch(0.40 0.10 230) 0%, oklch(0.46 0.12 200) 55%, oklch(0.52 0.14 170) 100%)",
      glow: "oklch(0.82 0.14 200)",
      Icon: Sun,
      chips: [
        { label: "Cool drinks", slug: "beverages" },
        { label: "Ice cream & dairy", slug: "dairy" },
        { label: "Light snacks", slug: "snacks" },
      ],
    };
  }
  if (month >= 10 && month <= 11) {
    return {
      id: "festive",
      eyebrow: "Festive feasts",
      title: "Everything for the celebration",
      sub: "Sweets, dry fruits and ghee for the festival table — and gifting sorted.",
      gradient: "linear-gradient(135deg, oklch(0.34 0.09 40) 0%, oklch(0.42 0.13 55) 55%, oklch(0.50 0.15 75) 100%)",
      glow: "oklch(0.82 0.16 75)",
      Icon: Sparkles,
      chips: [
        { label: "Sweets & snacks", slug: "snacks" },
        { label: "Ghee & dairy", slug: "dairy" },
        { label: "Grains & dry fruit", slug: "staples" },
      ],
    };
  }
  return {
    id: "winter",
    eyebrow: "Winter warmers",
    title: "Cosy up, kitchen stocked",
    sub: "Soups, dry fruits and warming spices for the cold evenings ahead.",
    gradient: "linear-gradient(135deg, oklch(0.30 0.06 265) 0%, oklch(0.36 0.08 250) 55%, oklch(0.44 0.10 230) 100%)",
    glow: "oklch(0.80 0.12 250)",
    Icon: Snowflake,
    chips: [
      { label: "Hot drinks", slug: "beverages" },
      { label: "Pantry staples", slug: "staples" },
      { label: "Namkeen", slug: "snacks" },
    ],
  };
}

export function SeasonalHero({ shopSlug }: { shopSlug?: string }) {
  const s = seasonFor(new Date().getMonth() + 1);
  const { Icon } = s;

  return (
    <section
      aria-label={`${s.eyebrow} — seasonal picks`}
      className="relative isolate overflow-hidden rounded-2xl px-6 py-8 sm:px-10 sm:py-12 text-white"
      style={{ background: s.gradient }}
    >
      {/* soft glow + (optional) drifting rain, purely decorative */}
      <div
        aria-hidden
        className="pointer-events-none absolute -right-16 -top-20 size-72 rounded-full blur-3xl opacity-40"
        style={{ background: s.glow }}
      />
      {s.rain && <div aria-hidden className="season-rain pointer-events-none absolute inset-0 opacity-60" />}

      <div className="relative grid gap-6 sm:grid-cols-[1fr_auto] sm:items-center">
        <div className="max-w-xl space-y-4">
          <span className="inline-flex items-center gap-1.5 rounded-full bg-white/15 px-3 py-1 text-xs font-medium tracking-wide uppercase backdrop-blur">
            <Icon className="size-3.5" /> {s.eyebrow}
          </span>
          <h1 className="font-display text-3xl sm:text-4xl font-semibold tracking-tight text-balance">
            {s.title}
          </h1>
          <p className="text-sm sm:text-base text-white/80 text-pretty">{s.sub}</p>
          <div className="flex flex-wrap gap-2 pt-1">
            {s.chips.map((c) => (
              <Link
                key={c.slug}
                href={shopHref(shopSlug, `/c/${c.slug}`)}
                className="group inline-flex items-center gap-1.5 rounded-full bg-white/95 px-4 py-2 text-sm font-medium text-neutral-900 shadow-sm transition hover:bg-white"
              >
                {c.label}
                <ArrowRight className="size-3.5 transition-transform group-hover:translate-x-0.5" />
              </Link>
            ))}
          </div>
        </div>

        {/* oversized season glyph — the visual anchor on wider screens */}
        <Icon
          aria-hidden
          className="hidden sm:block size-32 lg:size-40 text-white/15 shrink-0 self-center"
          strokeWidth={1.25}
        />
      </div>
    </section>
  );
}
