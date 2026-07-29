import { useState } from "react";

import { styled } from "@linaria/react";
import { useNodeId, useUpdateNodeInternals } from "@xyflow/react";

import Badge, { BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import {
  PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE,
  PIPELINE_CANVAS_NODE_PADDING,
  PIPELINE_CANVAS_NODE_TABLE_LIST_MAX_HEIGHT,
} from "@/pages/pipelines/canvas/nodes/constants";
import PipelineCanvasNodeIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeIsland";
import PipelineCanvasNodeSourceIslandTableList from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSourceIslandTableList";
import type { PipelineCanvasNodeTableInfo } from "@/pages/pipelines/canvas/types";

import { isSearchMatch } from "@/utils/search";

const IslandWrapper = styled(PipelineCanvasNodeIsland)`
  padding: 0;
`;

const SearchSection = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: ${PIPELINE_CANVAS_NODE_PADDING}px;
`;

const BadgeSlot = styled.span`
  width: ${PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE}px;
  flex-shrink: 0;

  display: flex;
  align-items: center;
  justify-content: center;
`;

const TableList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: ${PIPELINE_CANVAS_NODE_PADDING}px ${PIPELINE_CANVAS_NODE_PADDING}px
    ${PIPELINE_CANVAS_NODE_PADDING}px 12px;

  max-height: ${PIPELINE_CANVAS_NODE_TABLE_LIST_MAX_HEIGHT}px;
  overflow-y: auto;
`;

interface PipelineCanvasNodeSourceIslandProps {
  tables: PipelineCanvasNodeTableInfo[];
  error?: Error | null;
  isLoading?: boolean;
  isSelected?: boolean;
}

interface PipelineCanvasNodeSourceIslandState {
  search: string;
}

const DEFAULT_STATE: PipelineCanvasNodeSourceIslandState = {
  search: "",
};

const PipelineCanvasNodeSourceIsland = ({
  tables,
  error,
  isLoading = false,
  isSelected,
}: PipelineCanvasNodeSourceIslandProps) => {
  const nodeId = useNodeId();
  const updateNodeInternals = useUpdateNodeInternals();
  const [state, setState] = useState<PipelineCanvasNodeSourceIslandState>(DEFAULT_STATE);

  const syncNodeInternals = () => {
    if (nodeId) {
      updateNodeInternals(nodeId);
    }
  };

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
    syncNodeInternals();
  };

  const filteredTables = tables.filter((table) => isSearchMatch(state.search, table.name));
  const connectedCount = tables.filter((table) => table.isConnected).length;

  return (
    <IslandWrapper $isSelected={isSelected}>
      <SearchSection className="nodrag">
        <TextInput
          placeholder="Search"
          value={state.search}
          onChange={handleSearchChange}
          size={InputSize.LARGE}
          fillWidth
        />
        {connectedCount > 0 && (
          <BadgeSlot>
            <Badge count={connectedCount} variant={BadgeVariant.SECONDARY} />
          </BadgeSlot>
        )}
      </SearchSection>

      <HorizontalDivider />

      <TableList className="nowheel" onScroll={syncNodeInternals}>
        <PipelineCanvasNodeSourceIslandTableList
          tables={filteredTables}
          error={error}
          isLoading={isLoading}
        />
      </TableList>
    </IslandWrapper>
  );
};

export default PipelineCanvasNodeSourceIsland;
