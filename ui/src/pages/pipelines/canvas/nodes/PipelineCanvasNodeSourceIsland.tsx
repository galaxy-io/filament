import { useState } from "react";

import { styled } from "@linaria/react";

import Badge, { BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import {
  PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE,
  PIPELINE_CANVAS_NODE_PADDING,
  PIPELINE_CANVAS_NODE_TABLE_LIST_MAX_HEIGHT,
} from "@/pages/pipelines/canvas/nodes/constants";
import { usePipelineCanvasNodeIslandMeasurements } from "@/pages/pipelines/canvas/nodes/hooks/usePipelineCanvasNodeIslandMeasurements";
import PipelineCanvasNodeIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeIsland";
import PipelineCanvasNodeSourceIslandHiddenHandles from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSourceIslandHiddenHandles";
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
  const [state, setState] = useState<PipelineCanvasNodeSourceIslandState>(DEFAULT_STATE);

  const filteredTables = tables.filter((table) => isSearchMatch(state.search, table.name));
  const connectedCount = tables.filter((table) => table.isConnected).length;

  const hasRows = !isLoading && !error && filteredTables.length > 0;
  const renderedNames = new Set(hasRows ? filteredTables.map((table) => table.name) : []);
  const hiddenTables = tables.filter(
    (table) => table.isConnected && !renderedNames.has(table.name),
  );

  const { listRef, badgeRef, syncMeasurements } = usePipelineCanvasNodeIslandMeasurements(
    hiddenTables.map((table) => table.name),
  );

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
    syncMeasurements();
  };

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
          <BadgeSlot ref={badgeRef}>
            <Badge count={connectedCount} variant={BadgeVariant.SECONDARY} />
          </BadgeSlot>
        )}
      </SearchSection>

      <HorizontalDivider />

      <TableList ref={listRef} className="nowheel" onScroll={syncMeasurements}>
        <PipelineCanvasNodeSourceIslandTableList
          tables={filteredTables}
          error={error}
          isLoading={isLoading}
        />
        <PipelineCanvasNodeSourceIslandHiddenHandles tables={hiddenTables} />
      </TableList>
    </IslandWrapper>
  );
};

export default PipelineCanvasNodeSourceIsland;
