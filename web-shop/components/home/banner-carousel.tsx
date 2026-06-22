"use client";
import { useCallback, useEffect, useState } from "react";
import useEmblaCarousel from "embla-carousel-react";
import Autoplay from "embla-carousel-autoplay";
import Image from "next/image";
import Link from "next/link";
import { cn } from "@/lib/utils";
import { safeCtaLink } from "@/lib/safe-next";
import type { Banner } from "@/lib/shop-types";

export function BannerCarousel({ banners }: { banners: Banner[] }) {
  const [emblaRef, emblaApi] = useEmblaCarousel(
    { loop: true, align: "start" },
    [Autoplay({ delay: 5000, stopOnInteraction: false, stopOnMouseEnter: true })],
  );
  const [index, setIndex] = useState(0);

  useEffect(() => {
    if (!emblaApi) return;
    const onSelect = () => setIndex(emblaApi.selectedScrollSnap());
    emblaApi.on("select", onSelect);
    onSelect();
    return () => {
      emblaApi.off("select", onSelect);
    };
  }, [emblaApi]);

  const scrollTo = useCallback((i: number) => emblaApi?.scrollTo(i), [emblaApi]);

  return (
    <section>
      <div ref={emblaRef} className="overflow-hidden rounded-2xl">
        <div className="flex">
          {banners.map((b) => (
            <Link
              key={b.id}
              href={safeCtaLink(b.cta_link)}
              className="relative flex-[0_0_100%] aspect-[3/1] bg-brand-50"
            >
              {b.image_url && (
                <Image
                  src={b.image_url}
                  alt={b.title}
                  fill
                  sizes="(max-width: 1280px) 100vw, 1280px"
                  className="object-cover"
                />
              )}
              <div className="absolute inset-0 bg-gradient-to-r from-black/40 to-transparent flex flex-col justify-center p-6 text-white">
                <h3 className="text-2xl md:text-3xl font-semibold">{b.title}</h3>
                {b.subtitle && <p className="text-sm mt-1">{b.subtitle}</p>}
              </div>
            </Link>
          ))}
        </div>
      </div>

      <div className="flex gap-2 justify-center mt-3">
        {banners.map((_, i) => (
          <button
            key={i}
            type="button"
            aria-label={`Slide ${i + 1}`}
            onClick={() => scrollTo(i)}
            className={cn(
              "h-2 rounded-full transition-all",
              i === index ? "w-6 bg-brand-600" : "w-2 bg-border",
            )}
          />
        ))}
      </div>
    </section>
  );
}
