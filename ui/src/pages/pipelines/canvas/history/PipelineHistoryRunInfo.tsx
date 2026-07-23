import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";

import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  TableDensity,
  TableVariant,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import EmptyLayout from "@/layouts/EmptyLayout";

import {
  RUN_RESOURCE_LOADING_ROW_COUNT,
  RUN_TABLE_COLUMN_WIDTH_RECORDS,
  RUN_TABLE_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/constants";
import { formatBytes, formatCount } from "@/pages/pipelines/utils";

import { useGetRunQuery } from "@/api/queries/runs";

import { GetRunRequestSchema, type RunResourceState } from "@/gen/ingestion/v1/runs_pb";

const ResourceTableWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  background-color: ${({ theme }) => theme.color.background.tertiary};
`);

const RESOURCE_TABLE_COLUMNS: ColumnDef<RunResourceState>[] = [
  {
    id: "resource",
    header: "Resource",
    cellLoading: () => <TextShimmer width={160} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace isEllipsis>
        {row.original.resource}
      </Text>
    ),
  },
  {
    id: "records",
    header: "Records",
    size: RUN_TABLE_COLUMN_WIDTH_RECORDS,
    align: ColumnAlign.CENTER,
    cellLoading: () => <TextShimmer width={48} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {formatCount(row.original.records)}
      </Text>
    ),
  },
  {
    id: "volume",
    header: "Volume",
    size: RUN_TABLE_COLUMN_WIDTH_VOLUME,
    align: ColumnAlign.RIGHT,
    cellLoading: () => <TextShimmer width={52} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {formatBytes(row.original.bytes)}
      </Text>
    ),
  },
];

interface PipelineHistoryRunInfoProps {
  runId: string;
}

const PipelineHistoryRunInfo = ({ runId }: PipelineHistoryRunInfoProps) => {
  const { data, isLoading, isError } = useGetRunQuery({
    input: create(GetRunRequestSchema, {
      runId,
    }),
  });

  const resources = data?.snapshot?.resources ?? [];
  const isEmpty = !isLoading && resources.length === 0;

  if (isError) {
    return (
      <FlexWrapper padding={"12px"}>
        <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
          Failed to load run details.
        </Text>
      </FlexWrapper>
    );
  }

  if (isEmpty) {
    return (
      <ResourceTableWrapper>
        <FlexWrapper padding={"32px"}>
          <EmptyLayout message="The run did not record any resource activity." />
        </FlexWrapper>
      </ResourceTableWrapper>
    );
  }

  return (
    <ResourceTableWrapper>
      <InfiniteTable<RunResourceState>
        density={TableDensity.SMALL}
        variant={TableVariant.TERTIARY}
        columns={RESOURCE_TABLE_COLUMNS}
        data={resources}
        getRowId={(resource) => resource.resource}
        isLoading={isLoading}
        loadingRowCount={RUN_RESOURCE_LOADING_ROW_COUNT}
        noLastRowPadding
        noLastRowBorder
        fillWidth
      />
    </ResourceTableWrapper>
  );
};

export default PipelineHistoryRunInfo;
