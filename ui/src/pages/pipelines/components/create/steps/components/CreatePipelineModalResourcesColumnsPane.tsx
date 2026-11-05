import { type FC, useState } from "react";

import { ColumnsIcon, FlowArrowIcon, SquaresFourIcon } from "@phosphor-icons/react";

import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { ReadMode } from "@/gen/ingestion/v1/common_pb";

import BaseHeader, { BaseHeaderSize } from "@/components/BaseHeader";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import { CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_WIDTH } from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalResourcesColumnsPaneFooter from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesColumnsPaneFooter";
import CreatePipelineModalResourcesPaneSection from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesPaneSection";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";
import PipelineResourceColumnsChip from "@/pages/pipelines/components/resource/PipelineResourceColumnsChip";
import PipelineResourceColumnsTable from "@/pages/pipelines/components/resource/PipelineResourceColumnsTable";
import PipelineResourcePartitionsChip from "@/pages/pipelines/components/resource/PipelineResourcePartitionsChip";
import PipelineResourcePartitionsTable from "@/pages/pipelines/components/resource/PipelineResourcePartitionsTable";

interface CreatePipelineModalResourcesColumnsPaneProps {
  row: CreatePipelineModalResourceRow;
  onClose: () => void;
}

const CreatePipelineModalResourcesColumnsPane: FC<CreatePipelineModalResourcesColumnsPaneProps> = ({
  row,
  onClose,
}) => {
  const { activeSinkId, hasReadLevers } = useCreatePipelineModalState();
  const { setResourceColumns, setResourcePartitions } = useCreatePipelineModalActions();

  const [search, setSearch] = useState("");
  const [partitionSearch, setPartitionSearch] = useState("");
  const hasPartitions = row.partitions.length > 0 && row.readMode !== ReadMode.INCREMENTAL;

  return (
    <Box width={CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_WIDTH} height="100%">
      <Flex direction={FlexDirection.COLUMN} height="100%">
        <Flex alignItems={AlignItems.CENTER} padding={[12, 12, 12, 16]} shrink={0}>
          <BaseHeader
            title={row.displayName}
            family={FontFamily.MONO}
            icon={FlowArrowIcon}
            size={BaseHeaderSize.MEDIUM}
            onClose={onClose}
          />
        </Flex>
        <Divider />
        <FlexItem grow={1} basis={0} minHeight={0}>
          <ScrollArea>
            <Flex direction={FlexDirection.COLUMN} gap={8} padding={12}>
              <CreatePipelineModalResourcesPaneSection
                icon={ColumnsIcon}
                header="Columns"
                trailing={
                  <PipelineResourceColumnsChip
                    selectedCount={row.selectedColumns.length}
                    totalCount={row.columns.length}
                  />
                }
              >
                <Box padding={8}>
                  <SearchInput
                    value={search}
                    onChange={setSearch}
                    placeholder="Search columns..."
                    ariaLabel="Search columns"
                    fillWidth
                  />
                </Box>
                <Divider />
                <Flex direction={FlexDirection.COLUMN} shrink={1} minHeight={0}>
                  <PipelineResourceColumnsTable
                    columns={row.columns}
                    selected={row.selectedColumns}
                    locked={row.lockedColumns}
                    isDisabled={!row.isSelected}
                    search={search}
                    onChange={(columns) =>
                      setResourceColumns({ sinkId: activeSinkId, resource: row.name, columns })
                    }
                  />
                </Flex>
              </CreatePipelineModalResourcesPaneSection>
              {hasPartitions && (
                <>
                  <CreatePipelineModalResourcesPaneSection
                    icon={SquaresFourIcon}
                    header="Partitions"
                    trailing={
                      <PipelineResourcePartitionsChip
                        selectedCount={row.selectedPartitions.length}
                        totalCount={row.partitions.length}
                      />
                    }
                  >
                    <Box padding={8}>
                      <SearchInput
                        value={partitionSearch}
                        onChange={setPartitionSearch}
                        placeholder="Search partitions..."
                        ariaLabel="Search partitions"
                        fillWidth
                      />
                    </Box>
                    <Divider />
                    <Flex direction={FlexDirection.COLUMN} shrink={1} minHeight={0}>
                      <PipelineResourcePartitionsTable
                        partitions={row.partitions}
                        search={partitionSearch}
                        selected={row.selectedPartitions}
                        isDisabled={!row.isSelected}
                        onChange={(partitions) =>
                          setResourcePartitions({
                            sinkId: activeSinkId,
                            resource: row.name,
                            partitions,
                          })
                        }
                      />
                    </Flex>
                  </CreatePipelineModalResourcesPaneSection>
                </>
              )}
            </Flex>
          </ScrollArea>
        </FlexItem>
        {hasReadLevers && (
          <>
            <Divider />
            <CreatePipelineModalResourcesColumnsPaneFooter row={row} />
          </>
        )}
      </Flex>
    </Box>
  );
};

export default CreatePipelineModalResourcesColumnsPane;
