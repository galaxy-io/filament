import type { FC } from "react";

import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import PendingLayout, { PendingLayoutSize } from "@galaxy-io/dls/layout/PendingLayout";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { CONNECTOR_KIND_TO_PLURAL_NOUN_MAP } from "@/components/connections/constants";

import PipelineCanvasConnectionSelectorEmpty from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelectorEmpty";
import PipelineCanvasConnectionSelectorItem from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelectorItem";
import { isPipelineCanvasConnectionDisabled } from "@/pages/pipelines/canvas/utils";

interface PipelineCanvasConnectionSelectorListProps {
  connections: Connection[];
  hasConnections: boolean;
  isLoading: boolean;
  connectorKind: ConnectorKind;
  isSourceDisabled: boolean;
  onConnectionClick: (connection: Connection) => void;
}

const PipelineCanvasConnectionSelectorList: FC<PipelineCanvasConnectionSelectorListProps> = ({
  connections,
  hasConnections,
  isLoading,
  connectorKind,
  isSourceDisabled,
  onConnectionClick,
}) => {
  if (isLoading) {
    return (
      <Flex
        fillWidth
        minHeight={240}
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.CENTER}
      >
        <PendingLayout size={PendingLayoutSize.SMALL} />
      </Flex>
    );
  }

  if (!hasConnections) {
    return (
      <PipelineCanvasConnectionSelectorEmpty
        message={`No ${CONNECTOR_KIND_TO_PLURAL_NOUN_MAP[connectorKind]} found`}
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
    ...connections.filter(
      (connection) => !isPipelineCanvasConnectionDisabled(connection, isSourceDisabled),
    ),
    ...connections.filter((connection) =>
      isPipelineCanvasConnectionDisabled(connection, isSourceDisabled),
    ),
  ];

  return (
    <FlexItem grow={1} minHeight={0}>
      <ScrollArea>
        <Flex direction={FlexDirection.COLUMN} gap={2} padding={8}>
          {orderedConnections.map((connection) => (
            <PipelineCanvasConnectionSelectorItem
              key={connection.id}
              connection={connection}
              isDisabled={isPipelineCanvasConnectionDisabled(connection, isSourceDisabled)}
              onClick={() => onConnectionClick(connection)}
            />
          ))}
        </Flex>
      </ScrollArea>
    </FlexItem>
  );
};

export default PipelineCanvasConnectionSelectorList;
