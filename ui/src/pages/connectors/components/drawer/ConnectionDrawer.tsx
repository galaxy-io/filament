import { styled } from "@linaria/react";
import { KeyIcon, SlidersIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import CopyInput from "@galaxy-io/dls/inputs/CopyInput";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import Modal from "@galaxy-io/dls/modal/Modal";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import DangerZone from "@/components/DangerZone";
import DeleteConfirmDialog from "@/components/DeleteConfirmDialog";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectionDrawerHeader from "@/pages/connectors/components/drawer/ConnectionDrawerHeader";
import ConnectionDrawerJsonSection from "@/pages/connectors/components/drawer/ConnectionDrawerJsonSection";
import ConnectionDrawerKeyValueRow from "@/pages/connectors/components/drawer/ConnectionDrawerKeyValueRow";
import ConnectionDrawerList from "@/pages/connectors/components/drawer/ConnectionDrawerList";
import ConnectionDrawerPipelines from "@/pages/connectors/components/drawer/ConnectionDrawerPipelines";

import { useDeleteConnectionMutation } from "@/api/queries/connections";

import { useDeleteConfirm } from "@/hooks/useDeleteConfirm";

const DrawerWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

const DrawerBody = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
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

  const deleteConfirm = useDeleteConfirm({
    entityLabel: "Connection",
    entityName: connection.name,
    onDelete: (callbacks) => deleteConnection({ id: connection.id }, callbacks),
    onDeleted: () => {
      onClose();
      navigate({
        to: connection.kind === ConnectorKind.SINK ? "/sinks" : "/sources",
      });
    },
  });

  return (
    <DrawerWrapper>
      <ConnectionDrawerHeader connection={connection} onClose={onClose} />

      <HorizontalDivider />

      <DrawerBody>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={12} padding="16px" fillWidth>
          <ConnectionDrawerList>
            <ConnectionDrawerKeyValueRow
              label="Connection ID"
              value={
                <CopyInput value={connection.id} size={InputSize.SMALL} width={272} isMonospace />
              }
            />
            <ConnectionDrawerKeyValueRow
              label="Connector"
              value={<Text size={TextSize.BODY_SM}>{connection.connector}</Text>}
            />
            <ConnectionDrawerKeyValueRow
              label="Kind"
              value={<ConnectionKindChip kind={connection.kind} />}
            />
            <ConnectionDrawerKeyValueRow
              label="Version"
              value={<Text size={TextSize.BODY_SM}>{connection.version.toString()}</Text>}
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
          <ConnectionDrawerPipelines connectionId={connection.id} />
        </FlexWrapper>
      </DrawerBody>

      <HorizontalDivider />

      <FlexItem shrink={0} grow={0} padding="16px" fillWidth>
        <DangerZone
          title="Delete connection"
          description="This will permanently delete this connection."
          onClick={deleteConfirm.handleOpen}
        />
      </FlexItem>

      <Modal open={deleteConfirm.isOpen} onClose={deleteConfirm.handleClose}>
        <DeleteConfirmDialog
          open={deleteConfirm.isOpen}
          onClose={deleteConfirm.handleClose}
          onConfirm={deleteConfirm.handleConfirm}
          title="Delete connection"
          body="This will permanently delete this connection and all associated data."
          confirmationPhrase={connection.name || ""}
          confirmLabel="Delete connection"
          isPending={isDeleting}
        />
      </Modal>
    </DrawerWrapper>
  );
};

export default ConnectionDrawer;
