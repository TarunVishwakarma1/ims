export function safeNext(raw: string | null | undefined): string {
  if (!raw) return "/";
  const trimmed = raw.trim();
  if (!trimmed) return "/";
  // Reject any control character (NUL through SUB) — CRLF injection,
  // null-byte tricks, etc. Defense-in-depth before this path ever reaches
  // a server-side Location: header.
  if (/[\x00-\x1f]/.test(trimmed)) return "/";
  if (trimmed.startsWith("//") || trimmed.startsWith("\\") || /^[a-z][a-z0-9+.-]*:/i.test(trimmed)) {
    return "/";
  }
  if (!trimmed.startsWith("/")) return "/";
  return trimmed;
}

// safeCtaLink validates admin-curated banner CTA URLs. Allows absolute http/https
// and same-origin relative paths. Rejects javascript:, data:, file:, etc. to
// stop stored-XSS via a compromised admin or a banner_seed bug.
export function safeCtaLink(raw: string | null | undefined): string {
  if (!raw) return "#";
  const trimmed = raw.trim();
  if (!trimmed) return "#";
  if (/[\x00-\x1f]/.test(trimmed)) return "#";
  if (trimmed.startsWith("/") && !trimmed.startsWith("//")) return trimmed;
  if (/^https?:\/\//i.test(trimmed)) return trimmed;
  return "#";
}
