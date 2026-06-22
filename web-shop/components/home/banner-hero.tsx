import Image from "next/image";
import Link from "next/link";
import type { Banner } from "@/lib/shop-types";

export function BannerHero({ banner }: { banner: Banner }) {
  return (
    <Link
      href={banner.cta_link || "#"}
      className="block relative w-full aspect-[3/1] rounded-2xl overflow-hidden bg-brand-50"
    >
      {banner.image_url && (
        <Image
          src={banner.image_url}
          alt={banner.title}
          fill
          priority
          sizes="(max-width: 1280px) 100vw, 1280px"
          className="object-cover"
        />
      )}
      <div className="absolute inset-0 bg-gradient-to-r from-black/40 to-transparent flex flex-col justify-center p-8 text-white">
        <h2 className="text-3xl md:text-5xl font-semibold">{banner.title}</h2>
        {banner.subtitle && <p className="mt-2 text-lg">{banner.subtitle}</p>}
        {banner.cta_label && (
          <span className="mt-4 inline-block bg-brand-500 text-white px-6 py-2 rounded-xl w-fit">
            {banner.cta_label}
          </span>
        )}
      </div>
    </Link>
  );
}
