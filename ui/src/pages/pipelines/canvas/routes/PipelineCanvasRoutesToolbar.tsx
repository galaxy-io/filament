import { type FC, useMemo, useRef } from "react";

import { PlusIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { useElementWidth } from "@galaxy-io/dls/hooks/useElementWidth";
import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput, {
  MultiSelectInputSize,
  MultiSelectInputVariant,
} from "@galaxy-io/dls/inputs/MultiSelectInput";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text from "@galaxy-io/dls/text/Text";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";

import { PIPELINE_CANVAS_VIEW_SWITCHER_INSET } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasViewSwitcher from "@/pages/pipelines/canvas/PipelineCanvasViewSwitcher";
import PipelineCanvasPanelOpenButton from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelOpenButton";
import {
  PIPELINE_CANVAS_ROUTES_SEARCH_MIN_WIDTH,
  PIPELINE_CANVAS_ROUTES_SEARCH_WIDTH,
  PIPELINE_CANVAS_ROUTES_SINK_FILTER_MIN_WIDTH,
  PIPELINE_CANVAS_ROUTES_SINK_FILTER_SHRINK,
  PIPELINE_CANVAS_ROUTES_SINK_FILTER_WIDTH,
  PIPELINE_CANVAS_ROUTES_TOOLBAR_COMPACT_WIDTH,
  PIPELINE_CANVAS_ROUTES_TOOLBAR_ICON_ONLY_WIDTH,
  PIPELINE_CANVAS_ROUTES_TOOLBAR_NO_SINK_FILTER_WIDTH,
} from "@/pages/pipelines/canvas/routes/constants";
import { usePipelineCanvasRoutesSinks } from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutesSinks";
import type { CanvasNode } from "@/pages/pipelines/canvas/types";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/constants";

interface PipelineCanvasRoutesToolbarProps {
  onSearch: (search: string) => void;
  canAddRoute: boolean;
  onAddRoute: () => void;
}

const PipelineCanvasRoutesToolbar: FC<PipelineCanvasRoutesToolbarProps> = ({
  onSearch,
  canAddRoute,
  onAddRoute,
}) => {
  const { sinkIds, setSinkIds } = usePipelineCanvasSelection();
  const sinks = usePipelineCanvasRoutesSinks();
  const toolbarRef = useRef<HTMLElement>(null);
  const toolbarWidth = useElementWidth(toolbarRef);
  const isCompact =
    toolbarWidth !== undefined && toolbarWidth < PIPELINE_CANVAS_ROUTES_TOOLBAR_COMPACT_WIDTH;
  const isIconOnly =
    toolbarWidth !== undefined && toolbarWidth < PIPELINE_CANVAS_ROUTES_TOOLBAR_ICON_ONLY_WIDTH;
  const hasSinkFilter =
    toolbarWidth === undefined ||
    toolbarWidth >= PIPELINE_CANVAS_ROUTES_TOOLBAR_NO_SINK_FILTER_WIDTH;

  const sinkOptions = useMemo<SelectOption[]>(
    () =>
      sinks.map((sink) => ({
        id: sink.nodeId,
        label: sink.label,
        leading: (
          <ConnectorTile
            connector={sink.connection?.connector ?? ""}
            kind={ConnectorKind.SINK}
            size={ConnectorTileSize.SMALL}
            isDeleted={!!sink.connection?.deletedAt}
          />
        ),
      })),
    [sinks],
  );
  const selectedSinkIds = sinkIds.length ? sinkIds : sinkOptions.map((option) => option.id);

  const handleSinksChange = (ids: CanvasNode["id"][]) => {
    setSinkIds(ids.length === sinkOptions.length ? [] : ids);
  };

  return (
    <Flex
      ref={toolbarRef}
      alignItems={AlignItems.CENTER}
      gap={12}
      padding={PIPELINE_CANVAS_VIEW_SWITCHER_INSET}
      shrink={0}
      minWidth={0}
      overflow="hidden"
      fillWidth
    >
      <FlexItem shrink={0}>
        <PipelineCanvasViewSwitcher isIconOnly={isIconOnly} />
      </FlexItem>
      <FlexItem
        shrink={1}
        basis={PIPELINE_CANVAS_ROUTES_SEARCH_WIDTH}
        minWidth={PIPELINE_CANVAS_ROUTES_SEARCH_MIN_WIDTH}
      >
        <SearchInput
          ariaLabel="Search resources"
          variant={InputVariant.SECONDARY}
          debounceMs={LIST_SEARCH_DEBOUNCE_MS}
          onSearch={onSearch}
          placeholder="Search resources..."
          fillWidth
        />
      </FlexItem>
      {hasSinkFilter && (
        <FlexItem
          shrink={PIPELINE_CANVAS_ROUTES_SINK_FILTER_SHRINK}
          basis={PIPELINE_CANVAS_ROUTES_SINK_FILTER_WIDTH}
          minWidth={PIPELINE_CANVAS_ROUTES_SINK_FILTER_MIN_WIDTH}
        >
          <MultiSelectInput
            ariaLabel="Sinks"
            fillWidth
            options={sinkOptions}
            selectAllLabel="All sinks"
            value={selectedSinkIds}
            onChange={handleSinksChange}
            placeholder="Sinks..."
            variant={MultiSelectInputVariant.SECONDARY}
            size={MultiSelectInputSize.MEDIUM}
            renderValue={(options) => <Text>{pluralize("sink", options.length, true)}</Text>}
          />
        </FlexItem>
      )}
      <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.END} gap={12} grow={1}>
        {canAddRoute && (
          <Button
            icon={PlusIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.MEDIUM}
            onClick={onAddRoute}
            {...(isCompact
              ? { ariaLabel: "Add route", tooltip: "Add route" }
              : { label: "Add route" })}
          />
        )}
        <PipelineCanvasPanelOpenButton />
      </Flex>
    </Flex>
  );
};

export default PipelineCanvasRoutesToolbar;
