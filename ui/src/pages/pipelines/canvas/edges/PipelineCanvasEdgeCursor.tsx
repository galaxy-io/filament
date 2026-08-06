import { useState } from "react";

import { styled } from "@linaria/react";
import { CaretDownIcon } from "@phosphor-icons/react";

import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import NumberInput from "@galaxy-io/dls/inputs/NumberInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { CandidateStatus, type Requirement } from "@/gen/ingestion/v1/capabilities_pb";
import type { ResourceCursorConfig } from "@/gen/ingestion/v1/pipelines_pb";

import {
  PIPELINE_CANVAS_DEFAULT_LOOKBACK_SECONDS,
  PIPELINE_CANVAS_EDGE_AUTO_CURSOR_LABEL,
  PIPELINE_CANVAS_EDGE_COLUMN_LIST_MAX_HEIGHT,
  PIPELINE_CANVAS_EDGE_SELECT_WIDTH,
} from "@/pages/pipelines/canvas/edges/constants";
import PipelineCanvasEdgeCursorColumn from "@/pages/pipelines/canvas/edges/PipelineCanvasEdgeCursorColumn";
import { sortCursorCandidates } from "@/pages/pipelines/canvas/edges/utils";

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

const CandidateList = styled.div`
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

interface PipelineCanvasEdgeCursorState {
  isOpen: boolean;
  search: string;
}

const DEFAULT_STATE: PipelineCanvasEdgeCursorState = {
  isOpen: false,
  search: "",
};

interface PipelineCanvasEdgeCursorProps {
  requirement: Requirement | undefined;
  cursor: ResourceCursorConfig | null;
  isDisabled: boolean;
  onChange: (field: string, lookbackSeconds: number) => void;
}

const PipelineCanvasEdgeCursor = ({
  requirement,
  cursor,
  isDisabled,
  onChange,
}: PipelineCanvasEdgeCursorProps) => {
  const [state, setState] = useState<PipelineCanvasEdgeCursorState>(DEFAULT_STATE);

  const candidates = requirement?.candidates ?? [];
  const hasCandidates = candidates.length > 0;
  const isEnumerated = requirement?.candidateStatus === CandidateStatus.ENUMERATED;
  const selectedCandidate = candidates.find((candidate) => candidate.value === cursor?.field);
  const matchedCandidates = sortCursorCandidates(candidates).filter((candidate) =>
    isSearchMatch(state.search, candidate.value),
  );

  const handleToggle = () => {
    setState((prev) => ({ ...prev, isOpen: !prev.isOpen }));
  };

  const handleClose = () => {
    setState(DEFAULT_STATE);
  };

  const handleSearchChange = (search: string) => {
    setState((prev) => ({ ...prev, search }));
  };

  const handleSelect = (field: string) => {
    setState(DEFAULT_STATE);
    onChange(field, Number(cursor?.lookbackSeconds ?? 0));
  };

  const handleLookbackChange = (lookbackSeconds: number | undefined) => {
    if (!cursor) return;
    onChange(cursor.field, lookbackSeconds ?? 0);
  };

  const renderBody = () => {
    if (isEnumerated && !hasCandidates) {
      return (
        <PanelSection>
          <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
            {requirement?.message}
          </Text>
        </PanelSection>
      );
    }

    return (
      <>
        {!isEnumerated && requirement?.message && (
          <PanelSection>
            <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY}>
              {requirement.message}
            </Text>
          </PanelSection>
        )}
        <CandidateList>
          <AutoRow onClick={() => handleSelect("")}>
            <Text size={TextSize.BODY_SM}>{PIPELINE_CANVAS_EDGE_AUTO_CURSOR_LABEL}</Text>
            <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY}>
              Let the connector detect a cursor column at run time
            </Text>
          </AutoRow>
          {matchedCandidates.map((candidate) => (
            <PipelineCanvasEdgeCursorColumn
              key={candidate.value}
              candidate={candidate}
              onSelect={handleSelect}
            />
          ))}
        </CandidateList>
      </>
    );
  };

  return (
    <Dropdown
      position={DropdownPosition.BOTTOM_START}
      isOpen={state.isOpen}
      onClose={handleClose}
      noPadding
      header={
        hasCandidates ? (
          <PanelSection>
            <TextInput
              placeholder="Search columns"
              value={state.search}
              onChange={handleSearchChange}
              size={InputSize.SMALL}
              fillWidth
            />
          </PanelSection>
        ) : undefined
      }
      body={renderBody()}
      footer={
        selectedCandidate ? (
          <PanelSection>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL} fillWidth>
              {selectedCandidate.warning && (
                <Text size={TextSize.CAPTION} variant={TextVariant.WARNING}>
                  {selectedCandidate.warning}
                </Text>
              )}
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
