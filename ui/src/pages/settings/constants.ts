import { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import type { PaletteColor } from "@galaxy-io/dls/theme/tokens/types";

import { Role } from "@/gen/auth/v1/members_pb";

import { createEnumSelectOptions } from "@/utils/select";

export const SETTINGS_PAGE_SIDEBAR_WIDTH = 240;

export const SETTINGS_TEAM_TABLE_COLUMN_MIN_WIDTH_NAME = 200;
export const SETTINGS_TEAM_TABLE_COLUMN_MIN_WIDTH_EMAIL = 220;
export const SETTINGS_TEAM_TABLE_COLUMN_WIDTH_ROLE = 120;

export const SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_MIN_WIDTH_NAME = 200;
export const SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_CLIENT_ID = 220;
export const SETTINGS_SERVICE_ACCOUNTS_TABLE_COLUMN_WIDTH_ROLE = 140;

export const SETTINGS_MEMBER_FALLBACK_NAME = "Member";

const SETTINGS_ROLE_EMPTY_LABEL = "-";

const SETTINGS_ROLES: Role[] = [Role.ADMIN, Role.CREATOR, Role.VIEWER];

export const SETTINGS_ROLE_TO_LABEL_MAP: Record<Role, string> = {
  [Role.UNSPECIFIED]: SETTINGS_ROLE_EMPTY_LABEL,
  [Role.ADMIN]: "Admin",
  [Role.CREATOR]: "Creator",
  [Role.VIEWER]: "Viewer",
};

export const SETTINGS_SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP: Record<Role, string> = {
  [Role.UNSPECIFIED]: SETTINGS_ROLE_EMPTY_LABEL,
  [Role.ADMIN]: "All",
  [Role.CREATOR]: "Write",
  [Role.VIEWER]: "Read-only",
};

export const SETTINGS_ROLE_OPTIONS = createEnumSelectOptions(
  SETTINGS_ROLES,
  SETTINGS_ROLE_TO_LABEL_MAP,
);

export const SETTINGS_SERVICE_ACCOUNT_ROLE_OPTIONS = createEnumSelectOptions(
  SETTINGS_ROLES,
  SETTINGS_SERVICE_ACCOUNT_ROLE_TO_LABEL_MAP,
);

export const SETTINGS_ROLE_TO_CHIP_PROPS_MAP: Record<
  Role,
  { variant: ChipVariant } | { color: PaletteColor }
> = {
  [Role.UNSPECIFIED]: { variant: ChipVariant.TERTIARY },
  [Role.ADMIN]: { color: "purple" },
  [Role.CREATOR]: { variant: ChipVariant.SECONDARY },
  [Role.VIEWER]: { variant: ChipVariant.TERTIARY },
};

export const SETTINGS_INVITE_DEFAULT_ROLE = Role.CREATOR;

export const SETTINGS_SERVICE_ACCOUNT_DEFAULT_ROLE = Role.CREATOR;
