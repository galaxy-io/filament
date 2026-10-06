import ChartGroupProvider from "@galaxy-io/dls/charts/ChartGroupProvider";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, FlexWrap } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";

import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import ObservabilityMetricsWidget from "@/pages/observability/components/metrics/ObservabilityMetricsWidget";
import ObservabilityToolbar from "@/pages/observability/components/ObservabilityToolbar";
import ObservabilityRunsWidget from "@/pages/observability/components/runs/ObservabilityRunsWidget";
import ObservabilitySetupChecklist from "@/pages/observability/components/setup/ObservabilitySetupChecklist";
import {
  OBSERVABILITY_THROUGHPUT_VIEW_TO_CONFIG_MAP,
  OBSERVABILITY_USAGE_VIEW_TO_CONFIG_MAP,
} from "@/pages/observability/components/timeseries/constants";
import ObservabilityTimeseriesWidget from "@/pages/observability/components/timeseries/ObservabilityTimeseriesWidget";
import { useObservabilitySetup } from "@/pages/observability/hooks/useObservabilitySetup";
import { ObservabilityThroughputView, ObservabilityUsageView } from "@/pages/observability/types";

const OBSERVABILITY_TIMESERIES_WIDGET_BASIS = "400px";

const ObservabilityPage = () => {
  const { isComplete } = useObservabilitySetup();

  if (!isComplete) {
    return <ObservabilitySetupChecklist />;
  }

  return (
    <ChartGroupProvider shouldShareTooltip>
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} fillWidth height="100%">
        <ObservabilityToolbar />
        <FlexItem grow={0} shrink={0} fillWidth>
          <Divider />
        </FlexItem>
        <Flex
          alignItems={AlignItems.START}
          direction={FlexDirection.COLUMN}
          padding={12}
          gap={12}
          fillWidth
          height="100%"
          overflow="auto"
        >
          <ObservabilityMetricsWidget />
          <Flex gap={12} alignItems={AlignItems.STRETCH} wrap={FlexWrap.WRAP} fillWidth>
            <FlexItem grow={1} basis={OBSERVABILITY_TIMESERIES_WIDGET_BASIS} minWidth={0}>
              <ObservabilityTimeseriesWidget
                views={OBSERVABILITY_THROUGHPUT_VIEW_TO_CONFIG_MAP}
                defaultView={ObservabilityThroughputView.RECORDS}
                viewSearchKey="throughput"
                pivotSearchKey="throughputPivot"
              />
            </FlexItem>
            <FlexItem grow={1} basis={OBSERVABILITY_TIMESERIES_WIDGET_BASIS} minWidth={0}>
              <ObservabilityTimeseriesWidget
                views={OBSERVABILITY_USAGE_VIEW_TO_CONFIG_MAP}
                defaultView={ObservabilityUsageView.CPU}
                defaultPivot={MetricDimension.PIPELINE_ID}
                viewSearchKey="usage"
                pivotSearchKey="usagePivot"
              />
            </FlexItem>
          </Flex>
          <ObservabilityRunsWidget />
        </Flex>
      </Flex>
    </ChartGroupProvider>
  );
};

export default ObservabilityPage;
