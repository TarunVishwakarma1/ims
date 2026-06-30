"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Package, MapPin, LogOut, Mail, ChevronRight } from "lucide-react";
import { getMe, updateMe } from "@/lib/shop-api";
import type { CustomerProfile } from "@/lib/shop-types";
import { LoginModal } from "@/components/auth/login-modal";
import { toast } from "sonner";

const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

export function ProfileShell() {
  const router = useRouter();
  const [authed, setAuthed] = useState<boolean | null>(null);
  const [me, setMe] = useState<CustomerProfile | null>(null);
  const [editing, setEditing] = useState(false);
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [saving, setSaving] = useState(false);

  const load = async () => {
    try {
      const p = await getMe();
      setMe(p);
      setName(p.name);
      setEmail(p.email ?? "");
      setAuthed(true);
    } catch (e) {
      if ((e as { status?: number }).status === 401) setAuthed(false);
      else toast.error("Could not load profile");
    }
  };

  useEffect(() => { load(); /* eslint-disable-line react-hooks/exhaustive-deps */ }, []);

  const onSave = async () => {
    if (!name.trim()) {
      toast.error("Name is required");
      return;
    }
    if (email.trim() && !EMAIL_RE.test(email.trim())) {
      toast.error("Enter a valid email");
      return;
    }
    setSaving(true);
    try {
      await updateMe({ name: name.trim(), email: email.trim() });
      toast.success("Profile updated");
      setEditing(false);
      await load();
    } catch (e) {
      const code = (e as { code?: string }).code;
      toast.error(code === "email_taken" ? "That email is already in use" : "Could not update profile");
    } finally {
      setSaving(false);
    }
  };

  const logout = async () => {
    await fetch("/api/auth/logout", { method: "POST" });
    router.push("/");
    router.refresh();
  };

  if (authed === false) {
    return <LoginModal open onClose={() => router.push("/")} onSuccess={() => { setAuthed(null); load(); }} />;
  }
  if (authed === null || !me) return <p className="text-text-muted">Loading…</p>;

  const needsEmail = !me.email;

  return (
    <div className="space-y-6">
      {needsEmail && !editing && (
        <div className="flex items-start gap-3 p-3 rounded-lg bg-brand-50 border border-brand-200 text-sm">
          <Mail className="size-5 shrink-0 text-brand-600" aria-hidden />
          <div className="flex-1">
            <p className="font-medium">Add your email</p>
            <p className="text-text-muted">Get order confirmations, payment receipts, and delivery updates by email.</p>
          </div>
          <button type="button" onClick={() => setEditing(true)} className="h-8 px-3 rounded bg-brand-600 text-white text-xs font-medium shrink-0">
            Add
          </button>
        </div>
      )}

      <section className="border border-border rounded-lg p-4 space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="font-semibold">Profile</h2>
          {!editing && (
            <button type="button" onClick={() => setEditing(true)} className="text-sm text-brand-600 hover:underline">
              Edit
            </button>
          )}
        </div>

        {editing ? (
          <div className="space-y-3">
            <label className="block text-sm">
              Name
              <input maxLength={80} value={name} onChange={(e) => setName(e.target.value)} className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg" />
            </label>
            <label className="block text-sm">
              Email
              <input type="email" maxLength={120} value={email} onChange={(e) => setEmail(e.target.value)} placeholder="you@example.com" className="mt-1 w-full h-10 px-3 rounded border border-border bg-bg" />
            </label>
            <p className="text-xs text-text-muted">Phone {me.phone} can’t be changed (used for login).</p>
            <div className="flex gap-2 justify-end">
              <button type="button" onClick={() => { setEditing(false); setName(me.name); setEmail(me.email ?? ""); }} disabled={saving} className="h-10 px-4 rounded border border-border text-sm disabled:opacity-60">Cancel</button>
              <button type="button" onClick={onSave} disabled={saving} className="h-10 px-4 rounded bg-brand-600 text-white text-sm disabled:opacity-60">{saving ? "Saving…" : "Save"}</button>
            </div>
          </div>
        ) : (
          <dl className="text-sm space-y-2">
            <div className="flex justify-between"><dt className="text-text-muted">Name</dt><dd>{me.name || "—"}</dd></div>
            <div className="flex justify-between"><dt className="text-text-muted">Email</dt><dd>{me.email || "—"}</dd></div>
            <div className="flex justify-between"><dt className="text-text-muted">Phone</dt><dd>{me.phone || "—"}</dd></div>
          </dl>
        )}
      </section>

      <nav className="border border-border rounded-lg divide-y divide-border">
        <Link href="/orders" className="flex items-center gap-3 p-4 hover:bg-brand-50/40">
          <Package className="size-5 text-text-muted" aria-hidden />
          <span className="flex-1 text-sm font-medium">My orders</span>
          <ChevronRight className="size-4 text-text-muted" aria-hidden />
        </Link>
        <Link href="/addresses" className="flex items-center gap-3 p-4 hover:bg-brand-50/40">
          <MapPin className="size-5 text-text-muted" aria-hidden />
          <span className="flex-1 text-sm font-medium">Saved addresses</span>
          <ChevronRight className="size-4 text-text-muted" aria-hidden />
        </Link>
      </nav>

      <button
        type="button"
        onClick={logout}
        className="inline-flex items-center gap-2 h-10 px-4 rounded border border-border text-sm text-danger hover:bg-red-50"
      >
        <LogOut className="size-4" /> Sign out
      </button>
    </div>
  );
}
