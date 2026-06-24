"use client";
import { createContext, useContext, useState, type ReactNode } from "react";

type Ctx = { open: () => void; close: () => void; isOpen: boolean };
const CartDrawerCtx = createContext<Ctx | null>(null);

export function CartDrawerProvider({ children }: { children: ReactNode }) {
  const [isOpen, setOpen] = useState(false);
  return (
    <CartDrawerCtx.Provider value={{ open: () => setOpen(true), close: () => setOpen(false), isOpen }}>
      {children}
    </CartDrawerCtx.Provider>
  );
}

export function useCartDrawer(): Ctx {
  const c = useContext(CartDrawerCtx);
  if (!c) throw new Error("useCartDrawer must be used inside CartDrawerProvider");
  return c;
}
