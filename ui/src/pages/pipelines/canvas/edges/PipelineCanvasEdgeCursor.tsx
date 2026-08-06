import { useState } from "react";

import { styled } from "@linaria/react";
import { CaretDownIcon, WarningCircleIcon } from "@phosphor-icons/react";

import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import NumberInput from "@galaxy-io/dls/inputs/NumberInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ResourceCursorConfig } from "@/gen/ingestion/v1/pipelines_pb";
import type { ResourceColumn } from "@/gen/ingestion/v1/providers_pb";

import ErrorLayout from "@/layouts/ErrorLayout";

import {
  PIPELINE_CANVAS_DEFAULT_LOOKBACK_SECONDS,
  PIPELINE_CANVAS_EDGE_AUTO_CURSOR_LABEL,
  PIPELINE_CANVAS_EDGE_COLUMN_LIST_MAX_HEIGHT,
  PIPELINE_CANVAS_EDGE_SELECT_WIDTH,
} from "@/pages/pipelines/canvas/edges/constants";
import PipelineCanvasEdgeCursorColumn from "@/pages/pipelines/canvas/edges/PipelineCanvasEdgeCursorColumn";
import { sortCursorColumns } from "@/pages/pipelines/canvas/edges/utils";

import { isSearchMatch } from "@/utils/search";

const CursorTrigger = withTheme(styled.button<PropsWithTheme>`
  width: ${PIPELINE_CANVAS_EDGE_SELECT_WIDTH}px;
  padding: 4px 8px;

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 4px;

  background-color: ${({ theme }) => theme.color.background.base};
  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 6px;
  cursor: pointer;

  &:disabled {
    cursor: default;
  }
`);

const PanelSection = styled.div`
  padding: 8px;
`;

const ColumnList = styled.div`
  max-height: ${PIPELINE_CANVAS_EDGE_COLUMN_LIST_MAX_HEIGHT}px;
  overflow-y: auto;
  padding: 4px;
`;

const AutoRow = withTheme(styled.div<PropsWithTheme>`
  padding: 6px 8px;
  border-radius: 4px;
  cursor: pointer;

  &:hover {
    background-color: ${({ theme }) => theme.color.background.secondary};
  }
`);

const SHIMMER_COUNT = 5;

interface PipelineCanvasEdgeCursorState {
  isOpen: boolean;
  search: string;
}

const DEFAULT_STATE: PipelineCanvasEdgeCursorState = {
  isOpen: false,
  search: "",
};

interface PipelineCanvasEdgeCursorProps {
  columns: ResourceColumn[];
  cursor: ResourceCursorConfig | null;
  error: Error | null;
  isLoading: boolean;
  isDisabled: boolean;
  onChange: (field: string, lookbackSeconds: number) => void;
}

const PipelineCanvasEdgeCursor = ({
  columns,
  cursor,
  error,
  isLoading,
  isDisabled,
  onChange,
}: PipelineCanvasEdgeCursorProps) => {
  const [state, setState] = useState<PipelineCanvasEdgeCursorState>(DEFAULT_STATE);

  const selectedColumn = columns.find((column) => column.name === cursor?.field) ?? null;
  const matchedColumns = sortCursorColumns(columns).filter((column) =>
    isSearchMatch(state.search, column.name),
  );

  const handleToggle = () => {
    setState((prev) => ({ ...prev, isOpen: !prev.isOpen }));
  };

  const handleClose = () => {
    setState(DEFAULT_STATE);
  };

  const handleSelect = (field: string) => {
    setState(DEFAULT_STATE);
    onChange(field, Number(cursor?.lookbackSeconds ?? 0));
  };

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  const handleLookbackChange = (lookbackSeconds: number | undefined) => {
    if (!cursor) return;
    onChange(cursor.field, lookbackSeconds ?? 0);
  };

  const renderBody = () => {
    if (isLoading && columns.length === 0) {
      return (
        <ColumnList>
          {Array.from({ length: SHIMMER_COUNT }).map((_, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows with no identity
            <TextShimmer key={index} height={16} width="100%" />
          ))}
        </ColumnList>
      );
    }

    if (error) {
      return (
        <FlexWrapper padding={"20px 16px"} fillWidth>
          <ErrorLayout
            icon={<Icon component={WarningCircleIcon} size={20} variant={IconVariant.ERROR} />}
            message="Failed to load columns"
            error={error}
          />
        </FlexWrapper>
      );
    }

    return (
      <ColumnList>
        <AutoRow onClick={() => handleSelect("")}>
          <Text size={TextSize.BODY_SM}>{PIPELINE_CANVAS_EDGE_AUTO_CURSOR_LABEL}</Text>
          <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY}>
            Let the connector detect a timestamp column at run time
          </Text>
        </AutoRow>
        {matchedColumns.map((column) => (
          <PipelineCanvasEdgeCursorColumn
            key={column.name}
            column={column}
            onSelect={handleSelect}
          />
        ))}
      </ColumnList>
    );
  };

  return (
    <Dropdown
      position={DropdownPosition.BOTTOM_START}
      isOpen={state.isOpen}
      onClose={handleClose}
      noPadding
      header={
        <PanelSection>
          <TextInput
            placeholder="Search columns"
            value={state.search}
            onChange={handleSearchChange}
            size={InputSize.SMALL}
            fillWidth
          />
        </PanelSection>
      }
      body={renderBody()}
      footer={
        selectedColumn ? (
          <PanelSection>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL} fillWidth>
              {selectedColumn.warning && (
                <Text size={TextSize.CAPTION} variant={TextVariant.WARNING}>
                  {selectedColumn.warning}
                </Text>
              )}
              {selectedColumn.supportsLookback && (
                <>
                  <HorizontalDivider />
                  <NumberInput
                    label="Lookback (seconds)"
                    value={cursor?.lookbackSeconds ? Number(cursor.lookbackSeconds) : undefined}
                    placeholder={String(PIPELINE_CANVAS_DEFAULT_LOOKBACK_SECONDS)}
                    onChange={handleLookbackChange}
                    size={InputSize.SMALL}
                    min={0}
                    step={1}
                    isDisabled={isDisabled}
                    fillWidth
                  />
                </>
              )}
            </FlexWrapper>
          </PanelSection>
        ) : undefined
      }
    >
      <CursorTrigger className="nodrag" onClick={handleToggle} disabled={isDisabled} type="button">
        <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY}>
          Cursor
        </Text>
        <FlexWrapper gap={FlexGap.XXSMALL}>
          <Text size={TextSize.CAPTION} isMonospace isEllipsis>
            {cursor?.field || PIPELINE_CANVAS_EDGE_AUTO_CURSOR_LABEL}
          </Text>
          <Icon component={CaretDownIcon} size={12} variant={IconVariant.TERTIARY} />
        </FlexWrapper>
      </CursorTrigger>
    </Dropdown>
  );
};

export default PipelineCanvasEdgeCursor;
