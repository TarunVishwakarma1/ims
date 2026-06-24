"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { toast } from "sonner";
import {
  fetchCart,
  listAddresses,
  fetchCheckoutSummary,
  fetchPaymentOptions,
} from "@/lib/shop-api";
import type {
  Address,
  CheckoutSummary as CheckoutSummaryT,
  PaymentOption,
} from "@/lib/shop-types";
import { useCartStore, selectItemCount } from "@/lib/cart-store";
import { AddressPicker } from "@/components/checkout/address-picker";
import { AddressForm } from "@/components/checkout/address-form";
import { PaymentMethod } from "@/components/checkout/payment-method";
import { OrderSummary } from "@/components/checkout/order-summary";
import { PlaceOrderButton } from "@/components/checkout/place-order-button";
import { LoginModal } from "@/components/auth/login-modal";

type Method = "razorpay" | "cod";

export function CheckoutShell() {
  const [authed, setAuthed] = useState<boolean | null>(null); // null = unknown
  const [showLogin, setShowLogin] = useState(false);
  const [addresses, setAddresses] = useState<Address[]>([]);
  const [selected, setSelected] = useState<Address | null>(null);
  const [showForm, setShowForm] = useState(false);
  const [options, setOptions] = useState<PaymentOption[]>([]);
  const [method, setMethod] = useState<Method>("razorpay");
  const [summary, setSummary] = useState<CheckoutSummaryT | null>(null);
  const hydrate = useCartStore((s) => s.hydrateFromServer);
  const itemCount = useCartStore(selectItemCount);

  // Step 1: probe auth + load cart
  useEffect(() => {
    (async () => {
      try {
        const cart = await fetchCart();
        hydrate(cart);
        setAuthed(true);
      } catch (e) {
        const status = (e as { status?: number }).status;
        if (status === 401) {
          setAuthed(false);
          setShowLogin(true);
        } else {
          toast.error("Could not load cart");
        }
      }
    })();
  }, [hydrate]);

  // Step 2: post-auth load addresses + options
  useEffect(() => {
    if (authed !== true) return;
    (async () => {
      try {
        const [addrs, opts] = await Promise.all([listAddresses(), fetchPaymentOptions()]);
        setAddresses(addrs);
        setOptions(opts);
        const def = addrs.find((a) => a.is_default) ?? addrs[0] ?? null;
        if (def) setSelected(def);
        else setShowForm(true);
        // pick first enabled method by default
        const firstEnabled = opts.find((o) => o.enabled);
        if (firstEnabled) setMethod(firstEnabled.id);
      } catch {
        toast.error("Could not load checkout");
      }
    })();
  }, [authed]);

  // Step 3: refetch summary when address changes
  useEffect(() => {
    if (!selected) return;
    (async () => {
      try {
        const s = await fetchCheckoutSummary(selected.id);
        setSummary(s);
      } catch (e) {
        const code = (e as { code?: string }).code;
        if (code === "cart_empty") toast.error("Cart is empty");
        else toast.error("Could not load summary");
      }
    })();
  }, [selected]);

  if (authed === null) {
    return <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-16 text-center">Loading…</main>;
  }

  if (authed === false) {
    return (
      <>
        <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-16 text-center">
          <h1 className="text-2xl font-semibold mb-3">Sign in to checkout</h1>
          <button onClick={() => setShowLogin(true)} className="h-10 px-6 rounded bg-brand-600 text-white">
            Sign in
          </button>
        </main>
        <LoginModal open={showLogin} onClose={() => setShowLogin(false)} onSuccess={() => { setShowLogin(false); setAuthed(true); }} />
      </>
    );
  }

  if (itemCount === 0) {
    return (
      <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-16 text-center">
        <h1 className="text-2xl font-semibold mb-3">Cart is empty</h1>
        <Link href="/" className="h-10 px-6 rounded bg-brand-600 text-white inline-grid place-items-center">Browse shop</Link>
      </main>
    );
  }

  return (
    <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-8 grid gap-8 lg:grid-cols-[1fr_360px]">
      <div className="space-y-6">
        <AddressPicker
          addresses={addresses}
          selected={selected}
          onSelect={(a) => { setSelected(a); setShowForm(false); }}
          onAddNew={() => setShowForm(true)}
        />
        {showForm && (
          <AddressForm
            onSave={(a) => {
              setAddresses((prev) => [a, ...prev]);
              setSelected(a);
              setShowForm(false);
            }}
            onCancel={() => setShowForm(false)}
          />
        )}
        <PaymentMethod options={options} value={method} onChange={setMethod} />
      </div>
      {summary && (
        <OrderSummary
          summary={summary}
          action={
            <PlaceOrderButton
              addressID={selected?.id ?? ""}
              paymentMethod={method}
              customerName={selected?.name}
              customerPhone={selected?.phone}
              disabled={!selected || !options.find((o) => o.id === method && o.enabled)}
            />
          }
        />
      )}
    </main>
  );
}
