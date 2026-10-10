import { type FC, useCallback, useEffect } from "react";

import { useRouteContext } from "@tanstack/react-router";
import { match } from "ts-pattern";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection, FlexVariant } from "@galaxy-io/dls/layout/Flex";
import Modal, { ModalSize } from "@galaxy-io/dls/modal/Modal";
import { Orientation, Radius } from "@galaxy-io/dls/theme/enums";

import SettingsPreferencesPanel from "@/pages/settings/panels/preferences/SettingsPreferencesPanel";
import ServiceAccountsPage from "@/pages/settings/ServiceAccountsPage";
import SettingsPageSidebar from "@/pages/settings/SettingsPageSidebar";
import TeamPage from "@/pages/settings/TeamPage";
import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";

import {
  useFilamentLayoutSearch,
  useFilamentSearchUpdate,
  useSettingsSearch,
} from "@/module/hooks";
import type { FilamentLayoutSearch, SettingsSearch } from "@/module/schemas";
import { Flow } from "@/module/types";

import { useListMembersQuery } from "@/api/queries/auth";

import type { AppSession } from "@/auth/types";

interface SettingsPageContentProps {
  session: AppSession;
  panel: SettingsPanel;
  isOpen: boolean;
}

const SettingsPageContent: FC<SettingsPageContentProps> = ({ session, panel, isOpen }) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch & SettingsSearch>();
  const membersQuery = useListMembersQuery({
    options: { enabled: session.isAuthenticated },
  });
  const canManageTeam = membersQuery.data?.canManage;
  const activePanel =
    panel === SettingsPanel.SERVICE_ACCOUNTS && canManageTeam === false
      ? SettingsPanel.TEAM
      : panel;

  const handlePanelChange = useCallback(
    (panel: SettingsPanel) => {
      void updateSearch((prev) => ({
        ...prev,
        settings: panel,
        teamView: panel === SettingsPanel.TEAM ? TeamSettingsView.MEMBERS : undefined,
        inviteToken: undefined,
      }));
    },
    [updateSearch],
  );

  useEffect(() => {
    if (isOpen && activePanel !== panel) {
      handlePanelChange(activePanel);
    }
  }, [activePanel, handlePanelChange, isOpen, panel]);

  const renderPanel = () =>
    match(activePanel)
      .with(SettingsPanel.TEAM, () => <TeamPage />)
      .with(SettingsPanel.SERVICE_ACCOUNTS, () => <ServiceAccountsPage />)
      .with(SettingsPanel.PREFERENCES, () => <SettingsPreferencesPanel />)
      .exhaustive();

  return (
    <Flex
      height="100%"
      minHeight={0}
      variant={FlexVariant.PRIMARY}
      hasBorder
      radius={Radius.LG}
      overflow="hidden"
    >
      <SettingsPageSidebar
        session={session}
        activePanel={activePanel}
        canManageTeam={canManageTeam}
        onPanelChange={handlePanelChange}
      />
      <Divider orientation={Orientation.VERTICAL} />
      <Flex
        direction={FlexDirection.COLUMN}
        grow={1}
        basis={0}
        minWidth={0}
        overflow="hidden"
        variant={FlexVariant.BASE}
      >
        {renderPanel()}
      </Flex>
    </Flex>
  );
};

const SettingsPage: FC = () => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch & SettingsSearch>();
  const { session } = useRouteContext({ from: "/_app" });
  const { flow } = useFilamentLayoutSearch();
  const { settings } = useSettingsSearch();

  const isSettingsOpen = session.isAuthenticated && flow === Flow.SETTINGS;
  const settingsPanel = settings ?? SettingsPanel.TEAM;
  const handleCloseSettings = useCallback(() => {
    void updateSearch((prev) => {
      const { flow: _, settings: __, teamView: ___, inviteToken: ____, ...rest } = prev;
      return rest;
    });
  }, [updateSearch]);

  return (
    <>
      <Modal
        header="Settings"
        size={ModalSize.X_LARGE}
        isOpen={isSettingsOpen}
        footer={
          <Button label="Cancel" variant={ButtonVariant.SECONDARY} onClick={handleCloseSettings} />
        }
        onOpenChange={(isOpen) => {
          if (!isOpen) handleCloseSettings();
        }}
      >
        <SettingsPageContent session={session} panel={settingsPanel} isOpen={isSettingsOpen} />
      </Modal>
    </>
  );
};

export default SettingsPage;
