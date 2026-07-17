import { useState } from "react";

import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";

import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Modal from "@galaxy-io/dls/modal/Modal";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import ConnectionDrawerDetails from "@/pages/connectors/components/drawer/ConnectionDrawerDetails";
import ConnectionDrawerHeader from "@/pages/connectors/components/drawer/ConnectionDrawerHeader";
import ConnectionDrawerMetrics from "@/pages/connectors/components/drawer/ConnectionDrawerMetrics";

import { ToastVariant } from "@/providers/toast/Toast";
import { useToast } from "@/providers/toast/useToast";

import { useDeleteConnectionMutation } from "@/api/queries/connectors";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import DangerZone from "@/components/DangerZone";
import DeleteConfirmDialog from "@/components/DeleteConfirmDialog";

const DrawerWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;

  width: 100%;
  height: 100%;

  overflow: hidden;

  background-color: ${({ theme }) => theme.color.background.primary};
`);

const DrawerBody = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  gap: 12px;

  flex: 1;

  padding: 16px;

  min-height: 0;

  overflow-y: auto;

  background-color: ${({ theme }) => theme.color.background.base};
`);

interface ConnectionDrawerProps {
  connection: Connection;
  onClose: () => void;
}

const ConnectionDrawer = ({ connection, onClose }: ConnectionDrawerProps) => {
  const navigate = useNavigate();
  const { mutate: deleteConnection, isPending: isDeleting } = useDeleteConnectionMutation();
  const { showToast } = useToast();

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);

  const handleOpenDeleteModal = () => setIsDeleteModalOpen(true);
  const handleCloseDeleteModal = () => setIsDeleteModalOpen(false);

  const handleConfirmDelete = () => {
    deleteConnection(
      { id: connection.id },
      {
        onSuccess: () => {
          showToast({
            header: "Connection deleted",
            subheader: `${connection.name} has been deleted successfully.`,
            variant: ToastVariant.SUCCESS,
          });
          onClose();
          navigate({ to: "/connections" });
        },
        onError: (error) => {
          showToast({
            header: "Delete failed",
            subheader: error instanceof Error ? error.message : "Failed to delete connection",
            variant: ToastVariant.ERROR,
          });
          setIsDeleteModalOpen(false);
        },
      },
    );
  };

  return (
    <DrawerWrapper>
      <ConnectionDrawerHeader connection={connection} onClose={onClose} />

      <HorizontalDivider />

      <DrawerBody>
        <ConnectionDrawerMetrics connection={connection} />

        <ConnectionDrawerDetails connection={connection} />

        {/* Spacer */}
        <FlexWrapper fillWidth fillHeight />

        <DangerZone
          title="Delete connection"
          description="This will permanently delete this connection."
          buttonLabel="Delete"
          onAction={handleOpenDeleteModal}
        />
      </DrawerBody>

      <Modal open={isDeleteModalOpen} onClose={handleCloseDeleteModal}>
        <DeleteConfirmDialog
          open={isDeleteModalOpen}
          onClose={handleCloseDeleteModal}
          onConfirm={handleConfirmDelete}
          title="Delete connection"
          body="This action cannot be undone. This will permanently delete this connection and all associated data."
          confirmationPhrase={connection.name || ""}
          confirmLabel="Delete connection"
          isPending={isDeleting}
        />
      </Modal>
    </DrawerWrapper>
  );
};

export default ConnectionDrawer;
