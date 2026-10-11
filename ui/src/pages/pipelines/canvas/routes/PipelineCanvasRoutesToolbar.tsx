import { type FC, useMemo } from "react";

import { PlusIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput, {
  MultiSelectInputSize,
  MultiSelectInputVariant,
} from "@galaxy-io/dls/inputs/MultiSelectInput";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text from "@galaxy-io/dls/text/Text";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";

import { PIPELINE_CANVAS_VIEW_SWITCHER_INSET } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasViewSwitcher from "@/pages/pipelines/canvas/PipelineCanvasViewSwitcher";
import {
  PIPELINE_CANVAS_ROUTES_SEARCH_WIDTH,
  PIPELINE_CANVAS_ROUTES_SINK_FILTER_WIDTH,
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
      alignItems={AlignItems.CENTER}
      gap={12}
      padding={PIPELINE_CANVAS_VIEW_SWITCHER_INSET}
      shrink={0}
      fillWidth
    >
      <PipelineCanvasViewSwitcher />
      <Box width={PIPELINE_CANVAS_ROUTES_SEARCH_WIDTH}>
        <SearchInput
          ariaLabel="Search resources"
          variant={InputVariant.SECONDARY}
          debounceMs={LIST_SEARCH_DEBOUNCE_MS}
          onSearch={onSearch}
          placeholder="Search resources..."
          fillWidth
        />
      </Box>
      <Box width={PIPELINE_CANVAS_ROUTES_SINK_FILTER_WIDTH}>
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
      </Box>
      <FlexItem grow={1} />
      {canAddRoute && (
        <Button
          label="Add route"
          icon={PlusIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.MEDIUM}
          onClick={onAddRoute}
        />
      )}
    </Flex>
  );
};

export default PipelineCanvasRoutesToolbar;
