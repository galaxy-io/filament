import { useState } from "react";

import { styled } from "@linaria/react";
import { Position } from "@xyflow/react";

import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  PIPELINE_NODE_GAP,
  PIPELINE_NODE_HANDLE_SLOT_SIZE,
  PIPELINE_NODE_PADDING,
} from "@/pages/pipelines/canvas/constants";
import { Island } from "@/pages/pipelines/canvas/nodes/PipelineNode";
import PipelineNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineNodeHandle";
import type { PipelineNodeSourceTableInfo } from "@/pages/pipelines/canvas/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

const IslandWrapper = styled(Island)`
  margin-top: ${PIPELINE_NODE_GAP}px;
`;

const Divider = withTheme(styled.div<PropsWithTheme>`
  height: 0.5px;
  margin: ${PIPELINE_NODE_PADDING}px -${PIPELINE_NODE_PADDING}px;
  background-color: ${({ theme }) => theme.color.border.primary};
`);

const TableList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 0px 0px 0px 4px;
`;

const TableRow = styled.div`
  display: flex;
  align-items: center;
  gap: 6px;
`;

const TableContent = styled.div`
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: space-between;
`;

// 24x24 container that centers the handle
const HandleSlot = styled.div`
  width: ${PIPELINE_NODE_HANDLE_SLOT_SIZE}px;
  height: ${PIPELINE_NODE_HANDLE_SLOT_SIZE}px;

  display: flex;
  align-items: center;
  justify-content: center;
`;

interface PipelineNodeSourceIslandProps {
  tables: PipelineNodeSourceTableInfo[];
  isSelected?: boolean;
}

const PipelineNodeSourceIsland = ({ tables, isSelected }: PipelineNodeSourceIslandProps) => {
  const [searchQuery, setSearchQuery] = useState("");

  const filteredTables = tables.filter((table) =>
    table.name.toLowerCase().includes(searchQuery.toLowerCase()),
  );

  return (
    <IslandWrapper $isSelected={isSelected}>
      <TextInput placeholder="Search" value={searchQuery} onChange={setSearchQuery} fillWidth />

      <Divider />

      <TableList>
        {filteredTables.map((table) => (
          <TableRow key={table.name}>
            <TableContent>
              <Text
                size={TextSize.BODY_SM}
                variant={table.isConnected ? TextVariant.SECONDARY : TextVariant.TERTIARY}
                isMonospace
              >
                {table.name}
              </Text>
            </TableContent>
            <HandleSlot>
              <PipelineNodeHandle
                id={table.name}
                kind={ConnectorKind.SOURCE}
                position={Position.Right}
                isConnected={table.isConnected}
              />
            </HandleSlot>
          </TableRow>
        ))}
      </TableList>
    </IslandWrapper>
  );
};

export default PipelineNodeSourceIsland;
