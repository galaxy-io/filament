import type { FC, PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import SelectInput, {
  SelectInputSize,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";

import { usePipelineCanvasEdgeConfig } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasEdgeConfig";
import { usePipelineCanvasEdgeResources } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasEdgeResources";
import { usePipelineCanvasReadOnly } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  PIPELINE_CANVAS_ROUTES_CURSOR_SELECT_WIDTH,
  PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP,
  PIPELINE_CANVAS_ROUTES_READ_MODE_SELECT_WIDTH,
  PIPELINE_CANVAS_ROUTES_WRITE_MODE_SELECT_WIDTH,
} from "@/pages/pipelines/canvas/routes/constants";
import type { PipelineCanvasRoute } from "@/pages/pipelines/canvas/routes/types";
import { getEdgeModeOptions, getEdgeResourceStatus } from "@/pages/pipelines/canvas/utils";
import { getCursorSelectOptions } from "@/pages/pipelines/components/resource/utils";
import PipelineTransformFieldsIssuesChip from "@/pages/pipelines/components/transform/PipelineTransformFieldsIssuesChip";

import { getEnumSelectValue, mapOptionIdToEnum } from "@/utils/select";

const ControlsGroup = styled.div`
  position: relative;
  z-index: 1;
  flex-shrink: 0;

  display: flex;
  align-items: center;
  gap: ${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}px;

  background-color: ${t.color.background.base};
  border-radius: ${t.radius.md};
`;

const CenterSlot = styled.div`
  position: relative;
  z-index: 1;
  flex: 1;
  min-width: 0;

  display: flex;
  align-items: center;
  justify-content: center;
  gap: ${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}px;

  pointer-events: none;

  > * {
    pointer-events: auto;
  }
`;

const stopPropagation = (event: React.SyntheticEvent) => event.stopPropagation();

interface PipelineCanvasRoutesRowControlsProps extends PropsWithChildren {
  route: PipelineCanvasRoute;
}

const PipelineCanvasRoutesRowControls: FC<PipelineCanvasRoutesRowControlsProps> = ({
  route,
  children,
}) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const resources = usePipelineCanvasEdgeResources(route.edge, { enabled: route.hasReadLevers });
  const { resource, cursorOptions, defaultCursor, isManagedIncremental, isLoadingColumns } =
    resources;

  const {
    readMode,
    writeMode,
    cursor,
    readModeSelectOptions,
    writeModeSelectOptions,
    handleReadModeChange,
    handleWriteModeChange,
    handleCursorChange,
  } = usePipelineCanvasEdgeConfig(route.edge, {
    ...getEdgeModeOptions(route.verdict, route.hasReadLevers),
    resource,
    defaultCursor,
    hasReadLevers: route.hasReadLevers,
  });

  const cursorSelectOptions = getCursorSelectOptions(cursorOptions);
  const hasCursorSelect =
    route.hasReadLevers && readMode === ReadMode.INCREMENTAL && !isManagedIncremental;
  const status = getEdgeResourceStatus(resources, {
    verdict: route.verdict,
    readMode,
    writeMode,
    cursor,
  });
  const issues = status?.isBlocking ? [...route.issues, status.message] : route.issues;
  const warnings = status && !status.isBlocking ? [status.message] : [];

  return (
    <>
      <ControlsGroup
        onClick={stopPropagation}
        onMouseDown={stopPropagation}
        onKeyDown={stopPropagation}
      >
        {route.hasReadLevers && (
          <Box width={PIPELINE_CANVAS_ROUTES_READ_MODE_SELECT_WIDTH}>
            <SelectInput
              ariaLabel="Read mode"
              fillWidth
              options={readModeSelectOptions}
              value={getEnumSelectValue(ReadMode, readMode)}
              onChange={(id) => {
                if (id !== null) handleReadModeChange(mapOptionIdToEnum(ReadMode, id));
              }}
              variant={SelectInputVariant.SECONDARY}
              size={SelectInputSize.SMALL}
              placeholder="Read mode..."
              isDisabled={isReadOnly}
            />
          </Box>
        )}
        {hasCursorSelect && (
          <Box width={PIPELINE_CANVAS_ROUTES_CURSOR_SELECT_WIDTH}>
            <SelectInput
              ariaLabel="Cursor"
              fillWidth
              options={cursorSelectOptions}
              value={cursor || null}
              onChange={(id) => {
                if (id !== null) handleCursorChange(id);
              }}
              variant={SelectInputVariant.SECONDARY}
              size={SelectInputSize.SMALL}
              placeholder="Cursor..."
              isDisabled={isReadOnly || isLoadingColumns}
            />
          </Box>
        )}
      </ControlsGroup>
      <CenterSlot>
        {children}
        <PipelineTransformFieldsIssuesChip issues={issues} warnings={warnings} />
      </CenterSlot>
      <ControlsGroup
        onClick={stopPropagation}
        onMouseDown={stopPropagation}
        onKeyDown={stopPropagation}
      >
        <Box width={PIPELINE_CANVAS_ROUTES_WRITE_MODE_SELECT_WIDTH}>
          <SelectInput
            ariaLabel="Write mode"
            fillWidth
            options={writeModeSelectOptions}
            value={getEnumSelectValue(WriteMode, writeMode)}
            onChange={(id) => {
              if (id !== null) handleWriteModeChange(mapOptionIdToEnum(WriteMode, id));
            }}
            variant={SelectInputVariant.SECONDARY}
            size={SelectInputSize.SMALL}
            placeholder="Write mode..."
            isDisabled={isReadOnly}
          />
        </Box>
      </ControlsGroup>
    </>
  );
};

export default PipelineCanvasRoutesRowControls;
