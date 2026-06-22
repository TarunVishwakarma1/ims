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
