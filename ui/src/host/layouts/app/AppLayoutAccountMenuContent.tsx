import type { FC } from "react";

import { css } from "@linaria/core";
import { styled } from "@linaria/react";
import {
  CaretUpDownIcon,
  GithubLogoIcon,
  PaletteIcon,
  PlusIcon,
  SignOutIcon,
  UsersThreeIcon,
  WrenchIcon,
} from "@phosphor-icons/react";
import { useNavigate, useRouteContext } from "@tanstack/react-router";

import Avatar, { AvatarSize } from "@galaxy-io/dls/avatar/Avatar";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Menu, { MenuItem, MenuRadioGroup, MenuSeparator } from "@galaxy-io/dls/menu/Menu";
import { FOCUS_RING, HAIRLINE_WIDTH, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { Placement } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import { useIsMainLayoutSidebarCollapsed } from "@/layouts/main/MainLayoutSidebarProvider";

import { SettingsTeamView } from "@/pages/settings/types";

import { useSignOut } from "@/host/auth/hooks/useSignOut";
import {
  APP_LAYOUT_ACCOUNT_FALLBACK_NAME,
  APP_LAYOUT_THEME_OPTIONS,
} from "@/host/layouts/app/constants";

import { useCanManageTeam, useListMembersQuery } from "@/api/queries/auth";

import { GITHUB_REPO_URL } from "@/constants";

const AccountButton = styled.button`
  ${INTERACTIVE_RESET}
  display: flex;
  align-items: center;
  gap: ${t.space[8]};
  width: 100%;
  min-height: ${t.size.control.large};
  padding: ${t.space[4]} ${t.space[8]};
  border: ${HAIRLINE_WIDTH} solid ${t.color.border.secondary};
  border-radius: ${t.radius.md};
  background-color: ${t.color.background.primary};
  transition:
    background-color ${t.duration.fast} ease-in-out,
    border-color ${t.duration.fast} ease-in-out;

  &:hover {
    background-color: ${t.color.background.hovered};
    border-color: ${t.color.border.tertiary};
  }

  &:active,
  &[aria-expanded="true"] {
    background-color: ${t.color.background.pressed};
    border-color: ${t.color.border.tertiary};
  }

  ${FOCUS_RING}
`;

const ACCOUNT_BUTTON_COLLAPSED_CSS = css`
  justify-content: center;
  width: ${t.size.control.medium};
  min-height: ${t.size.control.medium};
  padding: 0;
`;

const AppLayoutAccountMenuContent: FC = () => {
  const navigate = useNavigate();
  const signOut = useSignOut();
  const { session } = useRouteContext({ from: "/_app" });
  const { selectedTheme, setTheme } = useGalaxyTheme();
  const isSidebarCollapsed = useIsMainLayoutSidebarCollapsed();

  const membersQuery = useListMembersQuery();
  const currentMember = membersQuery.data?.members.find(
    (member) => member.userId === session.userId,
  );
  const canManageTeam = useCanManageTeam() === true;
  const name =
    currentMember?.name ||
    currentMember?.email ||
    session.name ||
    session.email ||
    APP_LAYOUT_ACCOUNT_FALLBACK_NAME;

  return (
    <Menu
      placement={isSidebarCollapsed ? Placement.RIGHT_END : Placement.TOP_START}
      shouldMatchTriggerWidth={!isSidebarCollapsed}
      ariaLabel="Account"
      trigger={
        <AccountButton
          type="button"
          aria-label={isSidebarCollapsed ? name : undefined}
          className={isSidebarCollapsed ? ACCOUNT_BUTTON_COLLAPSED_CSS : undefined}
        >
          <Avatar
            img={session.avatarUrl}
            size={AvatarSize.SMALL}
            seed={session.userId}
            name={name}
            isSquare
          />
          {!isSidebarCollapsed && (
            <>
              <FlexItem grow={1} minWidth={0}>
                <Text size={TextSize.BODY_SM} align="left" lineClamp={1}>
                  {name}
                </Text>
              </FlexItem>
              <FlexItem shrink={0}>
                <Icon component={CaretUpDownIcon} variant={IconVariant.TERTIARY} />
              </FlexItem>
            </>
          )}
        </AccountButton>
      }
    >
      <MenuItem
        label="Team"
        icon={UsersThreeIcon}
        onSelect={() => void navigate({ to: "/settings/team" })}
      />
      {canManageTeam && (
        <MenuItem
          label="Service accounts"
          icon={WrenchIcon}
          onSelect={() => void navigate({ to: "/settings/service-accounts" })}
        />
      )}
      {canManageTeam && (
        <MenuItem
          label="Invite team member"
          icon={PlusIcon}
          onSelect={() =>
            void navigate({ to: "/settings/team", search: { view: SettingsTeamView.INVITE } })
          }
        />
      )}
      <MenuSeparator />
      <MenuItem label="Theme" icon={PaletteIcon}>
        <MenuRadioGroup
          label="Theme"
          options={APP_LAYOUT_THEME_OPTIONS}
          value={selectedTheme}
          onChange={setTheme}
        />
      </MenuItem>
      <MenuItem label="Star on GitHub" icon={GithubLogoIcon} href={GITHUB_REPO_URL} isExternal />
      <MenuSeparator />
      <MenuItem label="Log out" icon={SignOutIcon} onSelect={() => void signOut()} />
    </Menu>
  );
};

export default AppLayoutAccountMenuContent;
