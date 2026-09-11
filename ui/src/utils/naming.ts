/**
 * Mirrors the server's identifier normalization (internal/naming.Normalize):
 * lowercase ASCII alphanumerics, runs of other characters collapse to one
 * underscore, edges are trimmed, leading digits are prefixed, and output is
 * capped at PostgreSQL's 63-byte identifier limit.
 *
 * This is presentation-only; the server computes the authoritative value when
 * the pipeline version is saved.
 */
const MAX_IDENTIFIER_LENGTH = 63;

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
  return out.length > MAX_IDENTIFIER_LENGTH
    ? out.slice(0, MAX_IDENTIFIER_LENGTH).replace(/_+$/, "")
    : out;
}
