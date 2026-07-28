import { styled } from "@linaria/react";
import { MagnifyingGlassIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Beacon, { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import InfiniteTable, {
  ColumnAlign,
  type ColumnDef,
  type Row,
} from "@galaxy-io/dls/table/InfiniteTable";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import EmptyLayout from "@/layouts/EmptyLayout";

import {
  PIPELINES_TABLE_COLUMN_WIDTH_CONNECTORS,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN,
  PIPELINES_TABLE_COLUMN_WIDTH_LAST_VOLUME,
  PIPELINES_TABLE_COLUMN_WIDTH_VERSION,
} from "@/pages/pipelines/components/table/constants";
import PipelinesTableFlowCell from "@/pages/pipelines/components/table/PipelinesTableFlowCell";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { formatBytes, formatTimeAgo } from "@/utils/format";

const PipelinesTableWrapper = styled.div`
  width: 100%;
  flex: 1;
  min-height: 0;
`;

const PIPELINES_TABLE_COLUMNS: ColumnDef<Pipeline>[] = [
  {
    id: "name",
    header: "Name",
    cellLoading: () => <TextShimmer width={160} height={14} />,
    cell: ({ row }) => (
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <FlexItem shrink={0} grow={0}>
          <Beacon variant={BeaconVariant.SUCCESS} />
        </FlexItem>
        <Text isEllipsis>{formatPipelineName(row.original)}</Text>
      </FlexWrapper>
    ),
  },
  {
    id: "connectors",
    header: "Connectors",
    size: PIPELINES_TABLE_COLUMN_WIDTH_CONNECTORS,
    cellLoading: () => <TextShimmer width={120} height={18} />,
    cell: ({ row }) => <PipelinesTableFlowCell pipeline={row.original} />,
  },
  {
    id: "lastRun",
    header: "Last run",
    size: PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN,
    cellLoading: () => <TextShimmer width={64} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isEllipsis>
        {row.original.lastRunAt > 0n ? formatTimeAgo(row.original.lastRunAt) : "—"}
      </Text>
    ),
  },
  {
    id: "lastVolume",
    header: "Last volume",
    size: PIPELINES_TABLE_COLUMN_WIDTH_LAST_VOLUME,
    cellLoading: () => <TextShimmer width={52} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {row.original.lastRunAt > 0n ? formatBytes(row.original.lastRunBytes) : "—"}
      </Text>
    ),
  },
  {
    id: "version",
    header: "Version",
    size: PIPELINES_TABLE_COLUMN_WIDTH_VERSION,
    align: ColumnAlign.RIGHT,
    cellLoading: () => <TextShimmer width={32} height={14} />,
    cell: ({ row }) => (
      <Text size={TextSize.BODY_SM} isMonospace>
        {row.original.currentVersionId > 0n ? row.original.currentVersionId.toString() : "—"}
      </Text>
    ),
  },
];

interface PipelinesTableProps {
  pipelines: Pipeline[];
}

const PipelinesTable = ({ pipelines }: PipelinesTableProps) => {
  const navigate = useNavigate();

  const handleRowClick = (row: Row<Pipeline>) => {
    navigate({
      to: "/pipelines/$id",
      params: { id: row.original.id },
    });
  };

  return (
    <PipelinesTableWrapper>
      <InfiniteTable<Pipeline>
        columns={PIPELINES_TABLE_COLUMNS}
        data={pipelines}
        getRowId={(pipeline) => pipeline.id}
        contentWhenEmpty={
          <EmptyLayout
            icon={<Icon component={MagnifyingGlassIcon} variant={IconVariant.TERTIARY} />}
            message="No pipelines match your search"
          />
        }
        onRowClick={handleRowClick}
        fillWidth
        fillHeight
      />
    </PipelinesTableWrapper>
  );
};

export default PipelinesTable;
