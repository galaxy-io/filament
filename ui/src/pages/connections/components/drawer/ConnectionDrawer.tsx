import type { FC } from "react";

import { create } from "@bufbuild/protobuf";
import {
  DotsThreeIcon,
  KeyIcon,
  LinkBreakIcon,
  PencilIcon,
  SlidersIcon,
  TrashIcon,
} from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Drawer, { DrawerSize } from "@galaxy-io/dls/drawer/Drawer";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import PendingLayout from "@galaxy-io/dls/layout/PendingLayout";
import Menu, { MenuItem, MenuItemVariant, MenuSeparator } from "@galaxy-io/dls/menu/Menu";
import ConfirmDialog from "@galaxy-io/dls/modal/ConfirmDialog";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { type Connection, DeleteConnectionRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { GetConnectorRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import ConnectionKindChip from "@/components/connections/ConnectionKindChip";
import { CONNECTOR_KIND_TO_PATH_MAP } from "@/components/connections/constants";
import {
  formatConnectorName,
  getConnectorFamilyName,
  getConnectorVariantName,
} from "@/components/connections/utils";
import KeyValueList from "@/components/KeyValueList";
import KeyValueListRow from "@/components/KeyValueListRow";

import ConnectionDrawerHeader from "@/pages/connections/components/drawer/ConnectionDrawerHeader";
import ConnectionDrawerJsonSection from "@/pages/connections/components/drawer/ConnectionDrawerJsonSection";
import ConnectionDrawerPipelines from "@/pages/connections/components/drawer/ConnectionDrawerPipelines";

import { useFilamentNavigate, useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";
import { Flow } from "@/module/types";

import {
  createGetConnectionInput,
  useDeleteConnectionMutation,
  useGetConnectionQuery,
} from "@/api/queries/connections";
import { useGetConnectorQuery } from "@/api/queries/connectors";

import { useConfirm } from "@/hooks/useConfirm";

import { formatVersion } from "@/utils/format";

interface ConnectionDrawerProps {
  connectionId?: string;
  isOpen: boolean;
  onClose: () => void;
}

const ConnectionDrawer: FC<ConnectionDrawerProps> = ({ connectionId, isOpen, onClose }) => {
  const navigate = useFilamentNavigate();
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();

  const { data, isError } = useGetConnectionQuery({
    input: createGetConnectionInput(connectionId ?? ""),
    options: { enabled: !!connectionId, retry: false },
  });
  const connection = data?.connection;

  const { data: connectorData } = useGetConnectorQuery({
    input: create(GetConnectorRequestSchema, {
      connector: connection?.connector ?? "",
      kind: connection?.kind,
    }),
    options: { enabled: !!connection },
  });
  const connector = connectorData?.connector;
  const connectorVariant = connection && getConnectorVariantName(connection.connector);

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
      deleteConnection(create(DeleteConnectionRequestSchema, { id: c.id }), { onSuccess, onError }),
    onConfirmed: (c) => {
      onClose();
      void navigate({ to: CONNECTOR_KIND_TO_PATH_MAP[c.kind] });
    },
  });

  const handleEdit = () => {
    void updateSearch((prev) => ({ ...prev, flow: Flow.EDIT_CONNECTION }));
  };

  const renderContent = () => {
    if (isError) {
      return (
        <ErrorLayout
          icon={LinkBreakIcon}
          header="Connection not found"
          description="This connection no longer exists."
          actions={<Button label="Close" onClick={onClose} variant={ButtonVariant.SECONDARY} />}
        />
      );
    }

    if (!connection) {
      return <PendingLayout />;
    }

    return (
      <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12} fillWidth>
        <KeyValueList>
          <KeyValueListRow label="Kind" value={<ConnectionKindChip kind={connection.kind} />} />
          <KeyValueListRow
            label="Connector"
            value={
              <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
                <Chip
                  label={
                    connector
                      ? formatConnectorName(connector)
                      : getConnectorFamilyName(connection.connector)
                  }
                  variant={ChipVariant.SECONDARY}
                  size={ChipSize.SMALL}
                />
                {connectorVariant && (
                  <Chip
                    label={connectorVariant}
                    variant={ChipVariant.SECONDARY}
                    size={ChipSize.SMALL}
                  />
                )}
              </Flex>
            }
          />
          <KeyValueListRow
            label="Version"
            value={
              <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                {formatVersion(connection.version)}
              </Text>
            }
          />
        </KeyValueList>

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
      </Flex>
    );
  };

  return (
    <Drawer
      size={DrawerSize.MEDIUM}
      isOpen={isOpen}
      hasDividers
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      header={connection ? <ConnectionDrawerHeader connection={connection} /> : "Connection"}
      actions={
        connection && (
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
              label="Edit connection"
              icon={PencilIcon}
              onSelect={handleEdit}
              isDisabled={!!connection.deletedAt}
            />
            <MenuSeparator />
            <MenuItem
              label="Delete connection"
              icon={TrashIcon}
              variant={MenuItemVariant.ERROR}
              onSelect={() => handleOpen(connection)}
              isDisabled={!!connection.deletedAt}
            />
          </Menu>
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
