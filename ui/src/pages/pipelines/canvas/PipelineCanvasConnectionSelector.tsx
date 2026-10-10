import { type FC, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { keepPreviousData } from "@tanstack/react-query";

import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { type Connection, ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";

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

import { createListConnectionsInput, useListConnectionsQuery } from "@/api/queries/connections";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/constants";

interface PipelineCanvasConnectionSelectorProps {
  kindFilter?: ConnectorKind;
  onSelect?: (connection: Connection) => void;
  width?: number;
  fillHeight?: boolean;
}

interface PipelineCanvasConnectionSelectorState {
  q: string;
}

const DEFAULT_STATE: PipelineCanvasConnectionSelectorState = {
  q: "",
};

const PipelineCanvasConnectionSelector: FC<PipelineCanvasConnectionSelectorProps> = ({
  kindFilter = ConnectorKind.UNSPECIFIED,
  onSelect,
  width = PIPELINE_CANVAS_CONNECTION_SELECTOR_WIDTH,
  fillHeight = false,
}) => {
  const [state, setState] = useState<PipelineCanvasConnectionSelectorState>(DEFAULT_STATE);
  const canvasState = usePipelineCanvasState();
  const { addNode } = usePipelineCanvasActions();

  const handleSearch = (q: string) => {
    setState((prev) => ({ ...prev, q }));
  };

  const { data } = useListConnectionsQuery({
    input: create(
      ListConnectionsRequestSchema,
      createListConnectionsInput(kindFilter, { q: state.q }),
    ),
    options: { placeholderData: keepPreviousData },
  });
  const connections = data?.connections ?? [];

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
          ariaLabel="Search connections"
          placeholder="Search connections..."
          debounceMs={LIST_SEARCH_DEBOUNCE_MS}
          onSearch={handleSearch}
          fillWidth
        />
      </Box>
      <Divider />
      <PipelineCanvasConnectionSelectorList
        connections={connections}
        hasConnections={!!state.q || connections.length > 0}
        isLoading={!data}
        connectorKind={kindFilter}
        isSourceDisabled={!canAddSourceNode(canvasState.nodes)}
        onConnectionClick={handleConnectionClick}
      />
    </Flex>
  );
};

export default PipelineCanvasConnectionSelector;
