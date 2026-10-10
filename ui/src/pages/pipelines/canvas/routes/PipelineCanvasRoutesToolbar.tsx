import { type FC, useMemo } from "react";

import { PlusIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
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

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import { PIPELINE_CANVAS_VIEW_SWITCHER_INSET } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasViewSwitcher from "@/pages/pipelines/canvas/PipelineCanvasViewSwitcher";
import {
  PIPELINE_CANVAS_ROUTES_SEARCH_WIDTH,
  PIPELINE_CANVAS_ROUTES_SINK_FILTER_WIDTH,
  PIPELINE_CANVAS_ROUTES_SINKS_PINNED_OPTION_ID,
} from "@/pages/pipelines/canvas/routes/constants";
import { usePipelineCanvasRoutesSinks } from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutesSinks";
import type { CanvasNode } from "@/pages/pipelines/canvas/types";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/api/utils";

import { getSelectAllChange, getSelectAllOptions, getSelectAllValue } from "@/utils/select";

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
  const selectAll = {
    id: PIPELINE_CANVAS_ROUTES_SINKS_PINNED_OPTION_ID,
    label: "All sinks",
    optionIds: sinkOptions.map((option) => option.id),
  };
  const selectedSinkIds = sinkIds.length ? sinkIds : selectAll.optionIds;

  const handleSinksChange = (ids: CanvasNode["id"][]) => {
    const next = getSelectAllChange(selectAll, ids, selectedSinkIds);
    setSinkIds(next.length === sinkOptions.length ? [] : next);
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
          options={getSelectAllOptions(selectAll, sinkOptions)}
          pinnedIds={[selectAll.id]}
          value={getSelectAllValue(selectAll, selectedSinkIds)}
          onChange={handleSinksChange}
          placeholder="Sinks..."
          variant={MultiSelectInputVariant.TERTIARY}
          size={MultiSelectInputSize.MEDIUM}
          renderValue={(options) => (
            <Text>
              {pluralize(
                "sink",
                options.filter((option) => option.id !== selectAll.id).length,
                true,
              )}
            </Text>
          )}
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
