/**
 * Mirrors the server's identifier normalization (internal/naming.Normalize):
 * lowercase alphanumerics, runs of other characters collapse to a single
 * underscore, trimmed edges, leading digit prefixed, capped at 63 characters.
 * Presentation-only; the server computes the authoritative value on save.
 */
export function normalizeIdentifier(name: string): string {
  let out = "";
  let pending = false;
  for (const char of name.toLowerCase()) {
    if (/[a-z0-9]/.test(char)) {
      if (pending && out.length > 0) out += "_";
      pending = false;
      out += char;
    } else {
      pending = true;
    }
  }
  if (out === "") return "";
  if (/[0-9]/.test(out[0] ?? "")) out = `_${out}`;
  return out.length > 63 ? out.slice(0, 63).replace(/_+$/, "") : out;
}
