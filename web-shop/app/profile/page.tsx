import { ProfileShell } from "@/components/account/profile-shell";

export const dynamic = "force-dynamic";

export default function ProfilePage() {
  return (
    <main className="max-w-(--spacing-shop-page-max) mx-auto px-4 py-8">
      <h1 className="text-2xl font-semibold mb-4">My account</h1>
      <ProfileShell />
    </main>
  );
}
