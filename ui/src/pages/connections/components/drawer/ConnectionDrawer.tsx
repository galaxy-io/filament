import type { FC } from "react";

import { create } from "@bufbuild/protobuf";
import { KeyIcon, LinkBreakIcon, SlidersIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import CopyButton from "@galaxy-io/dls/buttons/CopyButton";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Drawer, { DrawerSize } from "@galaxy-io/dls/drawer/Drawer";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import PendingLayout from "@galaxy-io/dls/layout/PendingLayout";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { GetConnectorRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import ConnectionKindChip from "@/components/connections/ConnectionKindChip";
import {
  formatConnectorName,
  getConnectorFamilyName,
  getConnectorVariantName,
} from "@/components/connections/utils";
import KeyValueList from "@/components/KeyValueList";
import KeyValueListRow from "@/components/KeyValueListRow";

import ConnectionDrawerActions from "@/pages/connections/components/drawer/ConnectionDrawerActions";
import ConnectionDrawerHeader from "@/pages/connections/components/drawer/ConnectionDrawerHeader";
import ConnectionDrawerJsonSection from "@/pages/connections/components/drawer/ConnectionDrawerJsonSection";
import ConnectionDrawerPipelines from "@/pages/connections/components/drawer/ConnectionDrawerPipelines";

import { createGetConnectionInput, useGetConnectionQuery } from "@/api/queries/connections";
import { useGetConnectorQuery } from "@/api/queries/connectors";

import { useOverlaySession } from "@/hooks/useOverlaySession";

import { IS_DEBUG } from "@/constants";

import { isNotFoundError } from "@/utils/errors";
import { formatVersion } from "@/utils/format";

interface ConnectionDrawerProps {
  connectionId?: Connection["id"];
  isOpen: boolean;
  onClose: () => void;
}

const ConnectionDrawer: FC<ConnectionDrawerProps> = ({ connectionId, isOpen, onClose }) => {
  const session = useOverlaySession(isOpen);

  const { data, error, isError } = useGetConnectionQuery({
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

  const renderContent = () => {
    if (isError) {
      const isNotFound = isNotFoundError(error);

      return (
        <ErrorLayout
          icon={isNotFound ? LinkBreakIcon : undefined}
          header={isNotFound ? "Connection not found" : "Failed to load connection"}
          description={
            isNotFound
              ? "This connection no longer exists."
              : "The connection could not be loaded. Try again later."
          }
          detail={IS_DEBUG ? error?.message : undefined}
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
          <KeyValueListRow
            label="ID"
            value={
              <Flex alignItems={AlignItems.CENTER} gap={4} minWidth={0}>
                <Text
                  size={TextSize.BODY_SM}
                  variant={TextVariant.SECONDARY}
                  family={FontFamily.MONO}
                  lineClamp={1}
                >
                  {connection.id}
                </Text>
                <CopyButton
                  value={connection.id}
                  ariaLabel="Copy connection ID"
                  variant={ButtonVariant.TERTIARY}
                  size={ButtonSize.SMALL}
                />
              </Flex>
            }
          />
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
          <ConnectionDrawerActions
            key={`${session}:${connection.id}`}
            connection={connection}
            isOpen={isOpen}
            onClose={onClose}
          />
        )
      }
    >
      {renderContent()}
    </Drawer>
  );
};

export default ConnectionDrawer;
