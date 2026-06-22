export function safeNext(raw: string | null | undefined): string {
  if (!raw) return "/";
  if (raw.startsWith("//") || raw.startsWith("\\") || /^[a-z][a-z0-9+.-]*:/i.test(raw)) {
    return "/";
  }
  if (!raw.startsWith("/")) return "/";
  return raw;
}
