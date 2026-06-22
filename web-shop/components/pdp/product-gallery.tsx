"use client";
import { useCallback, useEffect, useState } from "react";
import useEmblaCarousel from "embla-carousel-react";
import Image from "next/image";
import { cn } from "@/lib/utils";

type Props = { images: string[]; alt: string };

export function ProductGallery({ images, alt }: Props) {
  const [mainRef, mainApi] = useEmblaCarousel({ loop: false });
  const [index, setIndex] = useState(0);

  useEffect(() => {
    if (!mainApi) return;
    const onSelect = () => setIndex(mainApi.selectedScrollSnap());
    mainApi.on("select", onSelect);
    onSelect();
    return () => {
      mainApi.off("select", onSelect);
    };
  }, [mainApi]);

  const scrollTo = useCallback((i: number) => mainApi?.scrollTo(i), [mainApi]);

  if (images.length === 0) {
    return (
      <div className="aspect-square rounded-2xl bg-brand-100" aria-label="No image" />
    );
  }

  if (images.length === 1) {
    return (
      <div className="relative aspect-square rounded-2xl overflow-hidden bg-brand-50">
        <Image
          src={images[0]}
          alt={alt}
          fill
          priority
          sizes="(max-width: 1024px) 100vw, 50vw"
          className="object-cover"
        />
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <div ref={mainRef} className="overflow-hidden rounded-2xl bg-brand-50">
        <div className="flex">
          {images.map((src, i) => (
            <div key={src} className="relative flex-[0_0_100%] aspect-square">
              <Image
                src={src}
                alt={alt}
                fill
                priority={i === 0}
                sizes="(max-width: 1024px) 100vw, 50vw"
                className="object-cover"
              />
            </div>
          ))}
        </div>
      </div>
      <div className="flex gap-2 overflow-x-auto scrollbar-hidden">
        {images.map((src, i) => (
          <button
            key={src}
            type="button"
            aria-label={`Image ${i + 1} of ${images.length}`}
            aria-current={i === index ? "true" : undefined}
            onClick={() => scrollTo(i)}
            className={cn(
              "relative size-16 flex-shrink-0 rounded-lg overflow-hidden bg-brand-50 border-2",
              i === index ? "border-brand-600" : "border-transparent",
            )}
          >
            <Image src={src} alt="" fill sizes="64px" className="object-cover" />
          </button>
        ))}
      </div>
    </div>
  );
}
