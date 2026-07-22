import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { WarningCircleIcon } from "@phosphor-icons/react";
import { useParams } from "@tanstack/react-router";

import Beacon from "@galaxy-io/dls/beacons/Beacon";
import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import InfiniteTable, { ColumnAlign, type ColumnDef } from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import BaseHeader from "@/layouts/components/BaseHeader";
import { BaseHeaderSize } from "@/layouts/components/types";
import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";

import {
  RUN_HISTORY_LIMIT,
  RUN_HISTORY_LOADING_ROW_COUNT,
  RUN_STATUS_TO_BEACON_VARIANT_MAP,
  RUN_STATUS_TO_LABEL_MAP,
  RUN_STATUS_TO_TEXT_VARIANT_MAP,
  RUN_TABLE_COLUMN_WIDTH_RECORDS,
  RUN_TABLE_COLUMN_WIDTH_STATUS,
  RUN_TABLE_COLUMN_WIDTH_VERSION,
  RUN_TABLE_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/constants";
import { formatBytes, formatCount } from "@/pages/pipelines/utils";

import { useListRunsQuery } from "@/api/queries/runs";

import { ListRunsRequestSchema, type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

const PageWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  display: flex;
  flex-direction: column;

  overflow: hidden;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const RunTableWrapper = styled.div`
  width: 100%;
  flex: 1;
  min-height: 0;
`;

interface RunCellProps {
  run: RunInfo;
}

const StatusCell = ({ run }: RunCellProps) => {
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
      <Beacon
        variant={RUN_STATUS_TO_BEACON_VARIANT_MAP[run.status]}
        isPulse={run.status === RunStatus.RUNNING}
      />
      <Text size={TextSize.BODY_SM} variant={RUN_STATUS_TO_TEXT_VARIANT_MAP[run.status]}>
        {RUN_STATUS_TO_LABEL_MAP[run.status]}
      </Text>
    </FlexWrapper>
  );
};

const RunIdCell = ({ run }: RunCellProps) => {
  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={2}>
      <Text size={TextSize.BODY_SM} isMonospace isEllipsis>
        {run.runId}
      </Text>
      {run.error && (
        <Text size={TextSize.CAPTION} variant={TextVariant.ERROR} isEllipsis>
          {run.error}
        </Text>
      )}
    </FlexWrapper>
  );
};

const RUN_TABLE_COLUMNS: ColumnDef<RunInfo>[] = [
  {
    id: "status",
    header: "Status",
    size: RUN_TABLE_COLUMN_WIDTH_STATUS,
    cellLoading: () => <TextShimmer width={64} height={18} />,
    cell: ({ row }) => <StatusCell run={row.original} />,
  },
  {
    id: "run",
    header: "Run",
    cellLoading: () => <TextShimmer width={160} height={14} />,
    cell: ({ row }) => <RunIdCell run={row.original} />,
  },
  {
    id: "version",
    header: "Version",
    size: RUN_TABLE_COLUMN_WIDTH_VERSION,
    align: ColumnAlign.RIGHT,
    cellLoading: () => <TextShimmer width={32} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {row.original.pipelineVersionId ? `v${row.original.pipelineVersionId}` : "—"}
      </Text>
    ),
  },
  {
    id: "records",
    header: "Records",
    size: RUN_TABLE_COLUMN_WIDTH_RECORDS,
    align: ColumnAlign.RIGHT,
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

const PipelineHistoryPage = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

  const { data, isLoading, isError } = useListRunsQuery({
    input: create(ListRunsRequestSchema, { pipelineId: id, limit: RUN_HISTORY_LIMIT }),
  });

  return (
    <PageWrapper>
      <Wrapper padding={"16px"} fillWidth>
        <BaseHeader
          size={BaseHeaderSize.LARGE}
          title="History"
          description="Review recent pipeline runs."
        />
      </Wrapper>

      <HorizontalDivider />

      <RunTableWrapper>
        <InfiniteTable<RunInfo>
          columns={RUN_TABLE_COLUMNS}
          data={data?.runs ?? []}
          getRowId={(run) => run.runId}
          isLoading={isLoading}
          loadingRowCount={RUN_HISTORY_LOADING_ROW_COUNT}
          isError={isError}
          contentWhenError={
            <ErrorLayout
              icon={<Icon component={WarningCircleIcon} size={20} variant={IconVariant.ERROR} />}
              message="Failed to load runs. Please try again."
            />
          }
          contentWhenEmpty={
            <EmptyLayout header="No runs yet" message="Run a pipeline to see its history here." />
          }
          fillWidth
          fillHeight
        />
      </RunTableWrapper>
    </PageWrapper>
  );
};

export default PipelineHistoryPage;
