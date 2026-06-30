// Indian festival calendar for the home festive banner. Windows are
// month/day ranges (year-agnostic) so the banner recurs every year without a
// code change. Lunar festivals shift a few days yearly — these windows are
// deliberately wide (lead-up + the day) and are easy to nudge if you want them
// pinned to exact dates for a given year.

export type Festival = {
  id: string;
  emoji: string;
  eyebrow: string;
  title: string;
  sub: string;
  cta: { label: string; href: string };
  // inclusive window, [MM, DD]
  start: [number, number];
  end: [number, number];
  // dark celebratory band; light text sits on all of these
  gradient: string;
  glow: string;
};

export const FESTIVALS: Festival[] = [
  {
    id: "makar-sankranti",
    emoji: "🪁",
    eyebrow: "Makar Sankranti",
    title: "Kite-season treats",
    sub: "Til-gur, peanuts and everything for the rooftop.",
    cta: { label: "Shop sweets", href: "/c/snacks" },
    start: [1, 10], end: [1, 16],
    gradient: "linear-gradient(135deg, oklch(0.42 0.12 60) 0%, oklch(0.50 0.15 75) 100%)",
    glow: "oklch(0.84 0.16 80)",
  },
  {
    id: "holi",
    emoji: "🎨",
    eyebrow: "Holi hai!",
    title: "Colour, gujiya & thandai",
    sub: "Sweets, dry fruits and cold-drink mixers for the celebration.",
    cta: { label: "Shop Holi picks", href: "/c/snacks" },
    start: [2, 28], end: [3, 6],
    gradient: "linear-gradient(135deg, oklch(0.45 0.16 350) 0%, oklch(0.50 0.16 30) 50%, oklch(0.52 0.15 130) 100%)",
    glow: "oklch(0.80 0.18 350)",
  },
  {
    id: "eid",
    emoji: "🌙",
    eyebrow: "Eid Mubarak",
    title: "Sewai, dates & dry fruits",
    sub: "Everything for the feast and the gifting tray.",
    cta: { label: "Shop Eid picks", href: "/c/staples" },
    start: [3, 16], end: [3, 22],
    gradient: "linear-gradient(135deg, oklch(0.32 0.07 175) 0%, oklch(0.40 0.10 195) 100%)",
    glow: "oklch(0.82 0.13 195)",
  },
  {
    id: "raksha-bandhan",
    emoji: "🪢",
    eyebrow: "Raksha Bandhan",
    title: "Sweeten the bond",
    sub: "Mithai and gifting hampers for your sibling.",
    cta: { label: "Shop gifting", href: "/c/snacks" },
    start: [8, 22], end: [8, 29],
    gradient: "linear-gradient(135deg, oklch(0.42 0.14 25) 0%, oklch(0.48 0.15 45) 100%)",
    glow: "oklch(0.83 0.16 50)",
  },
  {
    id: "ganesh-chaturthi",
    emoji: "🐘",
    eyebrow: "Ganesh Chaturthi",
    title: "Modak & pooja staples",
    sub: "Sweets, ghee and everything for the celebration at home.",
    cta: { label: "Shop the festival", href: "/c/dairy" },
    start: [9, 10], end: [9, 17],
    gradient: "linear-gradient(135deg, oklch(0.42 0.13 35) 0%, oklch(0.48 0.14 60) 100%)",
    glow: "oklch(0.84 0.15 65)",
  },
  {
    id: "navratri",
    emoji: "🌺",
    eyebrow: "Navratri & Dussehra",
    title: "Vrat-friendly picks",
    sub: "Fasting staples, fruit and dry fruits for all nine nights.",
    cta: { label: "Shop staples", href: "/c/staples" },
    start: [10, 5], end: [10, 22],
    gradient: "linear-gradient(135deg, oklch(0.40 0.15 350) 0%, oklch(0.46 0.16 20) 100%)",
    glow: "oklch(0.82 0.17 10)",
  },
  {
    id: "diwali",
    emoji: "🪔",
    eyebrow: "Diwali Dhamaka",
    title: "Lights, sweets & gifting",
    sub: "Stock the festival table and sort every gift — up to 30% off.",
    cta: { label: "Shop the festival", href: "/c/snacks" },
    start: [10, 28], end: [11, 10],
    gradient: "linear-gradient(135deg, oklch(0.30 0.08 320) 0%, oklch(0.40 0.13 35) 55%, oklch(0.50 0.16 70) 100%)",
    glow: "oklch(0.85 0.17 75)",
  },
  {
    id: "christmas",
    emoji: "🎄",
    eyebrow: "Christmas treats",
    title: "Cakes, cocoa & baking",
    sub: "Festive baking essentials and treats for the table.",
    cta: { label: "Shop Christmas", href: "/c/staples" },
    start: [12, 18], end: [12, 26],
    gradient: "linear-gradient(135deg, oklch(0.34 0.10 150) 0%, oklch(0.40 0.14 25) 100%)",
    glow: "oklch(0.82 0.16 30)",
  },
];

function within(now: Date, start: [number, number], end: [number, number]): boolean {
  const md = (now.getMonth() + 1) * 100 + now.getDate();
  const s = start[0] * 100 + start[1];
  const e = end[0] * 100 + end[1];
  return s <= e ? md >= s && md <= e : md >= s || md <= e; // handle year-wrap
}

// activeFestival returns the festival to show now, or a preview override by id.
export function activeFestival(now: Date, previewId?: string): Festival | null {
  if (previewId) return FESTIVALS.find((f) => f.id === previewId) ?? null;
  return FESTIVALS.find((f) => within(now, f.start, f.end)) ?? null;
}
