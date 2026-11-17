import type { FC } from "react";

import { create } from "@bufbuild/protobuf";
import { DotsThreeIcon, FlowArrowIcon, PencilIcon, TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Menu, { MenuItem, MenuItemVariant, MenuSeparator } from "@galaxy-io/dls/menu/Menu";
import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";

import { type Connection, DeleteConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import { CONNECTOR_KIND_TO_PIPELINE_SEED_MAP } from "@/pages/connections/constants";

import { useFilamentFlowOpen, useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";
import { Flow } from "@/module/types";

import { useDeleteConnectionMutation } from "@/api/queries/connections";

import { useConfirm } from "@/hooks/useConfirm";

interface ConnectionDrawerActionsProps {
  connection: Connection;
  isOpen: boolean;
  onClose: () => void;
}

const ConnectionDrawerActions: FC<ConnectionDrawerActionsProps> = ({
  connection,
  isOpen,
  onClose,
}) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  const openFlow = useFilamentFlowOpen();

  const { mutate: deleteConnection } = useDeleteConnectionMutation();

  const {
    handleOpen,
    isOpen: confirmIsOpen,
    target,
    handleClose,
    handleConfirm,
  } = useConfirm<Connection>({
    entityLabel: "Connection",
    onConfirm: (c, { onSuccess, onError }) =>
      deleteConnection(create(DeleteConnectionRequestSchema, { id: c.id }), { onSuccess, onError }),
    onConfirmed: onClose,
  });

  const isDeleted = !!connection.deletedAt;

  const handleCreatePipeline = () => {
    openFlow(
      Flow.CREATE_PIPELINE,
      CONNECTOR_KIND_TO_PIPELINE_SEED_MAP[connection.kind](connection.id),
    );
  };

  const handleEdit = () => {
    void updateSearch((prev) => ({ ...prev, flow: Flow.EDIT_CONNECTION }));
  };

  return (
    <>
      <Menu
        trigger={
          <Button
            icon={DotsThreeIcon}
            ariaLabel="Connection actions"
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
          />
        }
      >
        <MenuItem
          label="Create pipeline"
          icon={FlowArrowIcon}
          onSelect={handleCreatePipeline}
          isDisabled={isDeleted}
        />
        <MenuItem
          label="Edit connection"
          icon={PencilIcon}
          onSelect={handleEdit}
          isDisabled={isDeleted}
        />
        <MenuSeparator />
        <MenuItem
          label="Delete connection"
          icon={TrashIcon}
          variant={MenuItemVariant.ERROR}
          onSelect={() => handleOpen(connection)}
          isDisabled={isDeleted}
        />
      </Menu>
      <ConfirmDialog
        isOpen={isOpen && confirmIsOpen}
        onOpenChange={(isOpen) => {
          if (!isOpen) handleClose();
        }}
        onConfirm={handleConfirm}
        header="Delete connection?"
        description="This deletes the connection. It cannot be undone."
        confirmValue={target?.name}
        label="Delete connection"
        isDestructive
      />
    </>
  );
};

export default ConnectionDrawerActions;
