import { useCallback, useEffect } from "react";

import { useNavigate, useRouteContext, useSearch } from "@tanstack/react-router";
import { match } from "ts-pattern";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Modal, { ModalSize } from "@galaxy-io/dls/modal/Modal";
import { Orientation } from "@galaxy-io/dls/theme/enums";

import { Flow } from "@/layouts/app/types";

import SettingsPreferencesPanel from "@/pages/settings/panels/preferences/SettingsPreferencesPanel";
import SettingsServiceAccountsPanel from "@/pages/settings/panels/service-accounts/SettingsServiceAccountsPanel";
import SettingsTeamPanel from "@/pages/settings/panels/team/SettingsTeamPanel";
import SettingsTeamPanelInvite from "@/pages/settings/panels/team/SettingsTeamPanelInvite";
import SettingsPageSidebar from "@/pages/settings/SettingsPageSidebar";
import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";

import { useListMembersQuery } from "@/api/queries/auth";

import type { AppSession } from "@/auth/types";

interface SettingsPageContentProps {
  session: AppSession;
  onInviteTeam: () => void;
}

const SettingsPageContent = ({ session, onInviteTeam }: SettingsPageContentProps) => {
  const navigate = useNavigate();
  const { settings = SettingsPanel.TEAM } = useSearch({ from: "/_app" });
  const membersQuery = useListMembersQuery({
    options: { enabled: session.isAuthenticated },
  });
  const canManageTeam = membersQuery.data?.canManage;
  const activePanel =
    settings === SettingsPanel.SERVICE_ACCOUNTS && canManageTeam === false
      ? SettingsPanel.TEAM
      : settings;

  const handlePanelChange = useCallback(
    (panel: SettingsPanel) => {
      void navigate({
        to: ".",
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

  useEffect(() => {
    if (activePanel !== settings) {
      handlePanelChange(activePanel);
    }
  }, [activePanel, handlePanelChange, settings]);

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
    <Flex alignItems={AlignItems.STRETCH} height="100%" minHeight={0}>
      <SettingsPageSidebar
        session={session}
        activePanel={activePanel}
        canManageTeam={canManageTeam}
        onPanelChange={handlePanelChange}
      />
      <Divider orientation={Orientation.VERTICAL} />
      <Flex
        direction={FlexDirection.COLUMN}
        alignItems={AlignItems.STRETCH}
        grow={1}
        basis={0}
        minWidth={0}
        overflow="hidden"
      >
        {renderPanel()}
      </Flex>
    </Flex>
  );
};

const SettingsPage = () => {
  const navigate = useNavigate();
  const { session } = useRouteContext({ from: "/_app" });
  const { flow, settings, teamView, inviteToken } = useSearch({ from: "/_app" });

  const isSettingsOpen = session.isAuthenticated && flow === Flow.SETTINGS;
  const isTeamViewOpen =
    isSettingsOpen &&
    (settings ?? SettingsPanel.TEAM) === SettingsPanel.TEAM &&
    !!teamView &&
    teamView !== TeamSettingsView.MEMBERS;

  const handleCloseSettings = useCallback(() => {
    void navigate({
      to: ".",
      search: (prev) => {
        const { flow: _, settings: __, teamView: ___, inviteToken: ____, ...rest } = prev;
        return rest;
      },
    });
  }, [navigate]);

  const handleTeamViewChange = useCallback(
    (view: TeamSettingsView, options?: { replace?: boolean }) => {
      void navigate({
        to: ".",
        replace: options?.replace,
        search: (prev) => {
          const { inviteToken: _, ...rest } = prev;
          return { ...rest, settings: SettingsPanel.TEAM, teamView: view };
        },
      });
    },
    [navigate],
  );

  const handleInviteTeam = useCallback(() => {
    handleTeamViewChange(TeamSettingsView.INVITE);
  }, [handleTeamViewChange]);

  const handleInviteCreated = useCallback(
    (token: string) => {
      void navigate({
        to: ".",
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
      to: ".",
      search: (prev) => {
        const { inviteToken: _, ...rest } = prev;
        return { ...rest, settings: SettingsPanel.TEAM, teamView: TeamSettingsView.MEMBERS };
      },
    });
  }, [navigate]);

  return (
    <>
      <Modal
        header="Settings"
        size={ModalSize.X_LARGE}
        isOpen={isSettingsOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) handleCloseSettings();
        }}
      >
        <SettingsPageContent session={session} onInviteTeam={handleInviteTeam} />
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
