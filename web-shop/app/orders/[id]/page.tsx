import { notFound } from "next/navigation";
import { OrderDetailShell } from "@/components/orders/order-detail-shell";

export const dynamic = "force-dynamic";

const ID_RE = /^[0-9a-f-]{36}$/i;

export default async function OrderDetailPage(
  props: { params: Promise<{ id: string }>; searchParams: Promise<{ placed?: string }> },
) {
  const { id } = await props.params;
  const { placed } = await props.searchParams;
  if (!ID_RE.test(id)) notFound();
  return (
    <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-8">
      <OrderDetailShell id={id} placed={placed === "1"} />
    </main>
  );
}
