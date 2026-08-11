import { useMemo } from "react";

import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";

import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import InfiniteTable, {
  type ColumnDef,
  ColumnPin,
  type Row,
  TableVariant,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import EmptyLayout from "@/layouts/EmptyLayout";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import {
  CREATE_PIPELINE_MODAL_COLUMN_WIDTH_CURSOR,
  CREATE_PIPELINE_MODAL_COLUMN_WIDTH_READ_MODE,
  CREATE_PIPELINE_MODAL_RESOURCE_LOADING_ROW_COUNT,
} from "@/pages/pipelines/components/create/constants";
import CreatePipelineModalResourcesCursorCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesCursorCell";
import CreatePipelineModalResourcesReadModeCell from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalResourcesReadModeCell";
import type { CreatePipelineModalResourceRow } from "@/pages/pipelines/components/create/types";

const TableWrapper = styled.div`
  width: 100%;
  flex: 1;
  min-height: 0;
`;

const NameWrapper = styled.div`
  min-width: 0;
  overflow: hidden;
`;

const NAME_COLUMN: ColumnDef<CreatePipelineModalResourceRow> = {
  id: "name",
  header: "Resource",
  accessorFn: (row) => row.displayName,
  enableSorting: true,
  cellLoading: () => <TextShimmer width={180} height={16} />,
  cell: ({ row }) => (
    <NameWrapper>
      <Text size={TextSize.BODY_SM} isMonospace isEllipsis>
        {row.original.displayName}
      </Text>
    </NameWrapper>
  ),
};

const RESOURCE_COLUMNS_BASE: ColumnDef<CreatePipelineModalResourceRow>[] = [NAME_COLUMN];

const RESOURCE_COLUMNS_WITH_LEVERS: ColumnDef<CreatePipelineModalResourceRow>[] = [
  NAME_COLUMN,
  {
    id: "readMode",
    header: "Read mode",
    size: CREATE_PIPELINE_MODAL_COLUMN_WIDTH_READ_MODE,
    pin: ColumnPin.RIGHT,
    cellLoading: () => <TextShimmer width={120} height={16} />,
    cell: ({ row }) => <CreatePipelineModalResourcesReadModeCell row={row.original} />,
  },
  {
    id: "cursor",
    header: "Cursor",
    size: CREATE_PIPELINE_MODAL_COLUMN_WIDTH_CURSOR,
    pin: ColumnPin.RIGHT,
    cellLoading: () => <TextShimmer width={140} height={16} />,
    cell: ({ row }) => <CreatePipelineModalResourcesCursorCell row={row.original} />,
  },
];

interface CreatePipelineModalResourcesTableProps {
  rows: CreatePipelineModalResourceRow[];
}

const CreatePipelineModalResourcesTable = ({ rows }: CreatePipelineModalResourcesTableProps) => {
  const { activeSinkId, isCdc, isLoading } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  const hasLevers = !isCdc;

  const rowSelection = useMemo(
    () => Object.fromEntries(rows.map((row) => [row.name, row.isSelected])),
    [rows],
  );

  const columns = hasLevers ? RESOURCE_COLUMNS_WITH_LEVERS : RESOURCE_COLUMNS_BASE;

  return (
    <TableWrapper>
      <InfiniteTable<CreatePipelineModalResourceRow>
        variant={TableVariant.PRIMARY}
        columns={columns}
        data={rows}
        getRowId={(row) => row.name}
        isLoading={isLoading}
        loadingRowCount={CREATE_PIPELINE_MODAL_RESOURCE_LOADING_ROW_COUNT}
        rowSelection={rowSelection}
        onRowSelectionChange={(updater) =>
          dispatch({
            type: CreatePipelineModalActionType.SET_RESOURCE_SELECTION,
            payload: {
              sinkId: activeSinkId,
              visibleNames: rows.map((row) => row.name),
              selection: typeof updater === "function" ? updater(rowSelection) : updater,
            },
          })
        }
        enableRowSelection={(row: Row<CreatePipelineModalResourceRow>) => row.original.isSelectable}
        getRowSelectAriaLabel={(row: Row<CreatePipelineModalResourceRow>) => row.original.name}
        contentWhenEmpty={
          <FlexWrapper
            direction={FlexDirection.COLUMN}
            alignItems={AlignItems.CENTER}
            padding={24}
            fillWidth
          >
            <EmptyLayout
              icon={<Icon component={MagnifyingGlassIcon} variant={IconVariant.TERTIARY} />}
              message="No resources match your search"
            />
          </FlexWrapper>
        }
        showSelectionColumn
        enableSelectAll
        enableSelectionRange
        fillWidth
        fillHeight
      />
    </TableWrapper>
  );
};

export default CreatePipelineModalResourcesTable;
