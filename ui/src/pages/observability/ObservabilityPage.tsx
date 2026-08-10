import { useNavigate, useSearch } from "@tanstack/react-router";

import { DashboardProvider } from "@galaxy-io/dls/charts/DashboardProvider";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

import ObservabilityMetricsWidget from "@/pages/observability/components/metrics/ObservabilityMetricsWidget";
import ObservabilityToolbar from "@/pages/observability/components/ObservabilityToolbar";
import ObservabilityRunsWidget from "@/pages/observability/components/runs/ObservabilityRunsWidget";
import ObservabilitySetupChecklist from "@/pages/observability/components/setup/ObservabilitySetupChecklist";
import ObservabilityTimeseriesWidget from "@/pages/observability/components/timeseries/ObservabilityTimeseriesWidget";
import { useObservabilitySetup } from "@/pages/observability/hooks/useObservabilitySetup";
import { ObservabilityMetricView } from "@/pages/observability/types";

import type { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

const ObservabilityPage = () => {
  const navigate = useNavigate();
  const { metric = ObservabilityMetricView.RECORDS, pivot } = useSearch({
    from: "/_main/observability",
  });
  const { isComplete } = useObservabilitySetup();

  const handleMetricViewChange = (view: ObservabilityMetricView) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, metric: view }),
    });
  };

  const handlePivotChange = (nextPivot: MetricDimension | undefined) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, pivot: nextPivot }),
    });
  };

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
          <ObservabilityTimeseriesWidget
            view={metric}
            onViewChange={handleMetricViewChange}
            pivot={pivot}
            onPivotChange={handlePivotChange}
          />
          <ObservabilityRunsWidget />
        </FlexWrapper>
      </FlexWrapper>
    </DashboardProvider>
  );
};

export default ObservabilityPage;
