import type { FC, PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import SelectInput, {
  SelectInputSize,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { ReadMode, type WriteMode } from "@/gen/ingestion/v1/common_pb";

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
import { getEdgeModeOptions, getEdgeResourceStatuses } from "@/pages/pipelines/canvas/utils";
import { getCursorSelectOptions } from "@/pages/pipelines/components/resource/utils";
import PipelineTransformFieldsIssuesChip from "@/pages/pipelines/components/transform/PipelineTransformFieldsIssuesChip";

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
  const {
    coveredResources,
    cursorOptionsByResource,
    defaultCursorByResource,
    managedIncrementalResources,
    isLoadingColumns,
  } = resources;

  const {
    readMode,
    writeMode,
    cursorsByResource,
    readModeSelectOptions,
    writeModeSelectOptions,
    handleReadModeChange,
    handleWriteModeChange,
    handleCursorChange,
  } = usePipelineCanvasEdgeConfig(route.edge, {
    ...getEdgeModeOptions(route.verdict, route.hasReadLevers),
    coveredResources,
    defaultCursorByResource,
  });

  const cursorSelectOptions = getCursorSelectOptions(cursorOptionsByResource[route.resource] ?? []);
  const cursorValue = cursorsByResource.get(route.resource) ?? "";
  const hasCursorSelect =
    route.hasReadLevers &&
    route.isNamedResource &&
    readMode === ReadMode.INCREMENTAL &&
    !managedIncrementalResources.has(route.resource);
  const statuses = getEdgeResourceStatuses(resources, {
    verdict: route.verdict,
    readMode,
    writeMode,
    cursorsByResource,
  });
  const issues = [
    ...route.issues,
    ...statuses.filter((status) => status.isBlocking).map((status) => status.message),
  ];
  const warnings = statuses.filter((status) => !status.isBlocking).map((status) => status.message);

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
              value={String(readMode)}
              onChange={(id) => {
                if (id !== null) handleReadModeChange(Number(id) as ReadMode);
              }}
              variant={SelectInputVariant.TERTIARY}
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
              value={cursorValue || null}
              onChange={(id) => {
                if (id !== null) handleCursorChange(route.resource, id);
              }}
              variant={SelectInputVariant.TERTIARY}
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
            value={String(writeMode)}
            onChange={(id) => {
              if (id !== null) handleWriteModeChange(Number(id) as WriteMode);
            }}
            variant={SelectInputVariant.TERTIARY}
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
