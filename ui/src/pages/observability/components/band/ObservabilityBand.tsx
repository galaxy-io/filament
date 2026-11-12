import type { FC } from "react";

import { styled } from "@linaria/react";

import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { OBSERVABILITY_BAND_WRAP_MAX_WIDTH } from "@/pages/observability/components/band/constants";
import ObservabilityBandMetricTile from "@/pages/observability/components/band/ObservabilityBandMetricTile";
import ObservabilityBandRunsTile from "@/pages/observability/components/band/ObservabilityBandRunsTile";
import { OBSERVABILITY_LAYOUT_CONTAINER_NAME } from "@/pages/observability/constants";

const ObservabilityBandGrid = styled.div`
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  width: 100%;

  & > * + * {
    border-left: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  }

  @container ${OBSERVABILITY_LAYOUT_CONTAINER_NAME} (max-width: ${OBSERVABILITY_BAND_WRAP_MAX_WIDTH}px) {
    grid-template-columns: repeat(2, minmax(0, 1fr));

    & > :first-child {
      grid-column: 1 / -1;
      border-bottom: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
    }

    & > :nth-child(2) {
      border-left: none;
    }
  }
`;

const ObservabilityBand: FC = () => (
  <ObservabilityBandGrid>
    <ObservabilityBandRunsTile />
    <ObservabilityBandMetricTile
      label="Throughput"
      viewKey="throughput"
      pivotKey="throughputPivot"
      isStacked
    />
    <ObservabilityBandMetricTile
      label="Usage"
      viewKey="usage"
      pivotKey="usagePivot"
      isStacked={false}
    />
  </ObservabilityBandGrid>
);

export default ObservabilityBand;
