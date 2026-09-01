import { useCallback, useMemo, useState } from "react";

import { styled } from "@linaria/react";
import { PlusIcon, SignOutIcon, UsersThreeIcon, WrenchIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Avatar from "@galaxy-io/dls/avatar/Avatar";
import DotGridBackground from "@galaxy-io/dls/backgrounds/DotGridBackground";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { useGalaxyTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme, type PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { Role } from "@/gen/auth/v1/members_pb";

import { Flow } from "@/layouts/app/types";

import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";
import { roleLabel } from "@/pages/settings/utils";

import { useListMembersQuery } from "@/api/queries/auth";

import { useSignOut } from "@/auth/hooks/useSignOut";
import { useAppSession } from "@/auth/session";
import type { AppSession } from "@/auth/types";

const MenuHeader = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  background-color: ${({ theme }) => theme.color.background.primary};
  border-radius: 5px 5px 0 0;
  overflow: hidden;
`);

const AvatarButton = withTheme(styled.button<PropsWithTheme>`
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
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }

  &:focus-visible {
    outline: 1px solid ${({ theme }) => theme.color.border.secondary};
    outline-offset: 2px;
  }
`);

const MenuItem = withTheme(styled.button<PropsWithTheme>`
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
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }

  &:focus-visible {
    outline: 1px solid ${({ theme }) => theme.color.border.secondary};
    outline-offset: 1px;
  }

  &:disabled {
    opacity: 0.55;
    cursor: default;
  }

  &:disabled:hover {
    background-color: transparent;
  }
`);

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
  themeItems: SwitcherInputItem[];
  selectedTheme: GalaxyTheme;
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
  themeItems,
  selectedTheme,
}: MainLayoutSettingsButtonMenuProps) => {
  const { theme } = useGalaxyTheme();
  const displayRole = role === undefined ? undefined : roleLabel(role);
  const shouldShowInvite = canManageTeam || isTeamActionsPending;

  return (
    <FlexWrapper
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.STRETCH}
      overflow="hidden"
      fillWidth
    >
      <MenuHeader>
        <DotGridBackground
          dotSize={2}
          dotOpacity={0.25}
          spacing={8}
          backgroundColor={theme.color.background.primary}
          fillContainer={false}
        >
          <FlexWrapper
            direction={FlexDirection.COLUMN}
            alignItems={AlignItems.CENTER}
            gap={FlexGap.MEDIUM}
            padding="16px"
            fillWidth
            minWidth={0}
          >
            <Avatar img={avatarUrl} size={36} seed={seed} />
            <FlexWrapper
              direction={FlexDirection.COLUMN}
              alignItems={AlignItems.CENTER}
              gap={8}
              fillWidth
              minWidth={0}
            >
              <FlexWrapper
                direction={FlexDirection.COLUMN}
                alignItems={AlignItems.CENTER}
                gap={2}
                fillWidth
                minWidth={0}
              >
                <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM} align="center" isEllipsis>
                  {name}
                </Text>
                {email && (
                  <Text
                    size={TextSize.BODY_SM}
                    variant={TextVariant.SECONDARY}
                    align="center"
                    isEllipsis
                  >
                    {email}
                  </Text>
                )}
              </FlexWrapper>
              {displayRole && (
                <Chip label={displayRole} size={ChipSize.SMALL} variant={ChipVariant.SECONDARY} />
              )}
            </FlexWrapper>
          </FlexWrapper>
        </DotGridBackground>
      </MenuHeader>
      <HorizontalDivider />
      <FlexWrapper
        direction={FlexDirection.COLUMN}
        alignItems={AlignItems.STRETCH}
        padding="6px"
        fillWidth
      >
        <MenuItem type="button" onClick={onOpenTeamSettings}>
          <FlexItem
            width={16}
            height={16}
            shrink={0}
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.CENTER}
          >
            <Icon component={UsersThreeIcon} size={14} variant={IconVariant.TERTIARY} />
          </FlexItem>
          <Text weight={TextWeight.MEDIUM}>Manage team</Text>
        </MenuItem>
        {canManageTeam && (
          <MenuItem type="button" onClick={onOpenServiceAccounts}>
            <FlexItem
              width={16}
              height={16}
              shrink={0}
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.CENTER}
            >
              <Icon component={WrenchIcon} size={14} variant={IconVariant.TERTIARY} />
            </FlexItem>
            <Text weight={TextWeight.MEDIUM}>Service accounts</Text>
          </MenuItem>
        )}
      </FlexWrapper>
      {shouldShowInvite && (
        <>
          <HorizontalDivider />
          <FlexWrapper
            direction={FlexDirection.COLUMN}
            alignItems={AlignItems.STRETCH}
            padding="6px"
            fillWidth
          >
            <MenuItem type="button" onClick={onOpenInvite} disabled={isTeamActionsPending}>
              <FlexItem
                width={16}
                height={16}
                shrink={0}
                alignItems={AlignItems.CENTER}
                justifyContent={JustifyContent.CENTER}
              >
                <Icon component={PlusIcon} size={14} variant={IconVariant.TERTIARY} />
              </FlexItem>
              <Text weight={TextWeight.MEDIUM}>Invite team member</Text>
            </MenuItem>
          </FlexWrapper>
        </>
      )}
      <HorizontalDivider />
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        gap={6}
        padding="6px"
        fillWidth
      >
        <FlexItem shrink={0} minWidth={0}>
          <SwitcherInput items={themeItems} selectedId={selectedTheme} size={InputSize.SMALL} />
        </FlexItem>
        <Button
          label="Logout"
          icon={SignOutIcon}
          variant={ButtonVariant.TERTIARY}
          size={ButtonSize.SMALL}
          onClick={onLogout}
        />
      </FlexWrapper>
    </FlexWrapper>
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

  const themeItems = useMemo<SwitcherInputItem[]>(
    () => [
      {
        id: GalaxyTheme.LIGHT,
        label: "Light",
        onClick: () => setTheme(GalaxyTheme.LIGHT),
      },
      {
        id: GalaxyTheme.DARK,
        label: "Dark",
        onClick: () => setTheme(GalaxyTheme.DARK),
      },
      {
        id: GalaxyTheme.SYSTEM,
        label: "System",
        onClick: () => setTheme(GalaxyTheme.SYSTEM),
      },
    ],
    [setTheme],
  );

  const handleClose = useCallback(() => {
    setIsOpen(false);
  }, []);

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
    <Dropdown
      position={DropdownPosition.BOTTOM_END}
      minWidth={260}
      maxWidth={260}
      noPadding
      isOpen={isOpen}
      onClose={handleClose}
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
          themeItems={themeItems}
          selectedTheme={selectedTheme}
        />
      }
    >
      <AvatarButton
        type="button"
        title="Account settings"
        onClick={() => {
          setIsOpen((isDropdownOpen) => !isDropdownOpen);
        }}
      >
        <Avatar img={session.avatarUrl} size={26} seed={session.userId} />
      </AvatarButton>
    </Dropdown>
  );
};

const MainLayoutSettingsButton = () => {
  const session = useAppSession();

  if (!session.isAuthenticated) {
    return null;
  }

  return <AuthenticatedSettingsButton session={session} />;
};

export default MainLayoutSettingsButton;
