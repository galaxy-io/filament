import type { FC } from "react";
import { Fragment } from "react";

import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Grid, { GridAlignItems } from "@galaxy-io/dls/layout/Grid";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { formatBytes, formatNumber } from "@galaxy-io/dls/utils/format";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import { getPipelineHistoryRunTimestamp } from "@/pages/pipelines/history/utils";

import { formatTimestamp } from "@/utils/format";
import { formatRunDuration } from "@/utils/runs";

interface PipelinesTableColumnRecentRunsTooltipProps {
  run: RunInfo;
}

const PipelinesTableColumnRecentRunsTooltip: FC<PipelinesTableColumnRecentRunsTooltipProps> = ({
  run,
}) => {
  const rows = [
    { label: "Duration", value: formatRunDuration(run.startedAt, run.endedAt) },
    { label: "Records", value: formatNumber(run.records) },
    { label: "Volume", value: formatBytes(run.bytes) },
  ];

  return (
    <Box minWidth={160}>
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={8} fillWidth>
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} lineClamp={1}>
          {formatTimestamp(getPipelineHistoryRunTimestamp(run).timestamp)}
        </Text>
        {run.error ? (
          <Text size={TextSize.CAPTION} family={FontFamily.MONO} isSelectable>
            {run.error}
          </Text>
        ) : (
          <Grid
            columns="minmax(0, 1fr) auto"
            gap={[4, 16]}
            alignItems={GridAlignItems.CENTER}
            fillWidth
          >
            {rows.map((row) => (
              <Fragment key={row.label}>
                <Text size={TextSize.BODY_SM} lineClamp={1}>
                  {row.label}
                </Text>
                <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} align="right">
                  {row.value}
                </Text>
              </Fragment>
            ))}
          </Grid>
        )}
      </Flex>
    </Box>
  );
};

export default PipelinesTableColumnRecentRunsTooltip;
