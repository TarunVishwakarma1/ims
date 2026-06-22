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

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (!emblaApi) return;
      if (e.key === "ArrowLeft") {
        e.preventDefault();
        emblaApi.scrollPrev();
      } else if (e.key === "ArrowRight") {
        e.preventDefault();
        emblaApi.scrollNext();
      }
    },
    [emblaApi],
  );

  const total = banners.length;

  return (
    <section
      aria-roledescription="carousel"
      aria-label="Featured banners"
      onKeyDown={onKeyDown}
    >
      <div ref={emblaRef} className="overflow-hidden rounded-2xl" tabIndex={0}>
        <div className="flex">
          {banners.map((b, i) => (
            <Link
              key={b.id}
              href={safeCtaLink(b.cta_link)}
              role="group"
              aria-roledescription="slide"
              aria-label={`${i + 1} of ${total}`}
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
        {banners.map((b, i) => (
          <button
            key={b.id}
            type="button"
            aria-label={`Go to slide ${i + 1} of ${total}`}
            aria-current={i === index ? "true" : undefined}
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
