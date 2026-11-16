import type { FC } from "react";

import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import { SquareSize } from "@galaxy-io/dls/shapes/Square";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { PIPELINE_RUN_STATUS_TO_HUE_MAP } from "@/components/runs/constants";
import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";

import ObservabilityBandList from "@/pages/observability/components/band/ObservabilityBandList";
import ObservabilityBandRunsChart from "@/pages/observability/components/band/ObservabilityBandRunsChart";
import ObservabilityBandTile from "@/pages/observability/components/band/ObservabilityBandTile";
import { formatObservabilityBandCount } from "@/pages/observability/components/band/utils";
import ObservabilityRunsViewToggle from "@/pages/observability/components/runs/ObservabilityRunsViewToggle";
import { OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP } from "@/pages/observability/constants";
import { useObservabilityBandUpcoming } from "@/pages/observability/hooks/useObservabilityBandUpcoming";

import { useObservabilitySearch } from "@/module/hooks";

const ObservabilityBandRunsUpcomingTile: FC = () => {
  const { timeframe } = useObservabilitySearch();
  const { count, groups, nextUp, isLoading, isFetching } = useObservabilityBandUpcoming();

  return (
    <ObservabilityBandTile
      label="Runs"
      info={`Runs scheduled to start in the next ${OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP[timeframe]}. Each schedule plans only its next run ahead.`}
      actions={<ObservabilityRunsViewToggle />}
      isLoading={isLoading}
      value={
        <Flex alignItems={AlignItems.CENTER} gap={12} minWidth={0}>
          <Text size={TextSize.HEADING_SM} family={FontFamily.MONO}>
            {formatObservabilityBandCount(count)}
          </Text>
          <Flex alignItems={AlignItems.CENTER} gap={4}>
            <PipelineRunStatusSwatch status={RunStatus.SCHEDULED} size={SquareSize.SMALL} />
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY} lineClamp={1}>
              scheduled
            </Text>
          </Flex>
        </Flex>
      }
      chart={
        <ObservabilityBandRunsChart
          groups={groups}
          isLoading={isFetching}
          ariaLabel="Scheduled runs"
        />
      }
      top={
        <ObservabilityBandList
          title="Next up"
          emptyText="No upcoming runs"
          items={nextUp}
          isLoading={isLoading}
          colors={[PIPELINE_RUN_STATUS_TO_HUE_MAP[RunStatus.SCHEDULED]]}
        />
      }
    />
  );
};

export default ObservabilityBandRunsUpcomingTile;
