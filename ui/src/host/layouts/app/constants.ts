import { DesktopIcon, MoonIcon, SunIcon } from "@phosphor-icons/react";

import type { MenuRadioOption } from "@galaxy-io/dls/menu/Menu";
import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";

export const APP_LAYOUT_THEME_OPTIONS: MenuRadioOption<GalaxyTheme>[] = [
  { id: GalaxyTheme.SYSTEM, label: "System", icon: DesktopIcon },
  { id: GalaxyTheme.LIGHT, label: "Light", icon: SunIcon },
  { id: GalaxyTheme.DARK, label: "Dark", icon: MoonIcon },
];

export const APP_LAYOUT_ACCOUNT_FALLBACK_NAME = "Account";

export const APP_LAYOUT_COMMAND_PALETTE_PLACEHOLDER =
  "Search pipelines, connections, pages and actions...";
export const APP_LAYOUT_COMMAND_PALETTE_STORAGE_KEY = "filament:command-palette";
export const APP_LAYOUT_COMMAND_PALETTE_GROUP_SETTINGS = "Settings";
export const APP_LAYOUT_COMMAND_PALETTE_GROUP_PREFERENCES = "Preferences";
