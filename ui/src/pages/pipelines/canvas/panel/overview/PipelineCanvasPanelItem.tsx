import { styled } from "@linaria/react";

import { FOCUS_RING, HAIRLINE_WIDTH, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const PipelineCanvasPanelItem = styled.button`
  ${INTERACTIVE_RESET}
  ${FOCUS_RING}
  width: 100%;
  padding: ${t.space[12]};

  display: flex;
  align-items: center;
  gap: ${t.space[8]};

  text-align: left;

  transition: background-color ${t.duration.fast};

  &:not(:last-child) {
    border-bottom: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  }

  &:hover {
    background-color: ${t.color.background.hovered};
  }
`;

export default PipelineCanvasPanelItem;
