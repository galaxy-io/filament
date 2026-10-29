import { type FC, useMemo, useState } from "react";

import { FlowArrowIcon } from "@phosphor-icons/react";

import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, FlexVariant } from "@galaxy-io/dls/layout/Flex";
import { FontFamily, Orientation, Radius } from "@galaxy-io/dls/theme/enums";

import { ReadMode } from "@/gen/ingestion/v1/common_pb";

import BaseHeader, { BaseHeaderSize } from "@/components/BaseHeader";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import { CREATE_PIPELINE_MODAL_COLUMNS_PANE_WIDTH } from "@/pages/pipelines/components/create/constants";
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

  const locked = useMemo(
    () => [
      ...row.columns.filter((column) => column.isPrimaryKey).map((column) => column.name),
      ...(hasReadLevers && row.readMode === ReadMode.INCREMENTAL && row.cursorField
        ? [row.cursorField]
        : []),
    ],
    [row.columns, row.readMode, row.cursorField, hasReadLevers],
  );

  return (
    <Box
      position="absolute"
      inset={{ top: 0, right: 0, bottom: 0 }}
      width={CREATE_PIPELINE_MODAL_COLUMNS_PANE_WIDTH}
      variant={BoxVariant.BASE}
    >
      <Flex height="100%">
        <Divider orientation={Orientation.VERTICAL} />
        <Flex direction={FlexDirection.COLUMN} grow={1} basis={0} minWidth={0} padding={12}>
          <Flex
            direction={FlexDirection.COLUMN}
            grow={1}
            basis={0}
            minHeight={0}
            variant={FlexVariant.PRIMARY}
            hasBorder
            radius={Radius.LG}
            overflow="hidden"
          >
            <Flex alignItems={AlignItems.CENTER} padding={[8, 8, 8, 12]} shrink={0}>
              <BaseHeader
                title={row.displayName}
                family={FontFamily.MONO}
                icon={FlowArrowIcon}
                size={BaseHeaderSize.SMALL}
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
                locked={locked}
                isDisabled={false}
                search={search}
                onChange={(columns) =>
                  setResourceColumns({ sinkId: activeSinkId, resource: row.name, columns })
                }
              />
            </Flex>
          </Flex>
        </Flex>
      </Flex>
    </Box>
  );
};

export default CreatePipelineModalResourcesColumnsPane;
