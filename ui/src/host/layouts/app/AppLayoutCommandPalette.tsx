import { type FC, useMemo } from "react";

import { MoonIcon, SignOutIcon, SunIcon, UsersThreeIcon, WrenchIcon } from "@phosphor-icons/react";
import { useNavigate, useRouteContext } from "@tanstack/react-router";

import { useDisclosure } from "@galaxy-io/dls/hooks/useDisclosure";
import CommandPalette, { type CommandPaletteItem } from "@galaxy-io/dls/navigation/CommandPalette";
import { GalaxyTheme } from "@galaxy-io/dls/theme/enums";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import { useFilamentCommandItems } from "@/module/commands";

import { useSignOut } from "@/host/auth/hooks/useSignOut";
import {
  APP_LAYOUT_COMMAND_PALETTE_GROUP_PREFERENCES,
  APP_LAYOUT_COMMAND_PALETTE_GROUP_SETTINGS,
  APP_LAYOUT_COMMAND_PALETTE_PLACEHOLDER,
  APP_LAYOUT_COMMAND_PALETTE_STORAGE_KEY,
} from "@/host/layouts/app/constants";

import { useCanManageTeam } from "@/api/queries/auth";

const AppLayoutCommandPalette: FC = () => {
  const palette = useDisclosure();
  const navigate = useNavigate();
  const signOut = useSignOut();
  const { session } = useRouteContext({ from: "/_app" });
  const { activeTheme, setTheme } = useGalaxyTheme();
  const filamentItems = useFilamentCommandItems();
  const canManageTeam = useCanManageTeam({ enabled: session.isAuthenticated }) === true;
  const isDark = activeTheme === GalaxyTheme.DARK;

  const items = useMemo<CommandPaletteItem[]>(
    () => [
      ...filamentItems,
      ...(session.isAuthenticated
        ? [
            {
              id: "go-team",
              label: "Team",
              icon: UsersThreeIcon,
              group: APP_LAYOUT_COMMAND_PALETTE_GROUP_SETTINGS,
              keywords: ["members", "invite", "organization"],
              onSelect: () => void navigate({ to: "/settings/team" }),
            },
            ...(canManageTeam
              ? [
                  {
                    id: "go-service-accounts",
                    label: "Service accounts",
                    icon: WrenchIcon,
                    group: APP_LAYOUT_COMMAND_PALETTE_GROUP_SETTINGS,
                    keywords: ["credentials", "cli", "api"],
                    onSelect: () => void navigate({ to: "/settings/service-accounts" }),
                  },
                ]
              : []),
            {
              id: "sign-out",
              label: "Log out",
              icon: SignOutIcon,
              group: APP_LAYOUT_COMMAND_PALETTE_GROUP_SETTINGS,
              onSelect: () => void signOut(),
            },
          ]
        : []),
      {
        id: "toggle-theme",
        label: isDark ? "Switch to light theme" : "Switch to dark theme",
        icon: isDark ? SunIcon : MoonIcon,
        group: APP_LAYOUT_COMMAND_PALETTE_GROUP_PREFERENCES,
        keywords: ["theme", "appearance"],
        onSelect: () => setTheme(isDark ? GalaxyTheme.LIGHT : GalaxyTheme.DARK),
      },
    ],
    [filamentItems, session.isAuthenticated, canManageTeam, isDark, navigate, signOut, setTheme],
  );

  return (
    <CommandPalette
      items={items}
      isOpen={palette.isOpen}
      onOpenChange={palette.setIsOpen}
      shouldBindHotKey
      placeholder={APP_LAYOUT_COMMAND_PALETTE_PLACEHOLDER}
      storageKey={APP_LAYOUT_COMMAND_PALETTE_STORAGE_KEY}
    />
  );
};

export default AppLayoutCommandPalette;
