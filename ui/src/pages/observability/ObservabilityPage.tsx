import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";

import ObservabilityMetricsWidget from "@/pages/observability/components/metrics/ObservabilityMetricsWidget";
import ObservabilityRunsWidget from "@/pages/observability/components/runs/ObservabilityRunsWidget";

const ObservabilityPage = () => (
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
);

export default ObservabilityPage;
