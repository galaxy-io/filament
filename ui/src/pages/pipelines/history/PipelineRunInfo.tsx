import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  TableVariant,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  GetRunRequestSchema,
  type RunInfo,
  type RunResourceState,
} from "@/gen/ingestion/v1/runs_pb";

import EmptyLayout from "@/layouts/EmptyLayout";
import ErrorLayout from "@/layouts/ErrorLayout";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import {
  PIPELINE_RUN_RESOURCE_LOADING_ROW_COUNT,
  PIPELINE_RUN_TABLE_COLUMN_WIDTH_RECORDS,
  PIPELINE_RUN_TABLE_COLUMN_WIDTH_VOLUME,
} from "@/pages/pipelines/history/constants";

import { useGetRunQuery } from "@/api/queries/runs";

import { formatBytes, formatCount } from "@/utils/format";

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
    cell: ({ row }) => {
      return (
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
          <ConnectorTile connector={row.original.resource} size={ConnectorTileSize.SMALL} />
          <Text size={TextSize.BODY_SM} variant={TextVariant.PRIMARY}>
            connection_name
          </Text>
          <Text size={TextSize.BODY_SM} isMonospace>
            /
          </Text>
          <Text size={TextSize.BODY_SM} isMonospace isEllipsis>
            {row.original.resource}
          </Text>
        </FlexWrapper>
      );
    },
  },
  {
    id: "records",
    header: "Records",
    size: PIPELINE_RUN_TABLE_COLUMN_WIDTH_RECORDS,
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
    size: PIPELINE_RUN_TABLE_COLUMN_WIDTH_VOLUME,
    align: ColumnAlign.RIGHT,
    cellLoading: () => <TextShimmer width={52} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {formatBytes(row.original.bytes)}
      </Text>
    ),
  },
];

interface PipelineRunInfoProps {
  runId: RunInfo["runId"];
}

const PipelineRunInfo = ({ runId }: PipelineRunInfoProps) => {
  const { data, isLoading, isError } = useGetRunQuery({
    input: create(GetRunRequestSchema, { runId }),
  });

  const resources = data?.snapshot?.resources ?? [];
  const isEmpty = !isLoading && resources.length === 0;

  if (isError) {
    return (
      <ResourceTableWrapper>
        <FlexWrapper height={240}>
          <ErrorLayout message="Failed to load run details." />
        </FlexWrapper>
      </ResourceTableWrapper>
    );
  }

  if (isEmpty) {
    return (
      <ResourceTableWrapper>
        <FlexWrapper height={240}>
          <EmptyLayout message="The run did not record any resource activity." />
        </FlexWrapper>
      </ResourceTableWrapper>
    );
  }

  return (
    <ResourceTableWrapper>
      <InfiniteTable<RunResourceState>
        variant={TableVariant.TERTIARY}
        columns={RESOURCE_TABLE_COLUMNS}
        data={resources}
        getRowId={(resource) => resource.resource}
        isLoading={isLoading}
        loadingRowCount={PIPELINE_RUN_RESOURCE_LOADING_ROW_COUNT}
        noLastRowPadding
        noLastRowBorder
        fillWidth
      />
    </ResourceTableWrapper>
  );
};

export default PipelineRunInfo;
