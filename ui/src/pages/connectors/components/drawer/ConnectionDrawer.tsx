import { create } from "@bufbuild/protobuf";
import { KeyIcon, LinkBreakIcon, SlidersIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Drawer, { DrawerSize } from "@galaxy-io/dls/drawer/Drawer";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { type Connection, GetConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import DangerZone from "@/components/DangerZone";

import ErrorLayout from "@/layouts/ErrorLayout";
import PendingLayout from "@/layouts/PendingLayout";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectionDrawerHeader from "@/pages/connectors/components/drawer/ConnectionDrawerHeader";
import ConnectionDrawerJsonSection from "@/pages/connectors/components/drawer/ConnectionDrawerJsonSection";
import ConnectionDrawerKeyValueRow from "@/pages/connectors/components/drawer/ConnectionDrawerKeyValueRow";
import ConnectionDrawerList from "@/pages/connectors/components/drawer/ConnectionDrawerList";
import ConnectionDrawerPipelines from "@/pages/connectors/components/drawer/ConnectionDrawerPipelines";

import { useDeleteConnectionMutation, useGetConnectionQuery } from "@/api/queries/connections";

import { useConfirm } from "@/hooks/useConfirm";

interface ConnectionDrawerProps {
  isOpen: boolean;
  onClose: () => void;
}

const ConnectionDrawer = ({ isOpen, onClose }: ConnectionDrawerProps) => {
  const navigate = useNavigate();
  const { connectionId } = useSearch({ from: "/_app" });

  const { data, isError } = useGetConnectionQuery({
    input: create(GetConnectionRequestSchema, { id: connectionId ?? "" }),
    options: { enabled: !!connectionId, retry: false },
  });
  const connection = data?.connection;

  const { mutate: deleteConnection } = useDeleteConnectionMutation();

  const {
    handleOpen,
    isOpen: confirmIsOpen,
    target,
    handleClose,
    handleConfirm,
  } = useConfirm<Connection>({
    entityLabel: "Connection",
    entityName: (c) => c.name,
    onConfirm: (c, { onSuccess, onError }) =>
      deleteConnection({ id: c.id }, { onSuccess, onError }),
    onConfirmed: (c) => {
      onClose();
      navigate({
        to: c.kind === ConnectorKind.SINK ? "/sinks" : "/sources",
      });
    },
  });

  const renderContent = () => {
    if (isError) {
      return (
        <ErrorLayout
          icon={LinkBreakIcon}
          header="Connection not found"
          message="This connection no longer exists."
          actions={<Button label="Close" onClick={onClose} variant={ButtonVariant.SECONDARY} />}
        />
      );
    }

    if (!connection) {
      return <PendingLayout />;
    }

    return (
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12} fillWidth>
        <ConnectionDrawerList>
          <ConnectionDrawerKeyValueRow
            label="Kind"
            value={<ConnectionKindChip kind={connection.kind} />}
          />
          <ConnectionDrawerKeyValueRow
            label="Version"
            value={
              <Text
                size={TextSize.BODY_SM}
                variant={TextVariant.SECONDARY}
              >{`Version ${connection.version.toString()}`}</Text>
            }
          />
        </ConnectionDrawerList>

        <ConnectionDrawerJsonSection
          header="Configuration"
          icon={SlidersIcon}
          data={connection.config}
          emptyHeader="No configuration"
          emptyMessage="This connection has no configuration values."
        />
        <ConnectionDrawerJsonSection
          header="Secrets"
          icon={KeyIcon}
          data={connection.secretRefs}
          emptyHeader="No secrets"
          emptyMessage="This connection has no secret references."
        />
        <ConnectionDrawerPipelines />
      </Flex>
    );
  };

  return (
    <Drawer
      size={DrawerSize.MEDIUM}
      isOpen={isOpen}
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      header={connection ? <ConnectionDrawerHeader /> : "Connection"}
      footer={
        connection && (
          <Box fillWidth>
            <DangerZone
              title="Delete connection"
              description="This will permanently delete this connection."
              onDelete={() => handleOpen(connection)}
              isDisabled={!!connection.deletedAt}
            />
          </Box>
        )
      }
    >
      {renderContent()}
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
    </Drawer>
  );
};

export default ConnectionDrawer;
