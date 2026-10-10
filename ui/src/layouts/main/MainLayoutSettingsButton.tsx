import { type FC, useCallback, useState } from "react";

import { styled } from "@linaria/react";
import {
  GithubLogoIcon,
  PlusIcon,
  SignOutIcon,
  UsersThreeIcon,
  WrenchIcon,
} from "@phosphor-icons/react";
import { useNavigate, useRouteContext } from "@tanstack/react-router";

import Avatar, { AvatarSize } from "@galaxy-io/dls/avatar/Avatar";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Popover from "@galaxy-io/dls/overlays/Popover";
import { FOCUS_RING, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";
import { Placement } from "@galaxy-io/dls/theme/enums";
import ThemeSwitcher, { ThemeSwitcherSize } from "@galaxy-io/dls/theme/ThemeSwitcher";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { MAIN_LAYOUT_SETTINGS_MENU_WIDTH } from "@/layouts/main/constants";

import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";

import { Flow } from "@/module/types";

import { useListMembersQuery } from "@/api/queries/auth";

import { GITHUB_REPO_URL } from "@/constants";

import { useSignOut } from "@/auth/hooks/useSignOut";
import type { AppSession } from "@/auth/types";

const AvatarButton = styled.button`
  ${INTERACTIVE_RESET}
  display: flex;
  align-items: center;
  justify-content: center;
  width: ${t.size.control.medium};
  height: ${t.size.control.medium};
  border-radius: ${t.radius.md};

  ${FOCUS_RING}
`;

const MenuItem = styled.button`
  ${INTERACTIVE_RESET}
  display: flex;
  align-items: center;
  gap: ${t.space[8]};
  width: 100%;
  min-height: ${t.size.control.large};
  padding: ${t.space[4]} ${t.space[8]};
  border-radius: ${t.radius.md};
  text-align: left;
  transition: background-color ${t.duration.fast} ease-in-out;

  &:hover {
    background-color: ${t.color.background.hovered};
  }

  ${FOCUS_RING}

  &:disabled {
    cursor: default;
    opacity: 0.55;
  }

  &:disabled:hover {
    background-color: transparent;
  }
`;

const memberDisplayName = (member: { name?: string; email?: string }): string =>
  member.name || member.email || "Member";

interface MainLayoutSettingsButtonMenuProps {
  name: string;
  avatarUrl?: string;
  seed?: string;
  canManageTeam: boolean;
  isTeamActionsPending: boolean;
  onOpenTeamSettings: () => void;
  onOpenServiceAccounts: () => void;
  onOpenInvite: () => void;
  onOpenGithub: () => void;
  onLogout: () => void;
}

const MainLayoutSettingsButtonMenu: FC<MainLayoutSettingsButtonMenuProps> = ({
  name,
  avatarUrl,
  seed,
  canManageTeam,
  isTeamActionsPending,
  onOpenTeamSettings,
  onOpenServiceAccounts,
  onOpenInvite,
  onOpenGithub,
  onLogout,
}) => {
  const shouldShowInvite = canManageTeam || isTeamActionsPending;

  return (
    <Flex
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.STRETCH}
      overflow="hidden"
      width={MAIN_LAYOUT_SETTINGS_MENU_WIDTH}
    >
      <Box variant={BoxVariant.PRIMARY} fillWidth>
        <Flex
          direction={FlexDirection.COLUMN}
          alignItems={AlignItems.CENTER}
          gap={12}
          padding={16}
          fillWidth
          minWidth={0}
        >
          <Avatar img={avatarUrl} size={AvatarSize.LARGE} seed={seed} name={name} isSquare />
          <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM} align="center" lineClamp={1}>
            {name}
          </Text>
          <ThemeSwitcher size={ThemeSwitcherSize.SMALL} isIconOnly />
        </Flex>
      </Box>
      <Divider />
      <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} padding={4} fillWidth>
        <MenuItem type="button" onClick={onOpenTeamSettings}>
          <Icon component={UsersThreeIcon} variant={IconVariant.TERTIARY} />
          <Text>Manage organization</Text>
        </MenuItem>
        {canManageTeam && (
          <MenuItem type="button" onClick={onOpenServiceAccounts}>
            <Icon component={WrenchIcon} variant={IconVariant.TERTIARY} />
            <Text>Service accounts</Text>
          </MenuItem>
        )}
      </Flex>
      <Divider />
      <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} padding={4} fillWidth>
        {shouldShowInvite && (
          <MenuItem type="button" onClick={onOpenInvite} disabled={isTeamActionsPending}>
            <Icon component={PlusIcon} variant={IconVariant.TERTIARY} />
            <Text weight={TextWeight.MEDIUM}>Invite team member</Text>
          </MenuItem>
        )}
        <MenuItem type="button" onClick={onOpenGithub}>
          <Icon component={GithubLogoIcon} variant={IconVariant.TERTIARY} />
          <Text>Star on GitHub</Text>
        </MenuItem>
      </Flex>
      <Divider />
      <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} padding={4} fillWidth>
        <MenuItem type="button" onClick={onLogout}>
          <Icon component={SignOutIcon} variant={IconVariant.TERTIARY} />
          <Text>Log out</Text>
        </MenuItem>
      </Flex>
    </Flex>
  );
};

const AuthenticatedSettingsButton: FC<{ session: AppSession }> = ({ session }) => {
  const navigate = useNavigate();
  const signOut = useSignOut();
  const [isOpen, setIsOpen] = useState(false);

  const membersQuery = useListMembersQuery({
    options: { enabled: isOpen && session.isAuthenticated },
  });

  const currentMember = membersQuery.data?.members.find(
    (member) => member.userId === session.userId,
  );
  const canManageTeam = membersQuery.data?.canManage ?? false;
  const isTeamActionsPending = membersQuery.isLoading && !membersQuery.data;
  const profileName =
    (currentMember ? memberDisplayName(currentMember) : session.name || session.email) ?? "Member";

  const handleOpenSettings = useCallback(
    (panel: SettingsPanel, teamView?: TeamSettingsView) => {
      setIsOpen(false);
      void navigate({
        to: ".",
        search: (prev) => ({
          ...prev,
          flow: Flow.SETTINGS,
          settings: panel,
          teamView,
          inviteToken: undefined,
        }),
      });
    },
    [navigate],
  );

  const handleOpenTeamSettings = useCallback(() => {
    handleOpenSettings(SettingsPanel.TEAM, TeamSettingsView.MEMBERS);
  }, [handleOpenSettings]);

  const handleOpenServiceAccounts = useCallback(() => {
    handleOpenSettings(SettingsPanel.SERVICE_ACCOUNTS);
  }, [handleOpenSettings]);

  const handleOpenInvite = useCallback(() => {
    handleOpenSettings(SettingsPanel.TEAM, TeamSettingsView.INVITE);
  }, [handleOpenSettings]);

  const handleOpenGithub = useCallback(() => {
    setIsOpen(false);
    window.open(GITHUB_REPO_URL, "_blank");
  }, []);

  const handleLogout = useCallback(() => {
    setIsOpen(false);
    void signOut();
  }, [signOut]);

  return (
    <Popover
      placement={Placement.BOTTOM_END}
      isOpen={isOpen}
      onOpenChange={setIsOpen}
      body={
        <MainLayoutSettingsButtonMenu
          name={profileName}
          avatarUrl={session.avatarUrl}
          seed={session.userId}
          canManageTeam={canManageTeam}
          isTeamActionsPending={isTeamActionsPending}
          onOpenTeamSettings={handleOpenTeamSettings}
          onOpenServiceAccounts={handleOpenServiceAccounts}
          onOpenInvite={handleOpenInvite}
          onOpenGithub={handleOpenGithub}
          onLogout={handleLogout}
        />
      }
    >
      <AvatarButton type="button" aria-label="Account settings">
        <Avatar
          img={session.avatarUrl}
          size={AvatarSize.SMALL}
          seed={session.userId}
          name={session.name}
          isSquare
        />
      </AvatarButton>
    </Popover>
  );
};

const MainLayoutSettingsButton: FC = () => {
  const { session } = useRouteContext({ from: "/_app" });

  if (!session.isAuthenticated) {
    return null;
  }

  return <AuthenticatedSettingsButton session={session} />;
};

export default MainLayoutSettingsButton;
