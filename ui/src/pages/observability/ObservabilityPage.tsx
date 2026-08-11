import { ChartGroupProvider } from "@galaxy-io/dls/charts/ChartGroupProvider";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  FlexWrap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

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
    <ChartGroupProvider sharedTooltip>
      <FlexWrapper direction={FlexDirection.COLUMN} fillWidth fillHeight>
        <ObservabilityToolbar />
        <FlexItem grow={0} shrink={0} fillWidth>
          <HorizontalDivider />
        </FlexItem>
        <FlexWrapper
          direction={FlexDirection.COLUMN}
          padding={"12px"}
          gap={FlexGap.MEDIUM}
          fillWidth
          fillHeight
          overflow="auto"
        >
          <ObservabilityMetricsWidget />
          <FlexWrapper
            gap={FlexGap.MEDIUM}
            alignItems={AlignItems.STRETCH}
            wrap={FlexWrap.WRAP}
            fillWidth
          >
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
                viewSearchKey="usage"
                pivotSearchKey="usagePivot"
              />
            </FlexItem>
          </FlexWrapper>
          <ObservabilityRunsWidget />
        </FlexWrapper>
      </FlexWrapper>
    </ChartGroupProvider>
  );
};

export default ObservabilityPage;
