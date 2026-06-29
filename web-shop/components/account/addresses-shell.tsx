"use client";

import { useEffect, useState } from "react";
import { MapPin, Pencil, Trash2, Star } from "lucide-react";
import { listAddresses, deleteAddress, setDefaultAddress } from "@/lib/shop-api";
import type { Address } from "@/lib/shop-types";
import { AddressForm } from "@/components/checkout/address-form";
import { toast } from "sonner";

type Mode = { kind: "list" } | { kind: "add" } | { kind: "edit"; addr: Address };

export function AddressesShell() {
  const [addresses, setAddresses] = useState<Address[] | null>(null);
  const [mode, setMode] = useState<Mode>({ kind: "list" });
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const load = async () => {
    try {
      setAddresses(await listAddresses());
    } catch {
      toast.error("Could not load addresses");
      setAddresses([]);
    }
  };

  useEffect(() => { load(); /* eslint-disable-line react-hooks/exhaustive-deps */ }, []);

  const onDelete = async (id: string) => {
    setBusy(true);
    try {
      await deleteAddress(id);
      toast.success("Address removed");
      setConfirmDelete(null);
      await load();
    } catch {
      toast.error("Could not remove address");
    } finally {
      setBusy(false);
    }
  };

  const onMakeDefault = async (id: string) => {
    setBusy(true);
    try {
      await setDefaultAddress(id);
      await load();
    } catch {
      toast.error("Could not set default");
    } finally {
      setBusy(false);
    }
  };

  if (addresses === null) return <p className="text-text-muted">Loading…</p>;

  if (mode.kind === "add") {
    return (
      <AddressForm
        onSave={() => { setMode({ kind: "list" }); load(); }}
        onCancel={() => setMode({ kind: "list" })}
      />
    );
  }

  if (mode.kind === "edit") {
    return (
      <AddressForm
        initial={mode.addr}
        onSave={() => { setMode({ kind: "list" }); load(); }}
        onCancel={() => setMode({ kind: "list" })}
      />
    );
  }

  return (
    <div className="space-y-4">
      {addresses.length === 0 ? (
        <div className="text-center py-12 space-y-3">
          <MapPin className="size-10 mx-auto text-text-muted" aria-hidden />
          <p className="text-text-muted">No saved addresses yet.</p>
        </div>
      ) : (
        <ul className="space-y-3">
          {addresses.map((a) => (
            <li key={a.id} className="border border-border rounded-lg p-4">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <span className="font-medium">{a.name}</span>
                    {a.is_default && (
                      <span className="px-2 py-0.5 rounded text-xs font-medium bg-emerald-100 text-emerald-700">
                        Default
                      </span>
                    )}
                  </div>
                  <p className="text-sm text-text-muted">{a.phone}</p>
                  <p className="text-sm text-text-muted">
                    {a.line1}{a.line2 ? `, ${a.line2}` : ""}, {a.city}, {a.state} {a.pincode}
                  </p>
                </div>
                <div className="flex shrink-0 gap-1">
                  <button
                    type="button"
                    onClick={() => setMode({ kind: "edit", addr: a })}
                    aria-label="Edit address"
                    className="size-8 grid place-items-center rounded hover:bg-brand-50"
                  >
                    <Pencil className="size-4" />
                  </button>
                  <button
                    type="button"
                    onClick={() => setConfirmDelete(a.id)}
                    aria-label="Delete address"
                    className="size-8 grid place-items-center rounded hover:bg-red-50 text-danger"
                  >
                    <Trash2 className="size-4" />
                  </button>
                </div>
              </div>

              {confirmDelete === a.id && (
                <div className="mt-3 flex flex-wrap items-center gap-2 border-t border-border pt-3">
                  <p className="text-sm flex-1 min-w-0">Remove this address?</p>
                  <button type="button" onClick={() => setConfirmDelete(null)} disabled={busy} className="h-9 px-3 rounded border border-border text-sm disabled:opacity-60">Keep</button>
                  <button type="button" onClick={() => onDelete(a.id)} disabled={busy} className="h-9 px-3 rounded bg-danger text-white text-sm disabled:opacity-60">{busy ? "Removing…" : "Remove"}</button>
                </div>
              )}

              {!a.is_default && confirmDelete !== a.id && (
                <button
                  type="button"
                  onClick={() => onMakeDefault(a.id)}
                  disabled={busy}
                  className="mt-3 inline-flex items-center gap-1 text-xs text-brand-600 hover:underline disabled:opacity-60"
                >
                  <Star className="size-3" /> Make default
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      <button
        type="button"
        onClick={() => setMode({ kind: "add" })}
        className="h-10 px-6 rounded bg-brand-600 text-white text-sm font-medium hover:bg-brand-700"
      >
        Add new address
      </button>
    </div>
  );
}
