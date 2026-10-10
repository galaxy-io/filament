import type { Member } from "@/gen/auth/v1/members_pb";

import { SETTINGS_MEMBER_FALLBACK_NAME } from "@/pages/settings/constants";

export const formatMemberName = (member: Pick<Member, "name" | "email">): string =>
  member.name || member.email || SETTINGS_MEMBER_FALLBACK_NAME;

export const formatCliLoginCommand = (clientId: string): string =>
  `filament auth login --server ${window.location.origin} --client-id ${clientId}`;
