import type { Member } from "@/gen/auth/v1/members_pb";

import {
  SETTINGS_INVITE_PATH_PREFIX,
  SETTINGS_MEMBER_FALLBACK_NAME,
} from "@/pages/settings/constants";
import type { SettingsInviteToken } from "@/pages/settings/types";

export const formatMemberName = (member: Pick<Member, "name" | "email">): string =>
  member.name || member.email || SETTINGS_MEMBER_FALLBACK_NAME;

export const formatCliLoginCommand = (clientId: string): string =>
  `filament auth login --server ${window.location.origin} --client-id ${clientId}`;

const toBase64Url = (value: string) =>
  btoa(value).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");

const fromBase64Url = (value: string) => {
  const base64 = value.replace(/-/g, "+").replace(/_/g, "/");
  return atob(base64.padEnd(base64.length + ((4 - (base64.length % 4)) % 4), "="));
};

export const encodeInviteToken = ({ userId, code }: SettingsInviteToken): string =>
  toBase64Url(JSON.stringify({ userId, code }));

export const decodeInviteToken = (token: string): SettingsInviteToken | undefined => {
  try {
    const parsed = JSON.parse(fromBase64Url(token)) as Partial<SettingsInviteToken>;
    return parsed.userId && parsed.code ? { userId: parsed.userId, code: parsed.code } : undefined;
  } catch {
    return undefined;
  }
};

export const createInviteUrl = (token: string): string =>
  `${window.location.origin}${SETTINGS_INVITE_PATH_PREFIX}/${token}`;
