import { useEffect, useState } from "react";

import { styled } from "@linaria/react";
import { WarningCircleIcon } from "@phosphor-icons/react";
import { Position, useNodeId, useUpdateNodeInternals } from "@xyflow/react";

import Badge, { BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";

import {
  PIPELINE_NODE_HANDLE_SLOT_SIZE,
  PIPELINE_NODE_PADDING,
  PIPELINE_NODE_TABLE_LIST_MAX_HEIGHT,
} from "@/pages/pipelines/canvas/constants";
import { Island } from "@/pages/pipelines/canvas/nodes/PipelineNode";
import PipelineNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineNodeHandle";
import type { PipelineNodeSourceTableInfo } from "@/pages/pipelines/canvas/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

const IslandWrapper = styled(Island)`
  padding: 0;
`;

const SearchSection = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: ${PIPELINE_NODE_PADDING}px;
`;

// Same fixed slot as the handles so the badge centers on the handle axis
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
  tables: PipelineNodeSourceTableInfo[];
  error?: Error | null;
  isLoading?: boolean;
  isSelected?: boolean;
}

const PipelineNodeSourceIsland = ({
  tables,
  error,
  isLoading = false,
  isSelected,
}: PipelineNodeSourceIslandProps) => {
  const nodeId = useNodeId();
  const updateNodeInternals = useUpdateNodeInternals();
  const [searchQuery, setSearchQuery] = useState("");

  const filteredTables = tables.filter((table) =>
    table.name.toLowerCase().includes(searchQuery.toLowerCase()),
  );

  // React Flow measures handle positions once per node; re-measure whenever rows
  // move (scroll) or the list content changes so edges stay anchored to their rows
  const handleListChanged = () => {
    if (nodeId) {
      updateNodeInternals(nodeId);
    }
  };

  // biome-ignore lint/correctness/useExhaustiveDependencies: re-measure on content changes
  useEffect(() => {
    handleListChanged();
  }, [filteredTables.length, isLoading, error]);

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
        <TextInput placeholder="Search" value={searchQuery} onChange={setSearchQuery} fillWidth />
        {/* Edges from rows scrolled out of view anchor to this badge */}
        {connectedCount > 0 && (
          <BadgeSlot data-resource-badge>
            <Badge count={connectedCount} variant={BadgeVariant.SECONDARY} />
          </BadgeSlot>
        )}
      </SearchSection>

      <HorizontalDivider />

      {/* nowheel: scrolling the list shouldn't zoom the canvas */}
      <TableList className="nowheel" data-table-list onScroll={handleListChanged}>
        {renderContent()}
      </TableList>
    </IslandWrapper>
  );
};

export default PipelineNodeSourceIsland;
