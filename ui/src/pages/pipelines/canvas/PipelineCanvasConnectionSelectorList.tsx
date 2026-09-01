import { styled } from "@linaria/react";
import { PlusIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";
import pluralize from "pluralize";

import Button, { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { Flow } from "@/layouts/app/types";
import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/pages/connectors/constants";
import PipelineCanvasConnectionSelectorItem from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelectorItem";

const ConnectionList = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 8px;
  background-color: ${({ theme }) => theme.color.background.primary};

  display: flex;
  flex-direction: column;
  gap: 2px;
`);

const PipelineCanvasConnectionSelectorEmpty = ({
  message,
  icon,
  connectorKind,
}: {
  message: string;
  icon?: React.ReactNode;
  connectorKind: ConnectorKind;
}) => {
  const navigate = useNavigate();

  const handleCreateConnection = () => {
    navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        connectionId: undefined,
        flow: Flow.CREATE_CONNECTION,
        connectorKind,
      }),
    });
  };

  return (
    <FlexWrapper
      fillWidth
      fillHeight
      minHeight={240}
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      padding={24}
    >
      <EmptyLayout
        size={EmptyLayoutSize.SMALL}
        message={message}
        icon={icon}
        actions={[
          <Button
            key="create-connection"
            label={`Create ${CONNECTOR_KIND_TO_LABEL_MAP[connectorKind].toLowerCase()}`}
            icon={PlusIcon}
            size={ButtonSize.SMALL}
            onClick={handleCreateConnection}
          />,
        ]}
      />
    </FlexWrapper>
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

const PipelineCanvasConnectionSelectorList = ({
  connections,
  hasConnections,
  connectorKind,
  isSourceDisabled,
  onConnectionClick,
}: PipelineCanvasConnectionSelectorListProps) => {
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
    <ConnectionList>
      {orderedConnections.map((connection) => (
        <PipelineCanvasConnectionSelectorItem
          key={connection.id}
          connection={connection}
          isDisabled={isConnectionDisabled(connection, isSourceDisabled)}
          onClick={() => onConnectionClick(connection)}
        />
      ))}
    </ConnectionList>
  );
};

export default PipelineCanvasConnectionSelectorList;
