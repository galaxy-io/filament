import { useCallback, useMemo, useState } from "react";

import { styled } from "@linaria/react";
import { PlusIcon, ShieldStarIcon, SignOutIcon, UsersThreeIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";
import { useAuth } from "react-oidc-context";

import Avatar from "@galaxy-io/dls/avatar/Avatar";
import DotGridBackground from "@galaxy-io/dls/backgrounds/DotGridBackground";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { useGalaxyTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme, type PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { Role } from "@/gen/auth/v1/members_pb";

import { roleLabel } from "@/components/settings/constants";
import { SettingsPanel, TeamSettingsView } from "@/components/settings/types";

import { useListMembersQuery } from "@/api/queries/auth";

import { type AppSession, useAppSession } from "@/auth/session";

const SETTINGS_DROPDOWN_ID = "settings-dropdown-menu";

const SettingsMenuWrapper = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  overflow: hidden;
`;

const SettingsMenuHeader = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  background-color: ${({ theme }) => theme.color.background.primary};
  border-radius: 5px 5px 0 0;
  overflow: hidden;
`);

const SettingsMenuHeaderContent = styled.div`
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  width: 100%;
  min-width: 0;
  padding: 16px;
`;

const SettingsMenuSection = styled.div`
  display: flex;
  flex-direction: column;
  width: 100%;
  padding: 6px;
`;

const SettingsMenuFooter = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 6px;
  width: 100%;
  padding: 6px;
`;

const ThemeSwitcherWrapper = styled.div`
  min-width: 0;
  flex-shrink: 0;
`;

const SettingsAvatarButton = withTheme(styled.button<PropsWithTheme>`
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
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

const SettingsMenuItem = withTheme(styled.button<PropsWithTheme>`
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

const SettingsMenuIconWrapper = styled.div`
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  width: 16px;
  height: 16px;
`;

const memberDisplayName = (member: { name?: string; email?: string }): string =>
  member.name || member.email || "Member";

interface SettingsDropdownMenuProps {
  name: string;
  email?: string;
  avatarUrl?: string;
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

const SettingsDropdownMenu = ({
  name,
  email,
  avatarUrl,
  role,
  canManageTeam,
  isTeamActionsPending,
  onOpenTeamSettings,
  onOpenServiceAccounts,
  onOpenInvite,
  onLogout,
  themeItems,
  selectedTheme,
}: SettingsDropdownMenuProps) => {
  const { theme } = useGalaxyTheme();
  const displayRole = roleLabel(role);
  const shouldShowInvite = canManageTeam || isTeamActionsPending;

  return (
    <SettingsMenuWrapper id={SETTINGS_DROPDOWN_ID}>
      <SettingsMenuHeader>
        <DotGridBackground
          dotSize={2}
          dotOpacity={0.25}
          spacing={8}
          backgroundColor={theme.color.background.primary}
          fillContainer={false}
        >
          <SettingsMenuHeaderContent>
            <Avatar img={avatarUrl} size={36} seed={email || name} />
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
          </SettingsMenuHeaderContent>
        </DotGridBackground>
      </SettingsMenuHeader>
      <HorizontalDivider />
      <SettingsMenuSection>
        <SettingsMenuItem type="button" onClick={onOpenTeamSettings}>
          <SettingsMenuIconWrapper>
            <Icon component={UsersThreeIcon} size={14} variant={IconVariant.TERTIARY} />
          </SettingsMenuIconWrapper>
          <Text weight={TextWeight.MEDIUM}>Manage team</Text>
        </SettingsMenuItem>
        {canManageTeam && (
          <SettingsMenuItem type="button" onClick={onOpenServiceAccounts}>
            <SettingsMenuIconWrapper>
              <Icon component={ShieldStarIcon} size={14} variant={IconVariant.TERTIARY} />
            </SettingsMenuIconWrapper>
            <Text weight={TextWeight.MEDIUM}>Service accounts</Text>
          </SettingsMenuItem>
        )}
      </SettingsMenuSection>
      {shouldShowInvite && (
        <>
          <HorizontalDivider />
          <SettingsMenuSection>
            <SettingsMenuItem type="button" onClick={onOpenInvite} disabled={isTeamActionsPending}>
              <SettingsMenuIconWrapper>
                <Icon component={PlusIcon} size={14} variant={IconVariant.TERTIARY} />
              </SettingsMenuIconWrapper>
              <Text weight={TextWeight.MEDIUM}>Invite team member</Text>
            </SettingsMenuItem>
          </SettingsMenuSection>
        </>
      )}
      <HorizontalDivider />
      <SettingsMenuFooter>
        <ThemeSwitcherWrapper>
          <SwitcherInput items={themeItems} selectedId={selectedTheme} size={InputSize.SMALL} />
        </ThemeSwitcherWrapper>
        <Button
          label="Logout"
          icon={SignOutIcon}
          variant={ButtonVariant.TERTIARY}
          size={ButtonSize.SMALL}
          onClick={onLogout}
        />
      </SettingsMenuFooter>
    </SettingsMenuWrapper>
  );
};

const AuthenticatedSettingsButton = ({ session }: { session: AppSession }) => {
  const auth = useAuth();
  const navigate = useNavigate();
  const { selectedTheme, setTheme } = useGalaxyTheme();
  const [settingsOpen, setSettingsOpen] = useState(false);

  const membersQuery = useListMembersQuery({
    options: { enabled: settingsOpen && !!session.accessToken },
  });

  const currentMember = membersQuery.data?.members.find(
    (member) => member.userId === session.userId,
  );
  const canManageTeam = membersQuery.data?.canManage ?? false;
  const isTeamActionsPending = membersQuery.isLoading && !membersQuery.data;
  const profileName =
    (currentMember ? memberDisplayName(currentMember) : session.name || session.email) ?? "Member";
  const profileEmail = currentMember?.email || session.email;

  const themeItems = useMemo(
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

  const handleSettingsClose = useCallback(() => {
    setSettingsOpen(false);
  }, []);

  const handleOpenTeamSettings = useCallback(() => {
    setSettingsOpen(false);
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        settings: SettingsPanel.TEAM,
        teamView: TeamSettingsView.MEMBERS,
      }),
    });
  }, [navigate]);

  const handleOpenInvite = useCallback(() => {
    setSettingsOpen(false);
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        settings: SettingsPanel.TEAM,
        teamView: TeamSettingsView.INVITE,
      }),
    });
  }, [navigate]);

  const handleOpenServiceAccounts = useCallback(() => {
    setSettingsOpen(false);
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        settings: SettingsPanel.SERVICE_ACCOUNTS,
      }),
    });
  }, [navigate]);

  const handleLogout = useCallback(() => {
    setSettingsOpen(false);
    void auth.signoutRedirect();
  }, [auth]);

  return (
    <Dropdown
      position={DropdownPosition.BOTTOM_END}
      minWidth={260}
      maxWidth={260}
      noPadding
      isOpen={settingsOpen}
      onClose={handleSettingsClose}
      body={
        <SettingsDropdownMenu
          name={profileName}
          email={profileEmail}
          avatarUrl={session.avatarUrl}
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
      <SettingsAvatarButton
        type="button"
        title="Account settings"
        aria-label="Account settings"
        onClick={() => {
          setSettingsOpen((isOpen) => !isOpen);
        }}
        aria-expanded={settingsOpen}
        aria-controls={SETTINGS_DROPDOWN_ID}
        aria-haspopup="menu"
      >
        <Avatar img={session.avatarUrl} size={20} seed={profileEmail || profileName} />
      </SettingsAvatarButton>
    </Dropdown>
  );
};

const SettingsButton = () => {
  const session = useAppSession();

  if (!session.isAuthEnabled || !session.accessToken) {
    return null;
  }

  return <AuthenticatedSettingsButton session={session} />;
};

export default SettingsButton;
