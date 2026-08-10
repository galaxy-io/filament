import { DashboardProvider } from "@galaxy-io/dls/charts/DashboardProvider";
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
  OBSERVABILITY_METRIC_VIEW_TO_CONFIG_MAP,
  OBSERVABILITY_USAGE_VIEW_TO_CONFIG_MAP,
} from "@/pages/observability/components/timeseries/constants";
import ObservabilityTimeseriesWidget from "@/pages/observability/components/timeseries/ObservabilityTimeseriesWidget";
import { useObservabilitySetup } from "@/pages/observability/hooks/useObservabilitySetup";
import { ObservabilityMetricView, ObservabilityUsageView } from "@/pages/observability/types";

const ObservabilityPage = () => {
  const { isComplete } = useObservabilitySetup();

  if (!isComplete) {
    return <ObservabilitySetupChecklist />;
  }

  return (
    <DashboardProvider sharedTooltip>
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
            <FlexItem grow={1} basis="400px" minWidth={0}>
              <ObservabilityTimeseriesWidget
                views={OBSERVABILITY_METRIC_VIEW_TO_CONFIG_MAP}
                defaultView={ObservabilityMetricView.RECORDS}
                viewSearchKey="metric"
                pivotSearchKey="pivot"
              />
            </FlexItem>
            <FlexItem grow={1} basis="400px" minWidth={0}>
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
    </DashboardProvider>
  );
};

export default ObservabilityPage;
