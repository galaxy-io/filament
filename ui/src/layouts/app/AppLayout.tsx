import { useCallback, useMemo, useSyncExternalStore } from "react";

import { Outlet, useNavigate, useSearch } from "@tanstack/react-router";

import Drawer from "@galaxy-io/dls/drawer/Drawer";
import Modal from "@galaxy-io/dls/modal/Modal";

import { Flow } from "@/layouts/app/types";

import CreateConnectionModal from "@/pages/connectors/components/create/CreateConnectionModal";
import ConnectionDrawer from "@/pages/connectors/components/drawer/ConnectionDrawer";
import EditConnectionModal from "@/pages/connectors/components/edit/EditConnectionModal";
import { CONNECTOR_DRAWER_WIDTH } from "@/pages/connectors/constants";
import CreatePipelineModal from "@/pages/pipelines/components/create/CreatePipelineModal";
import SettingsPage from "@/pages/settings/SettingsPage";

import { getSessionUser, subscribeToSession } from "@/auth/oidc";
import { AppSessionProvider } from "@/auth/session";
import type { AppSession } from "@/auth/types";

const AppLayout = () => {
  const navigate = useNavigate();
  const { connectionId, flow } = useSearch({ from: "/_app" });
  const user = useSyncExternalStore(subscribeToSession, getSessionUser);

  const session = useMemo<AppSession>(
    () =>
      user
        ? {
            isAuthenticated: true,
            userId: user.profile.sub,
            name: user.profile.name,
            email: user.profile.email,
            avatarUrl: user.profile.picture,
          }
        : { isAuthenticated: false },
    [user],
  );

  const handleCloseDrawer = useCallback(() => {
    void navigate({
      to: ".",
      search: (prev) => {
        const {
          connectionId: _,
          connector: __,
          connectorKind: ___,
          flow: prevFlow,
          ...rest
        } = prev;
        return prevFlow === Flow.EDIT_CONNECTION ? rest : { ...rest, flow: prevFlow };
      },
    });
  }, [navigate]);

  const handleCloseFlow = useCallback(() => {
    void navigate({
      to: ".",
      search: (prev) => {
        const { flow: _, connector: __, connectorKind: ___, connectorSearch: ____, ...rest } = prev;
        return rest;
      },
    });
  }, [navigate]);

  return (
    <AppSessionProvider value={session}>
      <Outlet />
      <Drawer open={!!connectionId} onClose={handleCloseDrawer} width={CONNECTOR_DRAWER_WIDTH}>
        {connectionId && <ConnectionDrawer onClose={handleCloseDrawer} />}
      </Drawer>
      <Modal open={flow === Flow.CREATE_CONNECTION} onClose={handleCloseFlow}>
        <CreateConnectionModal onClose={handleCloseFlow} />
      </Modal>
      <Modal open={flow === Flow.EDIT_CONNECTION && !!connectionId} onClose={handleCloseFlow}>
        {connectionId && <EditConnectionModal onClose={handleCloseFlow} />}
      </Modal>
      <Modal open={flow === Flow.CREATE_PIPELINE} onClose={handleCloseFlow}>
        <CreatePipelineModal onClose={handleCloseFlow} />
      </Modal>
      <SettingsPage session={session} />
    </AppSessionProvider>
  );
};

export default AppLayout;
