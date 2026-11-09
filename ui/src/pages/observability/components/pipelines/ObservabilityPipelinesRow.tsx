import type { FC } from "react";

import { styled } from "@linaria/react";

import Beacon, { BeaconSize } from "@galaxy-io/dls/beacons/Beacon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import { FOCUS_RING, HAIRLINE_WIDTH, TRUNCATE } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import { EMPTY_VALUE, formatNumber, formatRelativeTime } from "@galaxy-io/dls/utils/format";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineFlow, { PipelineFlowSize } from "@/components/pipelines/PipelineFlow";
import PipelineName from "@/components/pipelines/PipelineName";
import PipelineScheduleIndicator from "@/components/pipelines/PipelineScheduleIndicator";
import { mapVersionNodesToFlowEndpoints } from "@/components/pipelines/utils";
import RouterLink from "@/components/RouterLink";

import {
  OBSERVABILITY_PIPELINE_HEALTH_TO_LAST_RUN_VARIANT_MAP,
  OBSERVABILITY_PIPELINES_COLUMNS,
  OBSERVABILITY_PIPELINES_ROW_HEIGHT,
} from "@/pages/observability/components/pipelines/constants";
import ObservabilityPipelinesBar from "@/pages/observability/components/pipelines/ObservabilityPipelinesBar";
import ObservabilityPipelinesColumnCell from "@/pages/observability/components/pipelines/ObservabilityPipelinesColumnCell";
import {
  type ObservabilityPipelinesCell,
  ObservabilityPipelinesColumn,
} from "@/pages/observability/components/pipelines/types";
import { ObservabilityPipelineHealth } from "@/pages/observability/types";

import { createFilamentHref, FilamentPath } from "@/module/paths";

import { formatTimestamp } from "@/utils/format";

const ObservabilityPipelinesRowFrame = styled.div`
  position: relative;
  display: grid;
  grid-template-columns: ${OBSERVABILITY_PIPELINES_COLUMNS};
  column-gap: ${t.space[16]};
  align-items: center;
  height: ${OBSERVABILITY_PIPELINES_ROW_HEIGHT}px;
  padding: 0 ${t.space[12]};
  border-bottom: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  transition: background-color ${t.duration.fast};

  &:hover {
    background-color: ${t.color.background.hovered};
  }
`;

const ObservabilityPipelinesRowLink = styled(RouterLink)`
  ${TRUNCATE}
  ${FOCUS_RING}
  color: inherit;
  text-decoration: none;

  &::after {
    content: "";
    position: absolute;
    inset: 0;
  }
`;

const ObservabilityPipelinesRowRaised = styled.div`
  position: relative;
  z-index: 1;
  display: flex;
  gap: ${t.space[4]};
`;

interface ObservabilityPipelinesRowProps {
  pipeline: Pipeline;
  health: ObservabilityPipelineHealth;
  cells: ObservabilityPipelinesCell[];
}

const ObservabilityPipelinesRow: FC<ObservabilityPipelinesRowProps> = ({
  pipeline,
  health,
  cells,
}) => {
  const { lastRun } = pipeline;
  const graph = pipeline.currentVersion?.graph;
  const { sourceId, sinkIds } = mapVersionNodesToFlowEndpoints(graph?.nodes ?? []);

  const renderLastRun = () => {
    if (health === ObservabilityPipelineHealth.RUNNING) {
      return <Beacon color="blue" size={BeaconSize.SMALL} label="Running" isPulse />;
    }
    if (!lastRun) {
      return (
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
          {EMPTY_VALUE}
        </Text>
      );
    }
    return (
      <ObservabilityPipelinesRowRaised>
        <Tooltip body={formatTimestamp(lastRun.requestedAt)}>
          <Text
            size={TextSize.BODY_SM}
            variant={OBSERVABILITY_PIPELINE_HEALTH_TO_LAST_RUN_VARIANT_MAP[health]}
            lineClamp={1}
          >
            {formatRelativeTime(lastRun.requestedAt)}
          </Text>
        </Tooltip>
      </ObservabilityPipelinesRowRaised>
    );
  };

  return (
    <ObservabilityPipelinesRowFrame data-pipelines-grid>
      <ObservabilityPipelinesColumnCell column={ObservabilityPipelinesColumn.FLOW}>
        <PipelineFlow
          sourceId={sourceId}
          sinkIds={sinkIds}
          hasEdges={(graph?.edges ?? []).length > 0}
          size={PipelineFlowSize.SMALL}
        />
      </ObservabilityPipelinesColumnCell>
      <ObservabilityPipelinesColumnCell column={ObservabilityPipelinesColumn.PIPELINE}>
        <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
          <ObservabilityPipelinesRowLink
            href={createFilamentHref(FilamentPath.PIPELINE, { id: pipeline.id })}
          >
            <PipelineName pipelineId={pipeline.id} pipeline={pipeline} />
          </ObservabilityPipelinesRowLink>
          <ObservabilityPipelinesRowRaised>
            <PipelineScheduleIndicator pipelineId={pipeline.id} pipeline={pipeline} />
          </ObservabilityPipelinesRowRaised>
        </Flex>
      </ObservabilityPipelinesColumnCell>
      <ObservabilityPipelinesColumnCell column={ObservabilityPipelinesColumn.RUNS}>
        <ObservabilityPipelinesRowRaised role="group" aria-label="Runs over time">
          {cells.map((cell) => (
            <ObservabilityPipelinesBar key={cell.bucketStartMs.toString()} cell={cell} />
          ))}
        </ObservabilityPipelinesRowRaised>
      </ObservabilityPipelinesColumnCell>
      <ObservabilityPipelinesColumnCell column={ObservabilityPipelinesColumn.LAST_RUN}>
        {renderLastRun()}
      </ObservabilityPipelinesColumnCell>
      <ObservabilityPipelinesColumnCell column={ObservabilityPipelinesColumn.RECORDS}>
        <Text size={TextSize.BODY_SM} family={FontFamily.MONO} align="right">
          {lastRun ? formatNumber(lastRun.records) : EMPTY_VALUE}
        </Text>
      </ObservabilityPipelinesColumnCell>
    </ObservabilityPipelinesRowFrame>
  );
};

export default ObservabilityPipelinesRow;
