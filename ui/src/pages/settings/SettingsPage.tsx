import { type FC, useCallback, useEffect } from "react";

import { useRouteContext } from "@tanstack/react-router";
import { match } from "ts-pattern";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection, FlexVariant } from "@galaxy-io/dls/layout/Flex";
import Modal, { ModalSize } from "@galaxy-io/dls/modal/Modal";
import { Orientation, Radius } from "@galaxy-io/dls/theme/enums";

import SettingsPreferencesPanel from "@/pages/settings/panels/preferences/SettingsPreferencesPanel";
import SettingsServiceAccountsPanel from "@/pages/settings/panels/service-accounts/SettingsServiceAccountsPanel";
import SettingsTeamPanel from "@/pages/settings/panels/team/SettingsTeamPanel";
import SettingsTeamPanelInvite from "@/pages/settings/panels/team/SettingsTeamPanelInvite";
import SettingsPageSidebar from "@/pages/settings/SettingsPageSidebar";
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
  onInviteTeam: () => void;
}

const SettingsPageContent: FC<SettingsPageContentProps> = ({
  session,
  panel,
  isOpen,
  onInviteTeam,
}) => {
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
      .with(SettingsPanel.TEAM, () => (
        <SettingsTeamPanel session={session} onInvite={onInviteTeam} />
      ))
      .with(SettingsPanel.SERVICE_ACCOUNTS, () => (
        <SettingsServiceAccountsPanel session={session} />
      ))
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
  const { settings, teamView, inviteToken } = useSettingsSearch();

  const isSettingsOpen = session.isAuthenticated && flow === Flow.SETTINGS;
  const settingsPanel = settings ?? SettingsPanel.TEAM;
  const isTeamViewOpen =
    isSettingsOpen &&
    (settings ?? SettingsPanel.TEAM) === SettingsPanel.TEAM &&
    !!teamView &&
    teamView !== TeamSettingsView.MEMBERS;

  const handleCloseSettings = useCallback(() => {
    void updateSearch((prev) => {
      const { flow: _, settings: __, teamView: ___, inviteToken: ____, ...rest } = prev;
      return rest;
    });
  }, [updateSearch]);

  const handleTeamViewChange = useCallback(
    (view: TeamSettingsView, options?: { replace?: boolean }) => {
      void updateSearch(
        (prev) => {
          const { inviteToken: _, ...rest } = prev;
          return { ...rest, settings: SettingsPanel.TEAM, teamView: view };
        },
        { replace: options?.replace },
      );
    },
    [updateSearch],
  );

  const handleInviteTeam = useCallback(() => {
    handleTeamViewChange(TeamSettingsView.INVITE);
  }, [handleTeamViewChange]);

  const handleInviteCreated = useCallback(
    (token: string) => {
      void updateSearch((prev) => ({
        ...prev,
        settings: SettingsPanel.TEAM,
        teamView: TeamSettingsView.LINK,
        inviteToken: token,
      }));
    },
    [updateSearch],
  );

  const handleInviteClose = useCallback(() => {
    void updateSearch((prev) => {
      const { inviteToken: _, ...rest } = prev;
      return { ...rest, settings: SettingsPanel.TEAM, teamView: TeamSettingsView.MEMBERS };
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
        <SettingsPageContent
          session={session}
          panel={settingsPanel}
          isOpen={isSettingsOpen}
          onInviteTeam={handleInviteTeam}
        />
      </Modal>
      {isTeamViewOpen && teamView && (
        <SettingsTeamPanelInvite
          open
          session={session}
          view={teamView}
          inviteToken={inviteToken}
          onViewChange={handleTeamViewChange}
          onInviteCreated={handleInviteCreated}
          onClose={handleInviteClose}
        />
      )}
    </>
  );
};

export default SettingsPage;
