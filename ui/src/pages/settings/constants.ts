import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { Role } from "@/gen/auth/v1/members_pb";

export const SETTINGS_PAGE_INSET = 64;
export const SETTINGS_PAGE_SIDEBAR_WIDTH = 240;

export const SETTINGS_TEAM_TABLE_LOADING_ROW_COUNT = 1;
export const SETTINGS_TEAM_TABLE_COLUMN_WIDTH_EMAIL = 200;
export const SETTINGS_TEAM_TABLE_COLUMN_WIDTH_ROLE = 240;
export const SETTINGS_TEAM_TABLE_ROLE_SELECT_WIDTH = 160;
export const SETTINGS_TEAM_TABLE_ROLE_SELECT_DROPDOWN_WIDTH = 160;

export const SETTINGS_SERVICE_ACCOUNTS_TABLE_LOADING_ROW_COUNT = 1;
export const SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_CLIENT_ID = 220;
export const SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ROLE = 140;
export const SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ACTIONS = 44;
export const SETTINGS_SERVICE_ACCOUNTS_ACTIONS_DROPDOWN_WIDTH = 160;

export const ROLE_EMPTY_LABEL = "-";

export const ROLES: Role[] = [Role.ADMIN, Role.CREATOR, Role.VIEWER];

export const SERVICE_ACCOUNT_ROLES: Role[] = [Role.ADMIN, Role.CREATOR, Role.VIEWER];

export const ROLE_TO_LABEL_MAP: Record<Role, string> = {
  [Role.UNSPECIFIED]: ROLE_EMPTY_LABEL,
  [Role.ADMIN]: "Admin",
  [Role.CREATOR]: "Creator",
  [Role.VIEWER]: "Viewer",
};

export const SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP: Record<Role, string> = {
  [Role.UNSPECIFIED]: ROLE_EMPTY_LABEL,
  [Role.ADMIN]: "All",
  [Role.CREATOR]: "Write",
  [Role.VIEWER]: "Read-only",
};

export const ROLE_OPTIONS: SelectInputOption[] = ROLES.map((role) => ({
  id: String(role),
  label: ROLE_TO_LABEL_MAP[role],
  value: role,
}));

export const SERVICE_ACCOUNT_ROLE_OPTIONS: SelectInputOption[] = SERVICE_ACCOUNT_ROLES.map(
  (role) => ({
    id: String(role),
    label: SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP[role],
    value: role,
  }),
);

export const INVITE_DEFAULT_ROLE = Role.CREATOR;

export const SERVICE_ACCOUNT_DEFAULT_ROLE = Role.CREATOR;
