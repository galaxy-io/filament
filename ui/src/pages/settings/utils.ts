import { Role } from "@/gen/auth/v1/members_pb";

import { ROLE_TO_LABEL_MAP, SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP } from "@/pages/settings/constants";

export const roleLabel = (role: Role): string => ROLE_TO_LABEL_MAP[role];

export const serviceAccountRoleLabel = (role: Role): string =>
  SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP[role];

export const roleToOptionId = (role: Role): string => String(role);

export const optionIdToRole = (id: string | null): Role => {
  const role = Number(id);
  return id !== null && role in ROLE_TO_LABEL_MAP ? (role as Role) : Role.UNSPECIFIED;
};

export const buildCliLoginCommand = (clientId: string): string =>
  `filament auth login --server ${window.location.origin} --client-id ${clientId}`;
