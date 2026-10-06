import {
  BookOpenIcon,
  PaletteIcon,
  SignOutIcon,
  SlackLogoIcon,
  UsersThreeIcon,
  WrenchIcon,
} from "@phosphor-icons/react";

import Avatar from "@galaxy-io/dls/avatar/Avatar";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import SettingsNavigationGroup from "@/pages/settings/components/SettingsNavigationGroup";
import SettingsNavigationItem from "@/pages/settings/components/SettingsNavigationItem";
import SettingsNavigationLink from "@/pages/settings/components/SettingsNavigationLink";
import { SETTINGS_PAGE_SIDEBAR_WIDTH } from "@/pages/settings/constants";
import { SettingsPanel } from "@/pages/settings/types";

import { useListMembersQuery } from "@/api/queries/auth";

import { DOCUMENTATION_URL, SLACK_COMMUNITY_URL } from "@/constants";

import { useSignOut } from "@/auth/hooks/useSignOut";
import type { AppSession } from "@/auth/types";

interface SettingsPageSidebarProps {
  session: AppSession;
  activePanel: SettingsPanel;
  canManageTeam: boolean | undefined;
  onPanelChange: (panel: SettingsPanel) => void;
}

const SettingsPageSidebar = ({
  session,
  activePanel,
  canManageTeam,
  onPanelChange,
}: SettingsPageSidebarProps) => {
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
      gap={12}
      width={SETTINGS_PAGE_SIDEBAR_WIDTH}
      shrink={0}
      padding={12}
      overflow="auto"
    >
      <Flex alignItems={AlignItems.CENTER} gap={8} padding={[4, 8]} fillWidth minWidth={0}>
        <Avatar img={session.avatarUrl} size={26} seed={session.userId} />
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={0} minWidth={0}>
          <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} lineClamp={1}>
            {profileName}
          </Text>
          {profileEmail && (
            <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} lineClamp={1}>
              {profileEmail}
            </Text>
          )}
        </Flex>
      </Flex>

      <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} gap={16} fillWidth>
        <SettingsNavigationGroup title="Workspace">
          <SettingsNavigationItem
            label="Team"
            icon={UsersThreeIcon}
            isActive={activePanel === SettingsPanel.TEAM}
            onClick={() => onPanelChange(SettingsPanel.TEAM)}
          />
          {canManageTeam !== false && (
            <SettingsNavigationItem
              label="Service accounts"
              icon={WrenchIcon}
              isActive={activePanel === SettingsPanel.SERVICE_ACCOUNTS}
              onClick={() => onPanelChange(SettingsPanel.SERVICE_ACCOUNTS)}
            />
          )}
        </SettingsNavigationGroup>

        <SettingsNavigationGroup title="User">
          <SettingsNavigationItem
            label="Preferences"
            icon={PaletteIcon}
            isActive={activePanel === SettingsPanel.PREFERENCES}
            onClick={() => onPanelChange(SettingsPanel.PREFERENCES)}
          />
        </SettingsNavigationGroup>

        <SettingsNavigationGroup title="Resources">
          <SettingsNavigationLink
            label="Documentation"
            icon={BookOpenIcon}
            href={DOCUMENTATION_URL}
          />
          <SettingsNavigationLink
            label="Slack community"
            icon={SlackLogoIcon}
            href={SLACK_COMMUNITY_URL}
          />
        </SettingsNavigationGroup>

        <SettingsNavigationItem label="Logout" icon={SignOutIcon} onClick={() => void signOut()} />
      </Flex>
    </Flex>
  );
};

export default SettingsPageSidebar;
