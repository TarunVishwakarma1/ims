import { OrdersShell } from "@/components/orders/orders-shell";

export const dynamic = "force-dynamic";

export default function OrdersPage() {
  return (
    <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-8">
      <h1 className="text-2xl font-semibold mb-4">My Orders</h1>
      <OrdersShell />
    </main>
  );
}
