import { type FC, useState } from "react";

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
import Menu, { MenuItem, MenuRadioGroup, MenuSeparator } from "@galaxy-io/dls/menu/Menu";
import { FOCUS_RING, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { Placement } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

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
  border-radius: ${t.radius.md};
  transition: background-color ${t.duration.fast} ease-in-out;

  &:hover,
  &[aria-expanded="true"] {
    background-color: ${t.color.background.hovered};
  }

  ${FOCUS_RING}
`;

const AppLayoutAccountMenuContent: FC = () => {
  const navigate = useNavigate();
  const signOut = useSignOut();
  const { session } = useRouteContext({ from: "/_app" });
  const { selectedTheme, setTheme } = useGalaxyTheme();
  const [isOpen, setIsOpen] = useState(false);

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
      placement={Placement.TOP_START}
      isOpen={isOpen}
      onOpenChange={setIsOpen}
      ariaLabel="Account"
      trigger={
        <AccountButton type="button" aria-label="Account">
          <Avatar
            img={session.avatarUrl}
            size={AvatarSize.SMALL}
            seed={session.userId}
            name={name}
            isSquare
          />
          <Text size={TextSize.BODY_SM} lineClamp={1}>
            {name}
          </Text>
          <Icon component={CaretUpDownIcon} variant={IconVariant.TERTIARY} />
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
