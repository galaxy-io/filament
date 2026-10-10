import type { FC } from "react";

import { PlusIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Button from "@galaxy-io/dls/buttons/Button";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/components/connections/constants";

import PipelineCanvasConnectionSelectorItem from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelectorItem";

import { useFilamentFlowOpen } from "@/module/hooks";
import { Flow } from "@/module/types";

const PipelineCanvasConnectionSelectorEmpty: FC<{
  message: string;
  connectorKind: ConnectorKind;
}> = ({ message, connectorKind }) => {
  const openFlow = useFilamentFlowOpen();

  const handleCreateConnection = () => {
    openFlow(Flow.CREATE_CONNECTION, connectorKind);
  };

  return (
    <Flex
      fillWidth
      height="100%"
      minHeight={240}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      padding={24}
    >
      <EmptyLayout
        size={EmptyLayoutSize.SMALL}
        header={message}
        actions={
          <Button
            label={`Create ${CONNECTOR_KIND_TO_LABEL_MAP[connectorKind].toLowerCase()}`}
            icon={PlusIcon}
            onClick={handleCreateConnection}
          />
        }
      />
    </Flex>
  );
};

const isConnectionDisabled = (connection: Connection, isSourceDisabled: boolean) =>
  isSourceDisabled && connection.kind === ConnectorKind.SOURCE;

interface PipelineCanvasConnectionSelectorListProps {
  connections: Connection[];
  hasConnections: boolean;
  connectorKind: ConnectorKind;
  isSourceDisabled: boolean;
  onConnectionClick: (connection: Connection) => void;
}

const PipelineCanvasConnectionSelectorList: FC<PipelineCanvasConnectionSelectorListProps> = ({
  connections,
  hasConnections,
  connectorKind,
  isSourceDisabled,
  onConnectionClick,
}) => {
  if (!hasConnections) {
    return (
      <PipelineCanvasConnectionSelectorEmpty
        message={`No ${pluralize(CONNECTOR_KIND_TO_LABEL_MAP[connectorKind].toLowerCase())} found`}
        connectorKind={connectorKind}
      />
    );
  }

  if (!connections.length) {
    return (
      <PipelineCanvasConnectionSelectorEmpty
        message="No connections match your search"
        connectorKind={connectorKind}
      />
    );
  }

  const orderedConnections = [
    ...connections.filter((connection) => !isConnectionDisabled(connection, isSourceDisabled)),
    ...connections.filter((connection) => isConnectionDisabled(connection, isSourceDisabled)),
  ];

  return (
    <FlexItem grow={1} minHeight={0}>
      <ScrollArea>
        <Flex direction={FlexDirection.COLUMN} gap={2} padding={8}>
          {orderedConnections.map((connection) => (
            <PipelineCanvasConnectionSelectorItem
              key={connection.id}
              connection={connection}
              isDisabled={isConnectionDisabled(connection, isSourceDisabled)}
              onClick={() => onConnectionClick(connection)}
            />
          ))}
        </Flex>
      </ScrollArea>
    </FlexItem>
  );
};

export default PipelineCanvasConnectionSelectorList;
