import { type FC, Fragment } from "react";

import { ArrowRightIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Grid, { GridAlignItems } from "@galaxy-io/dls/layout/Grid";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily, Radius } from "@galaxy-io/dls/theme/enums";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import { formatRelativeTime } from "@galaxy-io/dls/utils/format";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineName from "@/components/pipelines/PipelineName";
import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";

import {
  OBSERVABILITY_FAILURES_DETAILS_MAX_HEIGHT,
  OBSERVABILITY_FAILURES_DETAILS_WIDTH,
} from "@/pages/observability/components/failures/constants";
import type { ObservabilityPipelineEntry } from "@/pages/observability/types";

import { useFilamentNavigate } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";

import { formatTimestamp } from "@/utils/format";

interface ObservabilityFailuresDetailsProps {
  entries: ObservabilityPipelineEntry[];
}

const ObservabilityFailuresDetails: FC<ObservabilityFailuresDetailsProps> = ({ entries }) => {
  const navigate = useFilamentNavigate();

  const handleViewRun = (pipeline: Pipeline) => {
    if (!pipeline.lastRun) {
      return;
    }
    void navigate({
      to: FilamentPath.PIPELINE_HISTORY,
      params: { id: pipeline.id },
      search: { runId: [pipeline.lastRun.id] },
    });
  };

  return (
    <Flex
      direction={FlexDirection.COLUMN}
      width={OBSERVABILITY_FAILURES_DETAILS_WIDTH}
      maxHeight={OBSERVABILITY_FAILURES_DETAILS_MAX_HEIGHT}
    >
      <FlexItem grow={1} minHeight={0}>
        <ScrollArea>
          {entries.map(({ pipeline }, index) => (
            <Fragment key={pipeline.id}>
              {index > 0 && <Divider />}
              <Box padding={[12, 16]} fillWidth>
                <Grid
                  columns="auto minmax(0, 1fr)"
                  gap={[8, 8]}
                  alignItems={GridAlignItems.CENTER}
                  fillWidth
                >
                  <PipelineRunStatusSwatch status={RunStatus.FAILED} />
                  <Flex alignItems={AlignItems.CENTER} gap={8} fillWidth>
                    <FlexItem grow={1} minWidth={0}>
                      <PipelineName pipelineId={pipeline.id} pipeline={pipeline} />
                    </FlexItem>
                    {pipeline.lastRun && (
                      <Tooltip body={formatTimestamp(pipeline.lastRun.requestedAt)}>
                        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                          {formatRelativeTime(pipeline.lastRun.requestedAt)}
                        </Text>
                      </Tooltip>
                    )}
                    <Button
                      label="View run"
                      size={ButtonSize.SMALL}
                      variant={ButtonVariant.TERTIARY}
                      icon={ArrowRightIcon}
                      isIconTrailing
                      onClick={() => handleViewRun(pipeline)}
                    />
                  </Flex>
                  {pipeline.lastRun?.error && (
                    <>
                      <Box />
                      <Box
                        variant={BoxVariant.SECONDARY}
                        radius={Radius.MD}
                        padding={[8, 12]}
                        fillWidth
                      >
                        <Text size={TextSize.CAPTION} family={FontFamily.MONO} isSelectable>
                          {pipeline.lastRun.error}
                        </Text>
                      </Box>
                    </>
                  )}
                </Grid>
              </Box>
            </Fragment>
          ))}
        </ScrollArea>
      </FlexItem>
    </Flex>
  );
};

export default ObservabilityFailuresDetails;
