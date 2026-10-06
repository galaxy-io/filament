import { useCallback, useEffect } from "react";

import { styled } from "@linaria/react";
import { useNavigate, useRouteContext, useSearch } from "@tanstack/react-router";
import { match } from "ts-pattern";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Modal, { ModalSize } from "@galaxy-io/dls/modal/Modal";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { Flow } from "@/layouts/app/types";

import SettingsPreferencesPanel from "@/pages/settings/panels/preferences/SettingsPreferencesPanel";
import SettingsServiceAccountsPanel from "@/pages/settings/panels/service-accounts/SettingsServiceAccountsPanel";
import SettingsTeamPanel from "@/pages/settings/panels/team/SettingsTeamPanel";
import SettingsTeamPanelInvite from "@/pages/settings/panels/team/SettingsTeamPanelInvite";
import SettingsPageSidebar from "@/pages/settings/SettingsPageSidebar";
import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";

import { useListMembersQuery } from "@/api/queries/auth";

import { useRetainedWhileClosed } from "@/hooks/useRetainedWhileClosed";

import type { AppSession } from "@/auth/types";

const FrameWrapper = styled.div`
  display: flex;
  height: 100%;
  min-height: 0;

  background-color: ${t.color.background.primary};
  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};
  overflow: hidden;
`;

const SidebarWrapper = styled.div`
  display: flex;
  flex-shrink: 0;

  background-color: ${t.color.background.primary};
  border-right: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
`;

const ContentWrapper = styled.div`
  display: flex;
  flex-direction: column;

  flex: 1;
  min-width: 0;
  overflow: hidden;
  background-color: ${t.color.background.base};
`;

interface SettingsPageContentProps {
  session: AppSession;
  panel: SettingsPanel;
  isOpen: boolean;
  onInviteTeam: () => void;
}

const SettingsPageContent = ({
  session,
  panel,
  isOpen,
  onInviteTeam,
}: SettingsPageContentProps) => {
  const navigate = useNavigate();
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
    <FrameWrapper>
      <SidebarWrapper>
        <SettingsPageSidebar
          session={session}
          activePanel={activePanel}
          canManageTeam={canManageTeam}
          onPanelChange={handlePanelChange}
        />
      </SidebarWrapper>
      <ContentWrapper>{renderPanel()}</ContentWrapper>
    </FrameWrapper>
  );
};

const SettingsPage = () => {
  const navigate = useNavigate();
  const { session } = useRouteContext({ from: "/_app" });
  const { flow, settings, teamView, inviteToken } = useSearch({ from: "/_app" });

  const isSettingsOpen = session.isAuthenticated && flow === Flow.SETTINGS;
  const settingsPanel = useRetainedWhileClosed(settings ?? SettingsPanel.TEAM, isSettingsOpen);
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
