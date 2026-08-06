import { styled } from "@linaria/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { CandidateValue } from "@/gen/ingestion/v1/capabilities_pb";

const CandidateRow = withTheme(styled.div<PropsWithTheme>`
  padding: 6px 8px;
  border-radius: 4px;
  cursor: pointer;

  &:hover {
    background-color: ${({ theme }) => theme.color.background.secondary};
  }
`);

interface PipelineCanvasEdgeCursorColumnProps {
  candidate: CandidateValue;
  onSelect: (field: string) => void;
}

const PipelineCanvasEdgeCursorColumn = ({
  candidate,
  onSelect,
}: PipelineCanvasEdgeCursorColumnProps) => (
  <CandidateRow onClick={() => onSelect(candidate.value)}>
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
      <Text size={TextSize.BODY_SM} isMonospace>
        {candidate.value}
      </Text>
      {candidate.recommended && (
        <Chip label="Recommended" size={ChipSize.SMALL} variant={ChipVariant.LIME} />
      )}
    </FlexWrapper>
    {candidate.warning && (
      <Text size={TextSize.CAPTION} variant={TextVariant.WARNING}>
        {candidate.warning}
      </Text>
    )}
  </CandidateRow>
);

export default PipelineCanvasEdgeCursorColumn;
