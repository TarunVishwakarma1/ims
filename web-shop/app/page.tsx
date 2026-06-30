import { redirect } from "next/navigation";

// The storefront is per-shop now: every catalog page lives under /s/<shop>.
// The root sends shoppers to the directory to pick a shop first.
export default function RootPage() {
  redirect("/shops");
}
