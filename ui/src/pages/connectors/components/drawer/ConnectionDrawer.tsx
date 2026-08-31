import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { KeyIcon, LinkBreakIcon, SlidersIcon } from "@phosphor-icons/react";
import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Modal from "@galaxy-io/dls/modal/Modal";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { GetConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";

import DangerZone from "@/components/DangerZone";
import DeleteConfirmDialog from "@/components/DeleteConfirmDialog";

import ErrorLayout from "@/layouts/ErrorLayout";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectionDrawerHeader from "@/pages/connectors/components/drawer/ConnectionDrawerHeader";
import ConnectionDrawerJsonSection from "@/pages/connectors/components/drawer/ConnectionDrawerJsonSection";
import ConnectionDrawerKeyValueRow from "@/pages/connectors/components/drawer/ConnectionDrawerKeyValueRow";
import ConnectionDrawerList from "@/pages/connectors/components/drawer/ConnectionDrawerList";
import ConnectionDrawerPipelines from "@/pages/connectors/components/drawer/ConnectionDrawerPipelines";

import { useDeleteConnectionMutation, useGetConnectionQuery } from "@/api/queries/connections";

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
  onClose: () => void;
}

const ConnectionDrawer = ({ onClose }: ConnectionDrawerProps) => {
  const navigate = useNavigate();
  const { connectionId } = useSearch({ from: "__root__" });

  const { data, isError } = useGetConnectionQuery({
    input: create(GetConnectionRequestSchema, { id: connectionId ?? "" }),
    options: { enabled: !!connectionId, retry: false },
  });
  const connection = data?.connection;

  const { mutate: deleteConnection, isPending: isDeleting } = useDeleteConnectionMutation();

  const { handleOpen, isOpen, handleClose, handleConfirm } = useDeleteConfirm({
    entityLabel: "Connection",
    entityName: connection?.name ?? "",
    onDelete: ({ onSuccess, onError }) => {
      if (!connection) return;
      deleteConnection({ id: connection.id }, { onSuccess, onError });
    },
    onDeleted: () => {
      onClose();
      navigate({
        to: connection?.kind === ConnectorKind.SINK ? "/sinks" : "/sources",
      });
    },
  });

  if (isError) {
    return (
      <DrawerWrapper>
        <ErrorLayout
          icon={<Icon component={LinkBreakIcon} size={24} variant={IconVariant.ERROR} />}
          header="Connection not found"
          message="This connection no longer exists."
          actions={<Button label="Close" onClick={onClose} variant={ButtonVariant.SECONDARY} />}
        />
      </DrawerWrapper>
    );
  }

  if (!connection) {
    return (
      <DrawerWrapper>
        <FlexWrapper alignItems={AlignItems.CENTER} padding="12px 16px" gap={12} fillWidth>
          <TextShimmer height={36} width={36} />
          <FlexWrapper direction={FlexDirection.COLUMN} gap={6}>
            <TextShimmer height={18} width={160} />
            <TextShimmer height={14} width={120} />
          </FlexWrapper>
        </FlexWrapper>
        <HorizontalDivider />
        <DrawerBody>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={12} padding="16px" fillWidth>
            <ConnectionDrawerList>
              <ConnectionDrawerKeyValueRow
                label="Kind"
                value={<TextShimmer height={16} width={60} />}
              />
              <ConnectionDrawerKeyValueRow
                label="Version"
                value={<TextShimmer height={16} width={40} />}
              />
            </ConnectionDrawerList>
          </FlexWrapper>
        </DrawerBody>
      </DrawerWrapper>
    );
  }

  return (
    <DrawerWrapper>
      <ConnectionDrawerHeader onClose={onClose} />

      <HorizontalDivider />

      <DrawerBody>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={12} padding="16px" fillWidth>
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
        </FlexWrapper>
      </DrawerBody>

      <HorizontalDivider />

      <FlexItem shrink={0} grow={0} padding="16px" fillWidth>
        <DangerZone
          title="Delete connection"
          description="This will permanently delete this connection."
          onDelete={handleOpen}
          isDisabled={!!connection.deletedAt}
        />
      </FlexItem>

      <Modal open={isOpen} onClose={handleClose}>
        <DeleteConfirmDialog
          open={isOpen}
          onClose={handleClose}
          onConfirm={handleConfirm}
          title="Delete connection"
          body="Are you sure you want to delete this connection? This is a destructive action and cannot be undone."
          confirmationPhrase={connection.name || ""}
          confirmLabel="Delete connection"
          isPending={isDeleting}
        />
      </Modal>
    </DrawerWrapper>
  );
};

export default ConnectionDrawer;
