import type { FC } from "react";

import {
  BookOpenIcon,
  GithubLogoIcon,
  PaletteIcon,
  SignOutIcon,
  SlackLogoIcon,
  UsersThreeIcon,
  WrenchIcon,
} from "@phosphor-icons/react";

import Avatar, { AvatarSize } from "@galaxy-io/dls/avatar/Avatar";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import SidebarNav, { NavGroup, NavItem } from "@galaxy-io/dls/navigation/SidebarNav";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import { SETTINGS_PAGE_SIDEBAR_WIDTH } from "@/pages/settings/constants";
import { SettingsPanel } from "@/pages/settings/types";

import { useListMembersQuery } from "@/api/queries/auth";

import { DOCUMENTATION_URL, GITHUB_REPO_URL, SLACK_COMMUNITY_URL } from "@/constants";

import { useSignOut } from "@/auth/hooks/useSignOut";
import type { AppSession } from "@/auth/types";

interface SettingsPageSidebarProps {
  session: AppSession;
  activePanel: SettingsPanel;
  canManageTeam: boolean | undefined;
  onPanelChange: (panel: SettingsPanel) => void;
}

const SettingsPageSidebar: FC<SettingsPageSidebarProps> = ({
  session,
  activePanel,
  canManageTeam,
  onPanelChange,
}) => {
  const signOut = useSignOut();
  const membersQuery = useListMembersQuery({
    options: { enabled: session.isAuthenticated },
  });
  const currentMember = membersQuery.data?.members.find(
    (member) => member.userId === session.userId,
  );
  const profileName = currentMember?.name || session.name || session.email || "Account";
  const profileEmail = currentMember?.email || session.email;

  return (
    <Flex
      direction={FlexDirection.COLUMN}
      alignItems={AlignItems.STRETCH}
      width={SETTINGS_PAGE_SIDEBAR_WIDTH}
      shrink={0}
    >
      <SidebarNav
        ariaLabel="Settings"
        header={
          <Flex alignItems={AlignItems.CENTER} gap={8} fillWidth minWidth={0}>
            <Avatar
              img={session.avatarUrl}
              size={AvatarSize.SMALL}
              seed={session.userId}
              name={profileName}
              isSquare
            />
            <FlexItem grow={1} minWidth={0}>
              <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN}>
                <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} lineClamp={1}>
                  {profileName}
                </Text>
                {profileEmail && (
                  <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} lineClamp={1}>
                    {profileEmail}
                  </Text>
                )}
              </Flex>
            </FlexItem>
          </Flex>
        }
        footer={<NavItem label="Log out" icon={SignOutIcon} onClick={() => void signOut()} />}
      >
        <NavGroup label="Workspace">
          <NavItem
            label="Team"
            icon={UsersThreeIcon}
            isActive={activePanel === SettingsPanel.TEAM}
            onClick={() => onPanelChange(SettingsPanel.TEAM)}
          />
          {canManageTeam !== false && (
            <NavItem
              label="Service accounts"
              icon={WrenchIcon}
              isActive={activePanel === SettingsPanel.SERVICE_ACCOUNTS}
              onClick={() => onPanelChange(SettingsPanel.SERVICE_ACCOUNTS)}
            />
          )}
        </NavGroup>
        <NavGroup label="User">
          <NavItem
            label="Preferences"
            icon={PaletteIcon}
            isActive={activePanel === SettingsPanel.PREFERENCES}
            onClick={() => onPanelChange(SettingsPanel.PREFERENCES)}
          />
        </NavGroup>
        <NavGroup label="Resources">
          <NavItem label="Documentation" icon={BookOpenIcon} href={DOCUMENTATION_URL} isExternal />
          <NavItem label="GitHub" icon={GithubLogoIcon} href={GITHUB_REPO_URL} isExternal />
          <NavItem
            label="Slack community"
            icon={SlackLogoIcon}
            href={SLACK_COMMUNITY_URL}
            isExternal
          />
        </NavGroup>
      </SidebarNav>
    </Flex>
  );
};

export default SettingsPageSidebar;
