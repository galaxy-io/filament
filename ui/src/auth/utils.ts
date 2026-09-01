import { INVITE_PATH_PREFIX } from "@/auth/constants";
import type { InviteToken } from "@/auth/types";

const toBase64Url = (value: string) =>
  btoa(value).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");

const fromBase64Url = (value: string) => {
  const base64 = value.replace(/-/g, "+").replace(/_/g, "/");
  return atob(base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), "="));
};

export const encodeInviteToken = ({ userId, code }: InviteToken): string =>
  toBase64Url(JSON.stringify({ userId, code }));

export const decodeInviteToken = (token: string): InviteToken | undefined => {
  try {
    const parsed = JSON.parse(fromBase64Url(token)) as Partial<InviteToken>;
    return parsed.userId && parsed.code ? { userId: parsed.userId, code: parsed.code } : undefined;
  } catch {
    return undefined;
  }
};

export const buildInviteUrl = (token: string): string =>
  `${window.location.origin}${INVITE_PATH_PREFIX}/${token}`;
