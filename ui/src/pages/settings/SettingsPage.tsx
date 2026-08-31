import { useCallback, useEffect, useMemo, useState } from "react";

import { styled } from "@linaria/react";
import {
  ArrowLeftIcon,
  ArrowSquareOutIcon,
  BookOpenIcon,
  PaletteIcon,
  ShieldStarIcon,
  SignOutIcon,
  SlackLogoIcon,
  UsersThreeIcon,
} from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useAuth } from "react-oidc-context";

import Avatar from "@galaxy-io/dls/avatar/Avatar";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";
import Modal from "@galaxy-io/dls/modal/Modal";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { useGalaxyTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme, type PropsWithTheme } from "@galaxy-io/dls/theme/types";

import ServiceAccountsModal from "@/components/settings/ServiceAccountsModal";
import TeamSettingsModal from "@/components/settings/TeamSettingsModal";
import { SettingsPanel, TeamSettingsView } from "@/components/settings/types";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import { useListMembersQuery } from "@/api/queries/auth";

import { DOCUMENTATION_URL, SLACK_COMMUNITY_URL } from "@/constants";

import { useAppSession } from "@/auth/session";

const PageWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  padding: 12px;
  background-color: ${({ theme }) => theme.color.background.tertiary};
`);

const PageSurface = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;
  min-height: 0;
  padding: 48px 0;
  overflow: auto;
  background-color: ${({ theme }) => theme.color.background.base};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 8px;
`);

const SettingsLayout = styled.div`
  display: grid;
  grid-template-columns: 180px minmax(0, 720px);
  gap: 48px;
  width: 948px;
  max-width: calc(100% - 48px);
  margin: 0 auto;

  @media (max-width: 820px) {
    grid-template-columns: 1fr;
    gap: 32px;
    width: auto;
  }
`;

const SettingsSidebar = styled.aside`
  position: sticky;
  top: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  height: fit-content;

  @media (max-width: 820px) {
    position: static;
  }
`;

const AccountSummary = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-width: 0;
  padding: 6px 8px;
`;

const NavigationGroups = styled.div`
  display: flex;
  flex-direction: column;
  gap: 16px;
`;

const NavigationGroup = styled.div`
  display: flex;
  flex-direction: column;
`;

const NavigationGroupTitle = styled.div`
  padding: 0 8px 8px;
`;

const SettingsNavigation = styled.nav`
  display: flex;
  flex-direction: column;
  gap: 4px;
`;

const SettingsNavigationItem = withTheme(
  styled.button<PropsWithTheme<{ $isActive?: boolean }>>`
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    min-width: 0;
    min-height: 32px;
    padding: 6px 8px;
    color: ${({ theme, $isActive }) =>
      $isActive ? theme.color.text.primary : theme.color.text.secondary};
    background-color: ${({ theme, $isActive }) =>
      $isActive ? theme.color.background.tertiary : "transparent"};
    border: 0.5px solid
      ${({ theme, $isActive }) => ($isActive ? theme.color.border.primary : "transparent")};
    border-radius: 5px;
    text-align: left;
    cursor: pointer;

    &:hover,
    &:focus-visible {
      color: ${({ theme }) => theme.color.text.primary};
      background-color: ${({ theme }) => theme.color.background.tertiary};
    }

    &:focus-visible {
      outline: 1px solid ${({ theme }) => theme.color.border.secondary};
      outline-offset: 1px;
    }
  `,
);

const SettingsNavigationLink = withTheme(styled.a<PropsWithTheme>`
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  min-width: 0;
  min-height: 32px;
  padding: 6px 8px;
  color: ${({ theme }) => theme.color.text.secondary};
  background-color: transparent;
  border: 0.5px solid transparent;
  border-radius: 5px;
  text-align: left;
  text-decoration: none;
  cursor: pointer;

  &:hover,
  &:focus-visible {
    color: ${({ theme }) => theme.color.text.primary};
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }

  &:focus-visible {
    outline: 1px solid ${({ theme }) => theme.color.border.secondary};
    outline-offset: 1px;
  }
`);

const NavigationLabel = styled.span`
  flex: 1;
  min-width: 0;
`;

const SettingsContent = styled.main`
  width: 100%;
  min-width: 0;
`;

const PreferencesBody = styled.div`
  display: flex;
  flex-direction: column;
  gap: 24px;
  width: 100%;
  padding-top: 21px;
`;

const PreferenceField = styled.div`
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
`;

const navIconVariant = (isActive: boolean) =>
  isActive ? IconVariant.PRIMARY : IconVariant.TERTIARY;

interface NavigationItemProps {
  label: string;
  icon: typeof UsersThreeIcon;
  isActive?: boolean;
  onClick: () => void;
}

const NavigationItem = ({ label, icon, isActive = false, onClick }: NavigationItemProps) => (
  <SettingsNavigationItem type="button" $isActive={isActive} onClick={onClick}>
    <Icon
      component={icon}
      size={16}
      variant={navIconVariant(isActive)}
      weight={isActive ? IconWeight.FILL : IconWeight.REGULAR}
    />
    <NavigationLabel>
      <Text weight={isActive ? TextWeight.MEDIUM : TextWeight.REGULAR} isEllipsis>
        {label}
      </Text>
    </NavigationLabel>
  </SettingsNavigationItem>
);

const ExternalNavigationItem = ({
  label,
  icon,
  href,
}: {
  label: string;
  icon: typeof BookOpenIcon;
  href: string;
}) => (
  <SettingsNavigationLink href={href} target="_blank" rel="noreferrer">
    <Icon component={icon} size={16} variant={IconVariant.TERTIARY} />
    <NavigationLabel>
      <Text isEllipsis>{label}</Text>
    </NavigationLabel>
    <Icon component={ArrowSquareOutIcon} size={13} variant={IconVariant.TERTIARY} />
  </SettingsNavigationLink>
);

const PreferencesPanel = () => {
  const { selectedTheme, setTheme } = useGalaxyTheme();
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

  return (
    <>
      <BaseHeader
        title="Preferences"
        description="Customize your experience with theme and display settings"
        size={BaseHeaderSize.LARGE}
      />
      <PreferencesBody>
        <PreferenceField>
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            Theme
          </Text>
          <SwitcherInput items={themeItems} selectedId={selectedTheme} size={InputSize.SMALL} />
        </PreferenceField>
      </PreferencesBody>
    </>
  );
};

const SettingsPage = () => {
  const session = useAppSession();
  const auth = useAuth();
  const navigate = useNavigate();
  const [isServiceAccountCreateOpen, setIsServiceAccountCreateOpen] = useState(false);
  const { settings = SettingsPanel.TEAM, teamView, inviteToken } = useSearch({ from: "__root__" });
  const membersQuery = useListMembersQuery({
    options: { enabled: !!session.accessToken },
  });
  const canManageTeam = membersQuery.data?.canManage;
  const currentMember = membersQuery.data?.members.find(
    (member) => member.userId === session.userId,
  );
  const profileName = currentMember?.name || session.name || session.email || "Account";
  const profileEmail = currentMember?.email || session.email;

  const handlePanelChange = useCallback(
    (panel: SettingsPanel) => {
      void navigate({
        to: "/settings",
        search: (prev) => ({
          ...prev,
          settings: panel,
          teamView: panel === SettingsPanel.TEAM ? TeamSettingsView.MEMBERS : undefined,
          inviteToken: undefined,
        }),
      });
    },
    [navigate],
  );

  const handleTeamViewChange = useCallback(
    (view: TeamSettingsView, options?: { replace?: boolean }) => {
      void navigate({
        to: "/settings",
        replace: options?.replace,
        search: (prev) => {
          const { inviteToken: _, ...rest } = prev;
          return { ...rest, settings: SettingsPanel.TEAM, teamView: view };
        },
      });
    },
    [navigate],
  );

  const handleInviteCreated = useCallback(
    (token: string) => {
      void navigate({
        to: "/settings",
        search: (prev) => ({
          ...prev,
          settings: SettingsPanel.TEAM,
          teamView: TeamSettingsView.LINK,
          inviteToken: token,
        }),
      });
    },
    [navigate],
  );

  const handleInviteClose = useCallback(() => {
    void navigate({
      to: "/settings",
      search: (prev) => {
        const { inviteToken: _, ...rest } = prev;
        return { ...rest, settings: SettingsPanel.TEAM, teamView: TeamSettingsView.MEMBERS };
      },
    });
  }, [navigate]);

  useEffect(() => {
    if (!session.isAuthEnabled || !session.accessToken) {
      void navigate({ to: "/observability", replace: true });
      return;
    }
    if (canManageTeam === false && settings === SettingsPanel.SERVICE_ACCOUNTS) {
      handlePanelChange(SettingsPanel.TEAM);
    }
  }, [canManageTeam, handlePanelChange, navigate, session, settings]);

  if (!session.isAuthEnabled || !session.accessToken) {
    return null;
  }

  return (
    <PageWrapper>
      <PageSurface>
        <SettingsLayout>
          <SettingsSidebar>
            <SettingsNavigationItem
              type="button"
              onClick={() => void navigate({ to: "/observability" })}
            >
              <Icon component={ArrowLeftIcon} size={16} variant={IconVariant.TERTIARY} />
              <NavigationLabel>
                <Text>Back to app</Text>
              </NavigationLabel>
            </SettingsNavigationItem>

            <AccountSummary>
              <Avatar img={session.avatarUrl} size={24} seed={profileEmail || profileName} />
              <FlexWrapper direction={FlexDirection.COLUMN} gap={1} minWidth={0}>
                <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} isEllipsis>
                  {profileName}
                </Text>
                {profileEmail && (
                  <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} isEllipsis>
                    {profileEmail}
                  </Text>
                )}
              </FlexWrapper>
            </AccountSummary>

            <NavigationGroups>
              <NavigationGroup>
                <NavigationGroupTitle>
                  <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                    Workspace
                  </Text>
                </NavigationGroupTitle>
                <SettingsNavigation aria-label="Workspace settings">
                  <NavigationItem
                    label="Team"
                    icon={UsersThreeIcon}
                    isActive={settings === SettingsPanel.TEAM}
                    onClick={() => handlePanelChange(SettingsPanel.TEAM)}
                  />
                  {canManageTeam !== false && (
                    <NavigationItem
                      label="Service accounts"
                      icon={ShieldStarIcon}
                      isActive={settings === SettingsPanel.SERVICE_ACCOUNTS}
                      onClick={() => handlePanelChange(SettingsPanel.SERVICE_ACCOUNTS)}
                    />
                  )}
                </SettingsNavigation>
              </NavigationGroup>

              <NavigationGroup>
                <NavigationGroupTitle>
                  <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                    User
                  </Text>
                </NavigationGroupTitle>
                <SettingsNavigation aria-label="User settings">
                  <NavigationItem
                    label="Preferences"
                    icon={PaletteIcon}
                    isActive={settings === SettingsPanel.PREFERENCES}
                    onClick={() => handlePanelChange(SettingsPanel.PREFERENCES)}
                  />
                </SettingsNavigation>
              </NavigationGroup>

              <NavigationGroup>
                <NavigationGroupTitle>
                  <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                    Resources
                  </Text>
                </NavigationGroupTitle>
                <SettingsNavigation aria-label="Resources">
                  <ExternalNavigationItem
                    label="Documentation"
                    icon={BookOpenIcon}
                    href={DOCUMENTATION_URL}
                  />
                  <ExternalNavigationItem
                    label="Slack community"
                    icon={SlackLogoIcon}
                    href={SLACK_COMMUNITY_URL}
                  />
                </SettingsNavigation>
              </NavigationGroup>

              <SettingsNavigation aria-label="Account actions">
                <SettingsNavigationItem type="button" onClick={() => void auth.signoutRedirect()}>
                  <Icon component={SignOutIcon} size={16} variant={IconVariant.TERTIARY} />
                  <NavigationLabel>
                    <Text>Logout</Text>
                  </NavigationLabel>
                </SettingsNavigationItem>
              </SettingsNavigation>
            </NavigationGroups>
          </SettingsSidebar>

          <SettingsContent>
            {settings === SettingsPanel.PREFERENCES ? (
              <PreferencesPanel />
            ) : settings === SettingsPanel.SERVICE_ACCOUNTS && canManageTeam !== false ? (
              <ServiceAccountsModal
                session={session}
                onCreate={() => setIsServiceAccountCreateOpen(true)}
                isPanel
              />
            ) : (
              <TeamSettingsModal
                session={session}
                view={TeamSettingsView.MEMBERS}
                onViewChange={handleTeamViewChange}
                onInviteCreated={handleInviteCreated}
                isPanel
              />
            )}
          </SettingsContent>
        </SettingsLayout>
      </PageSurface>
      <Modal open={isServiceAccountCreateOpen} onClose={() => setIsServiceAccountCreateOpen(false)}>
        <ServiceAccountsModal
          session={session}
          defaultToCreate
          onClose={() => setIsServiceAccountCreateOpen(false)}
        />
      </Modal>
      <Modal
        open={
          settings === SettingsPanel.TEAM && !!teamView && teamView !== TeamSettingsView.MEMBERS
        }
        onClose={handleInviteClose}
      >
        {teamView && teamView !== TeamSettingsView.MEMBERS && (
          <TeamSettingsModal
            session={session}
            view={teamView}
            inviteToken={inviteToken}
            onViewChange={handleTeamViewChange}
            onInviteCreated={handleInviteCreated}
            onClose={handleInviteClose}
          />
        )}
      </Modal>
    </PageWrapper>
  );
};

export default SettingsPage;
