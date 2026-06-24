import type { Metadata } from "next";
import { Toaster } from "sonner";
import { SiteHeader } from "@/components/site-header";
import { SiteFooter } from "@/components/site-footer";
import { CartDrawerProvider } from "@/components/cart/cart-drawer-context";
import "./globals.css";
import { Geist } from "next/font/google";
import { cn } from "@/lib/utils";

const geist = Geist({subsets:['latin'],variable:'--font-sans'});

export const metadata: Metadata = {
  title: "Shop",
  description: "Festival deals + everyday groceries",
};

export default function RootLayout({
  children,
  cart,
}: {
  children: React.ReactNode;
  cart: React.ReactNode;
}) {
  return (
    <html lang="en" className={cn("font-sans", geist.variable)}>
      <body className="min-h-screen flex flex-col">
        <CartDrawerProvider>
          <SiteHeader />
          <main className="flex-1 max-w-(--spacing-shop-page-max) w-full mx-auto px-4 py-6">
            {children}
          </main>
          <SiteFooter />
          {cart}
          <Toaster position="top-center" richColors />
        </CartDrawerProvider>
      </body>
    </html>
  );
}
