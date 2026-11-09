import type { FC } from "react";

import { styled } from "@linaria/react";

import Flex from "@galaxy-io/dls/layout/Flex";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import { PIPELINE_RUN_STATUS_TO_HUE_MAP } from "@/components/runs/constants";

import {
  OBSERVABILITY_PIPELINES_BAR_HEIGHT,
  OBSERVABILITY_PIPELINES_BAR_WIDTH,
} from "@/pages/observability/components/pipelines/constants";
import type { ObservabilityPipelinesCell } from "@/pages/observability/components/pipelines/types";
import { createObservabilityPipelinesBarSegments } from "@/pages/observability/components/pipelines/utils";

const ObservabilityPipelinesBarFrame = styled.span`
  display: flex;
  flex-direction: column;
  width: ${OBSERVABILITY_PIPELINES_BAR_WIDTH}px;
  height: ${OBSERVABILITY_PIPELINES_BAR_HEIGHT}px;
  background-color: ${t.color.border.primary};
`;

const ObservabilityPipelinesBarSegment = styled.span<{
  $hue: RoleColor | undefined;
  $height: number;
}>`
  flex: 0 0 ${({ $height }) => $height}px;
  background-color: ${({ $hue }) => ($hue ? t.color.mark[$hue] : t.color.text.disabled)};
`;

interface ObservabilityPipelinesBarProps {
  cell: ObservabilityPipelinesCell;
}

const ObservabilityPipelinesBar: FC<ObservabilityPipelinesBarProps> = ({ cell }) => (
  <Tooltip body={cell.summary}>
    <Flex>
      <ObservabilityPipelinesBarFrame>
        {createObservabilityPipelinesBarSegments(cell).map((segment) => (
          <ObservabilityPipelinesBarSegment
            key={segment.status}
            $hue={PIPELINE_RUN_STATUS_TO_HUE_MAP[segment.status]}
            $height={segment.height}
          />
        ))}
      </ObservabilityPipelinesBarFrame>
    </Flex>
  </Tooltip>
);

export default ObservabilityPipelinesBar;
