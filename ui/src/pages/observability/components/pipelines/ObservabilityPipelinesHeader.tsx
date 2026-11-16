import type { FC } from "react";

import { styled } from "@linaria/react";

import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import {
  OBSERVABILITY_PIPELINES_COLUMN_TO_ALIGN_MAP,
  OBSERVABILITY_PIPELINES_COLUMNS,
  OBSERVABILITY_PIPELINES_HEADER_HEIGHT,
} from "@/pages/observability/components/pipelines/constants";
import ObservabilityPipelinesColumnCell from "@/pages/observability/components/pipelines/ObservabilityPipelinesColumnCell";
import { ObservabilityPipelinesColumn } from "@/pages/observability/components/pipelines/types";
import { getObservabilityPipelinesColumnLabel } from "@/pages/observability/components/pipelines/utils";

import { useObservabilitySearch } from "@/module/hooks";

const ObservabilityPipelinesHeaderFrame = styled.div`
  display: grid;
  grid-template-columns: ${OBSERVABILITY_PIPELINES_COLUMNS};
  column-gap: ${t.space[16]};
  align-items: center;
  flex: none;
  height: ${OBSERVABILITY_PIPELINES_HEADER_HEIGHT}px;
  padding: 0 ${t.space[12]};
  border-bottom: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  background-color: ${t.color.background.primary};
`;

const ObservabilityPipelinesHeader: FC = () => {
  const { timeframe } = useObservabilitySearch();

  return (
    <ObservabilityPipelinesHeaderFrame data-pipelines-grid>
      {Object.values(ObservabilityPipelinesColumn).map((column) => (
        <ObservabilityPipelinesColumnCell key={column} column={column}>
          <Text
            size={TextSize.BODY_SM}
            weight={TextWeight.MEDIUM}
            variant={TextVariant.TERTIARY}
            align={OBSERVABILITY_PIPELINES_COLUMN_TO_ALIGN_MAP[column]}
            lineClamp={1}
          >
            {getObservabilityPipelinesColumnLabel(column, timeframe)}
          </Text>
        </ObservabilityPipelinesColumnCell>
      ))}
    </ObservabilityPipelinesHeaderFrame>
  );
};

export default ObservabilityPipelinesHeader;
