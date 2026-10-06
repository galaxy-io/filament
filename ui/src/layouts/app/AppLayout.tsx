import { useCallback } from "react";

import { Outlet, useNavigate, useSearch } from "@tanstack/react-router";

import Modal from "@galaxy-io/dls/modal/Modal";

import { Flow } from "@/layouts/app/types";

import CreateConnectionModal from "@/pages/connectors/components/create/CreateConnectionModal";
import ConnectionDrawer from "@/pages/connectors/components/drawer/ConnectionDrawer";
import EditConnectionModal from "@/pages/connectors/components/edit/EditConnectionModal";
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
      <ConnectionDrawer isOpen={!!connectionId} onClose={handleCloseDrawer} />
      {flow === Flow.CREATE_CONNECTION && <CreateConnectionModal onClose={handleCloseFlow} />}
      {flow === Flow.EDIT_CONNECTION && !!connectionId && (
        <EditConnectionModal onClose={handleCloseFlow} />
      )}
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
