import { useCallback, useState } from "react";

import { styled } from "@linaria/react";
import { PlusIcon, SignOutIcon, UsersThreeIcon, WrenchIcon } from "@phosphor-icons/react";
import { useNavigate, useRouteContext } from "@tanstack/react-router";

import Avatar, { AvatarSize } from "@galaxy-io/dls/avatar/Avatar";
import GridBackground, { GridBackgroundSize } from "@galaxy-io/dls/backgrounds/GridBackground";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import ToggleInput, { ToggleInputSize, type ToggleOption } from "@galaxy-io/dls/inputs/ToggleInput";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Popover from "@galaxy-io/dls/overlays/Popover";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { GalaxyTheme, Placement } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import type { Role } from "@/gen/auth/v1/members_pb";

import { Flow } from "@/layouts/app/types";

import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";
import { roleLabel } from "@/pages/settings/utils";

import { useListMembersQuery } from "@/api/queries/auth";

import { useSignOut } from "@/auth/hooks/useSignOut";
import type { AppSession } from "@/auth/types";

const MenuHeader = styled.div`
  width: 100%;
  background-color: ${t.color.background.primary};
  border-radius: ${t.radius.lg} ${t.radius.lg} 0 0;
  overflow: hidden;
`;

const AvatarButton = styled.button`
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 0;
  border-radius: 50%;
  background: transparent;
  cursor: pointer;
  overflow: hidden;
  transition: background-color 100ms ease;

  &:hover,
  &:focus-visible {
    background-color: ${t.color.background.tertiary};
  }

  &:focus-visible {
    outline: 1px solid ${t.color.border.secondary};
    outline-offset: 2px;
  }
`;

const MenuItem = styled.button`
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-height: 36px;
  padding: 6px 10px;
  border: 0;
  border-radius: 5px;
  background-color: transparent;
  text-align: left;
  cursor: pointer;
  transition: background-color 100ms ease;

  &:hover,
  &:focus-visible {
    background-color: ${t.color.background.tertiary};
  }

  &:focus-visible {
    outline: 1px solid ${t.color.border.secondary};
    outline-offset: 1px;
  }

  &:disabled {
    opacity: 0.55;
    cursor: default;
  }

  &:disabled:hover {
    background-color: transparent;
  }
`;

const MAIN_LAYOUT_SETTINGS_THEME_OPTIONS: ToggleOption<GalaxyTheme>[] = [
  { id: GalaxyTheme.LIGHT, label: "Light" },
  { id: GalaxyTheme.DARK, label: "Dark" },
  { id: GalaxyTheme.SYSTEM, label: "System" },
];

const memberDisplayName = (member: { name?: string; email?: string }): string =>
  member.name || member.email || "Member";

interface MainLayoutSettingsButtonMenuProps {
  name: string;
  email?: string;
  avatarUrl?: string;
  seed?: string;
  role?: Role;
  canManageTeam: boolean;
  isTeamActionsPending: boolean;
  onOpenTeamSettings: () => void;
  onOpenServiceAccounts: () => void;
  onOpenInvite: () => void;
  onLogout: () => void;
  selectedTheme: GalaxyTheme;
  onThemeChange: (theme: GalaxyTheme) => void;
}

const MainLayoutSettingsButtonMenu = ({
  name,
  email,
  avatarUrl,
  seed,
  role,
  canManageTeam,
  isTeamActionsPending,
  onOpenTeamSettings,
  onOpenServiceAccounts,
  onOpenInvite,
  onLogout,
  selectedTheme,
  onThemeChange,
}: MainLayoutSettingsButtonMenuProps) => {
  const displayRole = role === undefined ? undefined : roleLabel(role);
  const shouldShowInvite = canManageTeam || isTeamActionsPending;

  return (
    <Flex
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.STRETCH}
      overflow="hidden"
      width={260}
    >
      <MenuHeader>
        <GridBackground size={GridBackgroundSize.X_SMALL}>
          <Flex
            direction={FlexDirection.COLUMN}
            alignItems={AlignItems.CENTER}
            gap={12}
            padding={16}
            fillWidth
            minWidth={0}
          >
            <Avatar img={avatarUrl} size={AvatarSize.LARGE} seed={seed} />
            <Flex
              direction={FlexDirection.COLUMN}
              alignItems={AlignItems.CENTER}
              gap={8}
              fillWidth
              minWidth={0}
            >
              <Flex
                direction={FlexDirection.COLUMN}
                alignItems={AlignItems.CENTER}
                gap={2}
                fillWidth
                minWidth={0}
              >
                <Text
                  size={TextSize.BODY_LG}
                  weight={TextWeight.MEDIUM}
                  align="center"
                  lineClamp={1}
                >
                  {name}
                </Text>
                {email && (
                  <Text
                    size={TextSize.BODY_SM}
                    variant={TextVariant.SECONDARY}
                    align="center"
                    lineClamp={1}
                  >
                    {email}
                  </Text>
                )}
              </Flex>
              {displayRole && (
                <Chip label={displayRole} size={ChipSize.SMALL} variant={ChipVariant.SECONDARY} />
              )}
            </Flex>
          </Flex>
        </GridBackground>
      </MenuHeader>
      <Divider />
      <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} padding={4} fillWidth>
        <MenuItem type="button" onClick={onOpenTeamSettings}>
          <Flex
            width={16}
            height={16}
            shrink={0}
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.CENTER}
          >
            <Icon component={UsersThreeIcon} variant={IconVariant.TERTIARY} />
          </Flex>
          <Text>Manage organization</Text>
        </MenuItem>
        {canManageTeam && (
          <MenuItem type="button" onClick={onOpenServiceAccounts}>
            <Flex
              width={16}
              height={16}
              shrink={0}
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.CENTER}
            >
              <Icon component={WrenchIcon} variant={IconVariant.TERTIARY} />
            </Flex>
            <Text>Service accounts</Text>
          </MenuItem>
        )}
      </Flex>
      {shouldShowInvite && (
        <>
          <Divider />
          <Flex
            direction={FlexDirection.COLUMN}
            alignItems={AlignItems.STRETCH}
            padding={4}
            fillWidth
          >
            <MenuItem type="button" onClick={onOpenInvite} disabled={isTeamActionsPending}>
              <Flex
                width={16}
                height={16}
                shrink={0}
                alignItems={AlignItems.CENTER}
                justifyContent={JustifyContent.CENTER}
              >
                <Icon component={PlusIcon} variant={IconVariant.TERTIARY} />
              </Flex>
              <Text weight={TextWeight.MEDIUM}>Invite team member</Text>
            </MenuItem>
          </Flex>
        </>
      )}
      <Divider />
      <Flex
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        gap={8}
        padding={4}
        fillWidth
      >
        <FlexItem shrink={0} minWidth={0}>
          <ToggleInput
            options={MAIN_LAYOUT_SETTINGS_THEME_OPTIONS}
            value={selectedTheme}
            onChange={onThemeChange}
            size={ToggleInputSize.SMALL}
          />
        </FlexItem>
        <Button
          label="Logout"
          icon={SignOutIcon}
          variant={ButtonVariant.TERTIARY}
          size={ButtonSize.SMALL}
          onClick={onLogout}
        />
      </Flex>
    </Flex>
  );
};

const AuthenticatedSettingsButton = ({ session }: { session: AppSession }) => {
  const navigate = useNavigate();
  const signOut = useSignOut();
  const { selectedTheme, setTheme } = useGalaxyTheme();
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
  const profileEmail = currentMember?.email || session.email;

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
          email={profileEmail}
          avatarUrl={session.avatarUrl}
          seed={session.userId}
          role={currentMember?.role}
          canManageTeam={canManageTeam}
          isTeamActionsPending={isTeamActionsPending}
          onOpenTeamSettings={handleOpenTeamSettings}
          onOpenServiceAccounts={handleOpenServiceAccounts}
          onOpenInvite={handleOpenInvite}
          onLogout={handleLogout}
          selectedTheme={selectedTheme}
          onThemeChange={setTheme}
        />
      }
    >
      <AvatarButton type="button" title="Account settings">
        <Avatar img={session.avatarUrl} size={26} seed={session.userId} />
      </AvatarButton>
    </Popover>
  );
};

const MainLayoutSettingsButton = () => {
  const { session } = useRouteContext({ from: "/_app" });

  if (!session.isAuthenticated) {
    return null;
  }

  return <AuthenticatedSettingsButton session={session} />;
};

export default MainLayoutSettingsButton;
