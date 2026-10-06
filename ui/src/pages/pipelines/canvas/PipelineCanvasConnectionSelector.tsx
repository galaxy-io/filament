import { useState } from "react";

import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import {
  PIPELINE_CANVAS_CONNECTION_SELECTOR_MAX_HEIGHT,
  PIPELINE_CANVAS_CONNECTION_SELECTOR_WIDTH,
} from "@/pages/pipelines/canvas/constants";
import { getNextNodePosition } from "@/pages/pipelines/canvas/graph/layout";
import { canAddSourceNode, createNodeFromConnection } from "@/pages/pipelines/canvas/graph/rules";
import PipelineCanvasConnectionSelectorList from "@/pages/pipelines/canvas/PipelineCanvasConnectionSelectorList";
import {
  usePipelineCanvasActions,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";

import { isSearchMatch } from "@/utils/search";

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
  width = PIPELINE_CANVAS_CONNECTION_SELECTOR_WIDTH,
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
    <Flex
      direction={FlexDirection.COLUMN}
      width={width}
      height={fillHeight ? "100%" : undefined}
      maxHeight={PIPELINE_CANVAS_CONNECTION_SELECTOR_MAX_HEIGHT}
      overflow="hidden"
    >
      <Box padding={8}>
        <SearchInput
          value={state.search}
          onChange={handleSearchChange}
          placeholder="Search connections..."
          fillWidth
        />
      </Box>
      <Divider />
      <PipelineCanvasConnectionSelectorList
        connections={filteredConnections}
        hasConnections={kindConnections.length > 0}
        connectorKind={kindFilter}
        isSourceDisabled={!canAddSourceNode(canvasState.nodes)}
        onConnectionClick={handleConnectionClick}
      />
    </Flex>
  );
};

export default PipelineCanvasConnectionSelector;
