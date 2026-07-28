import { useEffect, useLayoutEffect, useRef, useState } from "react";

import { styled } from "@linaria/react";
import { WarningCircleIcon } from "@phosphor-icons/react";
import { Position, useNodeId, useReactFlow, useUpdateNodeInternals } from "@xyflow/react";

import Badge, { BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";

import {
  PIPELINE_NODE_HANDLE_SLOT_SIZE,
  PIPELINE_NODE_PADDING,
  PIPELINE_NODE_TABLE_LIST_MAX_HEIGHT,
} from "@/pages/pipelines/canvas/nodes/constants";
import {
  removePipelineNodeMeasurements,
  setPipelineNodeMeasurements,
} from "@/pages/pipelines/canvas/nodes/measurements";
import { Island } from "@/pages/pipelines/canvas/nodes/PipelineNode";
import PipelineNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineNodeHandle";
import type { PipelineSourceNodeTableInfo } from "@/pages/pipelines/canvas/types";

import { isSearchMatch } from "@/utils/search";

const IslandWrapper = styled(Island)`
  padding: 0;
`;

const SearchSection = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: ${PIPELINE_NODE_PADDING}px;
`;

const BadgeSlot = styled.span`
  width: ${PIPELINE_NODE_HANDLE_SLOT_SIZE}px;
  flex-shrink: 0;

  display: flex;
  align-items: center;
  justify-content: center;
`;

const TableList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: ${PIPELINE_NODE_PADDING}px ${PIPELINE_NODE_PADDING}px
    ${PIPELINE_NODE_PADDING}px 12px;

  max-height: ${PIPELINE_NODE_TABLE_LIST_MAX_HEIGHT}px;
  overflow-y: auto;
`;

const TableRow = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;
`;

const SHIMMER_COUNT = 5;

const TableListShimmer = () => (
  <>
    {Array.from({ length: SHIMMER_COUNT }).map((_, index) => (
      // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows with no identity
      <TextShimmer key={index} height={16} width="100%" />
    ))}
  </>
);

interface PipelineNodeSourceIslandProps {
  tables: PipelineSourceNodeTableInfo[];
  error?: Error | null;
  isLoading?: boolean;
  isSelected?: boolean;
}

interface PipelineNodeSourceIslandState {
  search: string;
}

const DEFAULT_STATE: PipelineNodeSourceIslandState = {
  search: "",
};

const PipelineNodeSourceIsland = ({
  tables,
  error,
  isLoading = false,
  isSelected,
}: PipelineNodeSourceIslandProps) => {
  const nodeId = useNodeId();
  const updateNodeInternals = useUpdateNodeInternals();
  const { getZoom } = useReactFlow();
  const listRef = useRef<HTMLDivElement>(null);
  const badgeRef = useRef<HTMLSpanElement>(null);
  const [state, setState] = useState<PipelineNodeSourceIslandState>(DEFAULT_STATE);

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  const filteredTables = tables.filter((table) => isSearchMatch(state.search, table.name));

  const publishMeasurements = () => {
    const listElement = listRef.current;
    const nodeElement = listElement?.closest(".react-flow__node");
    const zoom = getZoom();
    if (!nodeId || !listElement || !nodeElement || zoom <= 0) return;

    const nodeRect = nodeElement.getBoundingClientRect();
    const listRect = listElement.getBoundingClientRect();
    const badgeRect = badgeRef.current?.getBoundingClientRect() ?? null;
    const toNodeY = (clientY: number) => Math.round(((clientY - nodeRect.top) / zoom) * 100) / 100;

    setPipelineNodeMeasurements(nodeId, {
      listTop: toNodeY(listRect.top),
      listBottom: toNodeY(listRect.bottom),
      badgeCenterY: badgeRect ? toNodeY(badgeRect.top + badgeRect.height / 2) : null,
    });
  };

  const handleListChanged = () => {
    if (nodeId) {
      updateNodeInternals(nodeId);
    }
    publishMeasurements();
  };

  // biome-ignore lint/correctness/useExhaustiveDependencies: re-measure on content changes
  useEffect(() => {
    handleListChanged();
  }, [filteredTables.length, isLoading, error]);

  useLayoutEffect(() => {
    publishMeasurements();
  });

  useEffect(() => {
    return () => {
      if (nodeId) {
        removePipelineNodeMeasurements(nodeId);
      }
    };
  }, [nodeId]);

  const renderContent = () => {
    if (isLoading) {
      return <TableListShimmer />;
    }

    if (error) {
      return (
        <FlexWrapper padding={"20px 16px"} fillWidth>
          <ErrorLayout
            icon={<Icon component={WarningCircleIcon} size={20} variant={IconVariant.ERROR} />}
            message="Failed to load resources"
            error={error}
          />
        </FlexWrapper>
      );
    }

    if (!filteredTables.length) {
      return (
        <FlexWrapper padding={"20px 16px"} fillWidth>
          <EmptyLayout message="No tables match your search" />
        </FlexWrapper>
      );
    }

    return filteredTables.map((table) => (
      <TableRow key={table.name}>
        <Text
          size={TextSize.BODY_SM}
          variant={table.isConnected ? TextVariant.SECONDARY : TextVariant.TERTIARY}
          isMonospace
        >
          {table.name}
        </Text>
        <PipelineNodeHandle
          id={table.name}
          kind={ConnectorKind.SOURCE}
          position={Position.Right}
          isConnected={table.isConnected}
        />
      </TableRow>
    ));
  };

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
          <BadgeSlot ref={badgeRef}>
            <Badge count={connectedCount} variant={BadgeVariant.SECONDARY} />
          </BadgeSlot>
        )}
      </SearchSection>

      <HorizontalDivider />

      <TableList ref={listRef} className="nowheel" onScroll={handleListChanged}>
        {renderContent()}
      </TableList>
    </IslandWrapper>
  );
};

export default PipelineNodeSourceIsland;
