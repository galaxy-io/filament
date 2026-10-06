import { useCallback } from "react";

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

const AppLayout = () => {
  const navigate = useNavigate();
  const { connectionId, flow } = useSearch({ from: "/_app" });

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
    <>
      <Outlet />
      <Drawer /* @dls-migrate drawer.ariaLabel: The dialog panel needs a name: give it a `header` or an `ariaLabel`. */
        isOpen={!!connectionId}
        onOpenChange={(isOpen) => {
          if (!isOpen) handleCloseDrawer();
        }} /* @dls-migrate drawer.width-other: Pick a `DrawerSize` (default `MEDIUM`, 560px), or `isResizable`. */
        width={CONNECTOR_DRAWER_WIDTH}
      >
        {connectionId && <ConnectionDrawer onClose={handleCloseDrawer} />}
      </Drawer>
      <Modal /* @dls-migrate modal.ariaLabel: The dialog needs a name: give it a `header` (often the title from the old `Widget`) or an `ariaLabel`. */
        isOpen={flow === Flow.CREATE_CONNECTION}
        onOpenChange={(isOpen) => {
          if (!isOpen) handleCloseFlow();
        }}
      >
        <CreateConnectionModal onClose={handleCloseFlow} />
      </Modal>
      <Modal /* @dls-migrate modal.ariaLabel: The dialog needs a name: give it a `header` (often the title from the old `Widget`) or an `ariaLabel`. */
        isOpen={flow === Flow.EDIT_CONNECTION && !!connectionId}
        onOpenChange={(isOpen) => {
          if (!isOpen) handleCloseFlow();
        }}
      >
        {connectionId && <EditConnectionModal onClose={handleCloseFlow} />}
      </Modal>
      <Modal /* @dls-migrate modal.ariaLabel: The dialog needs a name: give it a `header` (often the title from the old `Widget`) or an `ariaLabel`. */
        isOpen={flow === Flow.CREATE_PIPELINE}
        onOpenChange={(isOpen) => {
          if (!isOpen) handleCloseFlow();
        }}
      >
        <CreatePipelineModal onClose={handleCloseFlow} />
      </Modal>
      <SettingsPage />
    </>
  );
};

export default AppLayout;
