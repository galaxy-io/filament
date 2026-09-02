import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { Role } from "@/gen/auth/v1/members_pb";

import {
  ROLE_OPTIONS,
  ROLE_TO_LABEL_MAP,
  SERVICE_ACCOUNT_ROLE_OPTIONS,
  SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP,
} from "@/pages/settings/constants";

export const roleLabel = (role: Role): string => ROLE_TO_LABEL_MAP[role];

export const serviceAccountRoleLabel = (role: Role): string =>
  SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP[role];

export const roleOption = (role: Role) =>
  ROLE_OPTIONS.find((option) => option.value === role) ?? null;

export const serviceAccountRoleOption = (role: Role) =>
  SERVICE_ACCOUNT_ROLE_OPTIONS.find((option) => option.value === role) ?? null;

export const optionRole = (option: SelectInputOption): Role =>
  typeof option.value === "number" ? (option.value as Role) : Role.UNSPECIFIED;

export const buildCliLoginCommand = (clientId: string): string =>
  `filament auth login --server ${window.location.origin} --client-id ${clientId}`;
