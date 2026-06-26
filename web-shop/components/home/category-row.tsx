import Link from "next/link";
import Image from "next/image";
import type { Category } from "@/lib/shop-types";

export function CategoryRow({ categories }: { categories: Category[] }) {
  return (
    <section>
      <h2 className="text-xl font-semibold mb-4">Shop by category</h2>
      <ul className="flex gap-3 overflow-x-auto pb-2 -mx-4 px-4 scrollbar-hidden">
        {categories.map((c) => (
          <li key={c.id} className="flex-shrink-0">
            <Link
              href={`/c/${c.slug}`}
              className="flex flex-col items-center gap-2 w-24 rounded-xl bg-brand-50 p-3 hover:bg-brand-100 transition-colors"
            >
              <div className="relative size-14 rounded-full overflow-hidden bg-bg">
                {c.icon_url ? (
                  <Image
                    src={c.icon_url}
                    alt={c.name}
                    fill
                    sizes="56px"
                    className="object-cover"
                  />
                ) : (
                  <div className="size-full bg-brand-100" />
                )}
              </div>
              <span className="text-xs font-medium text-center line-clamp-2">
                {c.name}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </section>
  );
}
