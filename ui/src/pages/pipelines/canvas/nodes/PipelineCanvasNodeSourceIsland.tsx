import { type FC, useState } from "react";

import { styled } from "@linaria/react";

import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import { isSearchMatch } from "@galaxy-io/dls/utils/search";

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

const IslandWrapper = styled(PipelineCanvasNodeIsland)`
  padding: 0;
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

const PipelineCanvasNodeSourceIsland: FC<PipelineCanvasNodeSourceIslandProps> = ({
  tables,
  error,
  isLoading = false,
  isSelected,
}) => {
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
      <Flex
        alignItems={AlignItems.CENTER}
        gap={8}
        padding={PIPELINE_CANVAS_NODE_PADDING}
        className="nodrag"
      >
        <SearchInput value={state.search} onChange={handleSearchChange} fillWidth />
        {connectedCount > 0 && (
          <Flex
            ref={badgeRef}
            as="span"
            width={PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE}
            shrink={0}
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.CENTER}
          >
            <Chip hasBorder isPill count={connectedCount} variant={ChipVariant.SECONDARY} />
          </Flex>
        )}
      </Flex>

      <Divider />

      <Box maxHeight={PIPELINE_CANVAS_NODE_TABLE_LIST_MAX_HEIGHT}>
        <ScrollArea ref={listRef} onScroll={syncMeasurements} className="nowheel">
          <Flex
            direction={FlexDirection.COLUMN}
            gap={4}
            padding={[
              PIPELINE_CANVAS_NODE_PADDING,
              PIPELINE_CANVAS_NODE_PADDING,
              PIPELINE_CANVAS_NODE_PADDING,
              12,
            ]}
          >
            <PipelineCanvasNodeSourceIslandTableList
              tables={filteredTables}
              error={error}
              isLoading={isLoading}
            />
            <PipelineCanvasNodeSourceIslandHiddenHandles tables={hiddenTables} />
          </Flex>
        </ScrollArea>
      </Box>
    </IslandWrapper>
  );
};

export default PipelineCanvasNodeSourceIsland;
