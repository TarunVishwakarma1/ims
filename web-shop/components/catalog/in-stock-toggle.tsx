"use client";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { useCallback } from "react";

export function InStockToggle({ checked }: { checked: boolean }) {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const onChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const next = new URLSearchParams(searchParams.toString());
      if (e.target.checked) next.set("in_stock", "true");
      else next.delete("in_stock");
      const qs = next.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname);
    },
    [router, pathname, searchParams],
  );

  return (
    <label className="flex items-center gap-2 text-sm cursor-pointer">
      <input
        type="checkbox"
        checked={checked}
        onChange={onChange}
        className="size-4 rounded border-border accent-brand-600"
      />
      <span>In stock only</span>
    </label>
  );
}
