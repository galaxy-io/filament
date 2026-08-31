import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { Role } from "@/gen/auth/v1/members_pb";

export const ROLE_TO_LABEL_MAP: Partial<Record<Role, string>> = {
  [Role.ADMIN]: "Admin",
  [Role.CREATOR]: "Creator",
  [Role.VIEWER]: "Viewer",
};

// Every role picker reads from the label map, so a new role needs one edit.
export const ROLE_OPTIONS: SelectInputOption[] = Object.entries(ROLE_TO_LABEL_MAP).map(
  ([role, label]) => ({ id: role, label, value: Number(role) as Role }),
);

const SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP: Partial<Record<Role, string>> = {
  [Role.ADMIN]: "All",
  [Role.CREATOR]: "Write",
  [Role.VIEWER]: "Read-only",
};

export const SERVICE_ACCOUNT_ROLE_OPTIONS: SelectInputOption[] = Object.entries(
  SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP,
).map(([role, label]) => ({ id: role, label, value: Number(role) as Role }));

export const SERVICE_ACCOUNT_DEFAULT_ROLE =
  SERVICE_ACCOUNT_ROLE_OPTIONS.find((option) => option.value === Role.CREATOR) ??
  SERVICE_ACCOUNT_ROLE_OPTIONS[0];

export const serviceAccountRoleLabel = (role: Role | undefined): string | undefined =>
  role === undefined ? undefined : SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP[role];

export const INVITE_DEFAULT_ROLE =
  ROLE_OPTIONS.find((option) => option.value === Role.CREATOR) ?? ROLE_OPTIONS[0];

export const roleOption = (role: Role) =>
  ROLE_OPTIONS.find((option) => option.value === role) ?? null;

export const optionRole = (option: SelectInputOption): Role | undefined =>
  typeof option.value === "number" ? (option.value as Role) : undefined;

export const roleLabel = (role: Role | undefined): string | undefined =>
  role === undefined ? undefined : ROLE_TO_LABEL_MAP[role];
