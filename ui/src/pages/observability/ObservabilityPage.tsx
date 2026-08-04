import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

import ObservabilityMetricsWidget from "@/pages/observability/components/metrics/ObservabilityMetricsWidget";
import ObservabilityToolbar from "@/pages/observability/components/ObservabilityToolbar";
import ObservabilityRunsWidget from "@/pages/observability/components/runs/ObservabilityRunsWidget";
import ObservabilityTimeframeProvider from "@/pages/observability/providers/ObservabilityTimeframeProvider";

const ObservabilityPage = () => (
  <ObservabilityTimeframeProvider>
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
        <ObservabilityRunsWidget />
      </FlexWrapper>
    </FlexWrapper>
  </ObservabilityTimeframeProvider>
);

export default ObservabilityPage;
