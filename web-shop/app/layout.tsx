import type { Metadata } from "next";
import { Toaster } from "sonner";
import { SiteHeader } from "@/components/site-header";
import { SiteFooter } from "@/components/site-footer";
import "./globals.css";

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
    <html lang="en">
      <body className="min-h-screen flex flex-col">
        <SiteHeader />
        <main className="flex-1 max-w-(--spacing-shop-page-max) w-full mx-auto px-4 py-6">
          {children}
        </main>
        <SiteFooter />
        {cart}
        <Toaster position="top-center" richColors />
      </body>
    </html>
  );
}
