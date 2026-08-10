import { useState } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import { getNextNodePosition } from "@/pages/pipelines/canvas/graph/layout";
import { canAddSourceNode, createNodeFromConnection } from "@/pages/pipelines/canvas/graph/rules";
import PipelineCanvasConnectionSelectorList from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelectorList";
import {
  usePipelineCanvasActions,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";

import { isSearchMatch } from "@/utils/search";

const DEFAULT_BODY_WIDTH = 320;

const BodyWrapper = styled.div<{ $width: number; $fillHeight?: boolean }>`
  width: ${({ $width }) => $width}px;
  height: ${({ $fillHeight }) => ($fillHeight ? "100%" : "auto")};
  max-height: 480px;
  display: flex;
  flex-direction: column;
  border-radius: 8px;
  overflow: hidden;
`;

const SearchWrapper = withTheme(styled.div<PropsWithTheme>`
  padding: 8px;
  flex-shrink: 0;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

interface PipelineCanvasConnectionSelectorProps {
  kindFilter?: ConnectorKind;
  onSelect?: (connection: Connection) => void;
  width?: number;
  fillHeight?: boolean;
}

interface PipelineCanvasConnectionSelectorState {
  search: string;
}

const DEFAULT_STATE: PipelineCanvasConnectionSelectorState = {
  search: "",
};

const PipelineCanvasConnectionSelector = ({
  kindFilter = ConnectorKind.UNSPECIFIED,
  onSelect,
  width = DEFAULT_BODY_WIDTH,
  fillHeight = false,
}: PipelineCanvasConnectionSelectorProps) => {
  const [state, setState] = useState<PipelineCanvasConnectionSelectorState>(DEFAULT_STATE);
  const canvasState = usePipelineCanvasState();
  const { addNode } = usePipelineCanvasActions();

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  const { data } = useSuspenseListConnectionsQuery();

  const kindConnections =
    kindFilter === ConnectorKind.UNSPECIFIED
      ? data.connections
      : data.connections.filter((connection) => connection.kind === kindFilter);
  const filteredConnections = kindConnections.filter((connection) =>
    isSearchMatch(state.search, connection.name),
  );

  const handleConnectionClick = (connection: Connection) => {
    if (onSelect) {
      onSelect(connection);
      return;
    }

    const position = getNextNodePosition(connection.kind, canvasState.nodes);
    addNode(createNodeFromConnection(connection, position));
  };

  return (
    <BodyWrapper $width={width} $fillHeight={fillHeight}>
      <SearchWrapper>
        <TextInput
          value={state.search}
          onChange={handleSearchChange}
          placeholder="Search connections..."
          leading={{ icon: MagnifyingGlassIcon }}
          size={InputSize.LARGE}
          fillWidth
        />
      </SearchWrapper>
      <HorizontalDivider />
      <PipelineCanvasConnectionSelectorList
        connections={filteredConnections}
        hasConnections={kindConnections.length > 0}
        connectorKind={kindFilter}
        isSourceDisabled={!canAddSourceNode(canvasState.nodes)}
        onConnectionClick={handleConnectionClick}
      />
    </BodyWrapper>
  );
};

export default PipelineCanvasConnectionSelector;
