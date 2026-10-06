import { useCallback, useEffect } from "react";

import { styled } from "@linaria/react";
import { useNavigate, useRouteContext, useSearch } from "@tanstack/react-router";
import { match } from "ts-pattern";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Modal from "@galaxy-io/dls/modal/Modal";
import { Orientation } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { Flow } from "@/layouts/app/types";
import BaseHeader from "@/layouts/components/BaseHeader";

import { SETTINGS_PAGE_INSET } from "@/pages/settings/constants";
import SettingsPreferencesPanel from "@/pages/settings/panels/preferences/SettingsPreferencesPanel";
import SettingsServiceAccountsPanel from "@/pages/settings/panels/service-accounts/SettingsServiceAccountsPanel";
import SettingsTeamPanel from "@/pages/settings/panels/team/SettingsTeamPanel";
import SettingsTeamPanelInvite from "@/pages/settings/panels/team/SettingsTeamPanelInvite";
import SettingsPageSidebar from "@/pages/settings/SettingsPageSidebar";
import { SettingsPanel, TeamSettingsView } from "@/pages/settings/types";

import { useListMembersQuery } from "@/api/queries/auth";

import type { AppSession } from "@/auth/types";

const PageWrapper = styled.div`
  display: flex;
  flex-direction: column;
  width: calc(100vw - ${SETTINGS_PAGE_INSET}px);
  height: calc(100vh - ${SETTINGS_PAGE_INSET}px);
  background-color: ${t.color.background.primary};
  border: 0.5px solid ${t.color.border.primary};
  border-radius: 8px;
  overflow: hidden;
`;

const BodyWrapper = styled.div`
  display: flex;
  flex: 1;
  min-height: 0;
  background-color: ${t.color.background.base};
`;

interface SettingsPageContentProps {
  session: AppSession;
  onClose: () => void;
  onInviteTeam: () => void;
}

const SettingsPageContent = ({ session, onClose, onInviteTeam }: SettingsPageContentProps) => {
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
    <PageWrapper>
      <Flex alignItems={AlignItems.START} padding={16}>
        <BaseHeader title="Settings" onClose={onClose} />
      </Flex>
      <Divider />
      <BodyWrapper>
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
      </BodyWrapper>
    </PageWrapper>
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
      <Modal /* @dls-migrate modal.ariaLabel: The dialog needs a name: give it a `header` (often the title from the old `Widget`) or an `ariaLabel`. */
        isOpen={isSettingsOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) handleCloseSettings();
        }}
      >
        <SettingsPageContent
          session={session}
          onClose={handleCloseSettings}
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
