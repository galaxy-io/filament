import { useState } from "react";

import { styled } from "@linaria/react";
import { WarningCircleIcon } from "@phosphor-icons/react";
import { Position } from "@xyflow/react";

import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import ErrorLayout from "@/layouts/ErrorLayout";

import { PIPELINE_NODE_PADDING } from "@/pages/pipelines/canvas/constants";
import { Island } from "@/pages/pipelines/canvas/nodes/PipelineNode";
import PipelineNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineNodeHandle";
import type { PipelineNodeSourceTableInfo } from "@/pages/pipelines/canvas/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";

const IslandWrapper = styled(Island)`
  padding: 0;
`;

const SearchSection = styled.div`
  padding: ${PIPELINE_NODE_PADDING}px;
`;

const TableList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: ${PIPELINE_NODE_PADDING}px ${PIPELINE_NODE_PADDING}px
    ${PIPELINE_NODE_PADDING}px 12px;
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
  const [searchQuery, setSearchQuery] = useState("");

  const filteredTables = tables.filter((table) =>
    table.name.toLowerCase().includes(searchQuery.toLowerCase()),
  );

  const renderContent = () => {
    if (isLoading) {
      return <TableListShimmer />;
    }

    if (error) {
      return (
        <FlexWrapper padding={"20px 16px"} fillWidth>
          <ErrorLayout
            icon={
              <Icon
                component={WarningCircleIcon}
                size={20}
                variant={IconVariant.ERROR}
              />
            }
            message="Failed to load resources"
          />
        </FlexWrapper>
      );
    }

    return filteredTables.map((table) => (
      <TableRow key={table.name}>
        <Text
          size={TextSize.BODY_SM}
          variant={
            table.isConnected ? TextVariant.SECONDARY : TextVariant.TERTIARY
          }
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

  return (
    <IslandWrapper $isSelected={isSelected}>
      <SearchSection className="nodrag">
        <TextInput
          placeholder="Search"
          value={searchQuery}
          onChange={setSearchQuery}
          fillWidth
        />
      </SearchSection>

      <HorizontalDivider />

      <TableList>{renderContent()}</TableList>
    </IslandWrapper>
  );
};

export default PipelineNodeSourceIsland;
