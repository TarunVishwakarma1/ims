"use client";

import { useState } from "react";
import type { Address, AddressInput } from "@/lib/shop-types";
import { addAddress } from "@/lib/shop-api";
import { toast } from "sonner";

const PIN_RE = /^[1-9]\d{5}$/;
const PHONE_RE = /^[6-9]\d{9}$/;

type Props = {
  onSave: (a: Address) => void;
  onCancel: () => void;
};

export function AddressForm({ onSave, onCancel }: Props) {
  const [v, setV] = useState<AddressInput>({
    name: "", phone: "", line1: "", line2: "", city: "", state: "", pincode: "",
  });
  const [saving, setSaving] = useState(false);

  const set = <K extends keyof AddressInput>(k: K, val: AddressInput[K]) =>
    setV((prev) => ({ ...prev, [k]: val }));

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!v.name.trim() || !v.line1.trim() || !v.city.trim() || !v.state.trim()) {
      toast.error("Fill all required fields");
      return;
    }
    if (!PHONE_RE.test(v.phone)) {
      toast.error("Enter a valid phone");
      return;
    }
    if (!PIN_RE.test(v.pincode)) {
      toast.error("Enter a valid 6-digit pincode");
      return;
    }
    setSaving(true);
    try {
      const created = await addAddress(v);
      onSave(created);
    } catch {
      toast.error("Could not save address");
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={submit} className="space-y-3 mt-3 p-3 border border-border rounded">
      <h3 className="font-medium">New address</h3>
      <div className="grid grid-cols-2 gap-2">
        <label className="block text-sm col-span-2">
          Name
          <input required maxLength={80} value={v.name} onChange={(e) => set("name", e.target.value)} className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg" />
        </label>
        <label className="block text-sm col-span-2">
          Phone
          <input required inputMode="numeric" maxLength={10} value={v.phone} onChange={(e) => set("phone", e.target.value.replace(/\D/g, ""))} className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg" />
        </label>
        <label className="block text-sm col-span-2">
          Address line 1
          <input required maxLength={120} value={v.line1} onChange={(e) => set("line1", e.target.value)} className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg" />
        </label>
        <label className="block text-sm col-span-2">
          Address line 2 (optional)
          <input maxLength={120} value={v.line2 ?? ""} onChange={(e) => set("line2", e.target.value)} className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg" />
        </label>
        <label className="block text-sm">
          City
          <input required maxLength={50} value={v.city} onChange={(e) => set("city", e.target.value)} className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg" />
        </label>
        <label className="block text-sm">
          State
          <input required maxLength={50} value={v.state} onChange={(e) => set("state", e.target.value)} className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg" />
        </label>
        <label className="block text-sm col-span-2">
          Pincode
          <input required inputMode="numeric" maxLength={6} value={v.pincode} onChange={(e) => set("pincode", e.target.value.replace(/\D/g, ""))} className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg" />
        </label>
      </div>
      <div className="flex gap-2 justify-end">
        <button type="button" onClick={onCancel} className="h-10 px-4 rounded border border-border text-sm hover:bg-brand-50">Cancel</button>
        <button type="submit" disabled={saving} className="h-10 px-4 rounded bg-brand-600 text-white text-sm disabled:opacity-60">{saving ? "Saving…" : "Save address"}</button>
      </div>
    </form>
  );
}
