import { styled } from "@linaria/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ResourceColumn } from "@/gen/ingestion/v1/providers_pb";

import { formatColumnType, isColumnSelectable } from "@/pages/pipelines/canvas/edges/utils";

const ColumnRow = withTheme(styled.div<PropsWithTheme<{ $isSelectable: boolean }>>`
  padding: 6px 8px;
  border-radius: 4px;

  cursor: ${({ $isSelectable }) => ($isSelectable ? "pointer" : "default")};

  &:hover {
    background-color: ${({ theme, $isSelectable }) =>
      $isSelectable ? theme.color.background.secondary : "transparent"};
  }
`);

interface PipelineCanvasEdgeCursorColumnProps {
  column: ResourceColumn;
  onSelect: (field: string) => void;
}

const PipelineCanvasEdgeCursorColumn = ({
  column,
  onSelect,
}: PipelineCanvasEdgeCursorColumnProps) => {
  const isSelectable = isColumnSelectable(column);
  const reason = column.warning || (isSelectable ? "" : "Not usable as a cursor");

  return (
    <ColumnRow
      $isSelectable={isSelectable}
      onClick={isSelectable ? () => onSelect(column.name) : undefined}
    >
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        gap={FlexGap.SMALL}
        fillWidth
      >
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
          <Text
            size={TextSize.BODY_SM}
            variant={isSelectable ? TextVariant.PRIMARY : TextVariant.TERTIARY}
            isMonospace
          >
            {column.name}
          </Text>
          {column.primaryKey && (
            <Chip label="PK" size={ChipSize.SMALL} variant={ChipVariant.TERTIARY} />
          )}
          {column.cursorRecommended && (
            <Chip label="Recommended" size={ChipSize.SMALL} variant={ChipVariant.LIME} />
          )}
        </FlexWrapper>
        <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY} isMonospace>
          {formatColumnType(column)}
        </Text>
      </FlexWrapper>
      {reason && (
        <Text
          size={TextSize.CAPTION}
          variant={isSelectable ? TextVariant.WARNING : TextVariant.TERTIARY}
        >
          {reason}
        </Text>
      )}
    </ColumnRow>
  );
};

export default PipelineCanvasEdgeCursorColumn;
