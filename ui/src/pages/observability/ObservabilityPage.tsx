import { useNavigate, useSearch } from "@tanstack/react-router";

import { ChartPalette } from "@galaxy-io/dls/charts/types";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  FlexWrap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

import { Metric } from "@/gen/metrics/v1/metrics_pb";

import ObservabilityMetricsWidget from "@/pages/observability/components/metrics/ObservabilityMetricsWidget";
import ObservabilityToolbar from "@/pages/observability/components/ObservabilityToolbar";
import ObservabilityRunsWidget from "@/pages/observability/components/runs/ObservabilityRunsWidget";
import ObservabilityTimeseriesWidget from "@/pages/observability/components/timeseries/ObservabilityTimeseriesWidget";

import type { ObservabilityPivotDimension } from "@/routes/_main/observability";

import { formatBytes, formatCount } from "@/utils/format";

const OBSERVABILITY_COUNT_VALUE_FORMATTER = (value: number) =>
  formatCount(BigInt(Math.round(value)));

const OBSERVABILITY_BYTES_VALUE_FORMATTER = (value: number) =>
  formatBytes(BigInt(Math.round(value)));

const ObservabilityPage = () => {
  const navigate = useNavigate();
  const { records, volume } = useSearch({ from: "/_main/observability" });

  const handleRecordsPivotChange = (pivot: ObservabilityPivotDimension | null) => {
    void navigate({ to: ".", search: (prev) => ({ ...prev, records: pivot ?? undefined }) });
  };

  const handleVolumePivotChange = (pivot: ObservabilityPivotDimension | null) => {
    void navigate({ to: ".", search: (prev) => ({ ...prev, volume: pivot ?? undefined }) });
  };

  return (
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
              title="Records"
              seriesLabel="Records"
              metric={Metric.RUN_RECORDS}
              color={ChartPalette.PURPLE}
              pivot={records}
              onPivotChange={handleRecordsPivotChange}
              valueFormatter={OBSERVABILITY_COUNT_VALUE_FORMATTER}
            />
          </FlexItem>
          <FlexItem grow={1} basis="400px" minWidth={0}>
            <ObservabilityTimeseriesWidget
              title="Volume"
              seriesLabel="Bytes"
              metric={Metric.RUN_BYTES}
              color={ChartPalette.TEAL}
              pivot={volume}
              onPivotChange={handleVolumePivotChange}
              valueFormatter={OBSERVABILITY_BYTES_VALUE_FORMATTER}
            />
          </FlexItem>
        </FlexWrapper>
        <ObservabilityRunsWidget />
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default ObservabilityPage;
