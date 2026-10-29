import { type FC, useState } from "react";

import { FlowArrowIcon } from "@phosphor-icons/react";

import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import BaseHeader, { BaseHeaderSize } from "@/components/BaseHeader";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import { CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_WIDTH } from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalResourcesColumnsPaneFooter from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesColumnsPaneFooter";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";
import PipelineResourceColumnsChip from "@/pages/pipelines/components/resource/PipelineResourceColumnsChip";
import PipelineResourceColumnsTable from "@/pages/pipelines/components/resource/PipelineResourceColumnsTable";

interface CreatePipelineModalResourcesColumnsPaneProps {
  row: CreatePipelineModalResourceRow;
  onClose: () => void;
}

const CreatePipelineModalResourcesColumnsPane: FC<CreatePipelineModalResourcesColumnsPaneProps> = ({
  row,
  onClose,
}) => {
  const { activeSinkId, hasReadLevers } = useCreatePipelineModalState();
  const { setResourceColumns } = useCreatePipelineModalActions();

  const [search, setSearch] = useState("");

  return (
    <Box width={CREATE_PIPELINE_MODAL_RESOURCES_COLUMNS_PANE_WIDTH} height="100%">
      <Flex direction={FlexDirection.COLUMN} height="100%">
        <Flex alignItems={AlignItems.CENTER} padding={[12, 12, 12, 16]} shrink={0}>
          <BaseHeader
            title={row.displayName}
            family={FontFamily.MONO}
            icon={FlowArrowIcon}
            size={BaseHeaderSize.MEDIUM}
            actions={[
              <PipelineResourceColumnsChip
                key="columns"
                selectedCount={row.selectedColumns.length}
                totalCount={row.columns.length}
              />,
            ]}
            onClose={onClose}
          />
        </Flex>
        <Divider />
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
        <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} minHeight={0}>
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
