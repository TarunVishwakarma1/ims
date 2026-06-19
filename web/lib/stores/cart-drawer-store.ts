import { create } from 'zustand'

interface CartDrawerState {
  open: boolean
  setOpen: (next: boolean) => void
  toggle: () => void
}

export const useCartDrawer = create<CartDrawerState>((set) => ({
  open: false,
  setOpen: (next) => set({ open: next }),
  toggle: () => set((s) => ({ open: !s.open })),
}))
