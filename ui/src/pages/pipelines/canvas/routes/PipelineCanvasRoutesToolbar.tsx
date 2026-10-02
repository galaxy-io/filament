import { useMemo } from "react";

import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

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

interface PipelineCanvasRoutesToolbarProps {
  search: string;
  onSearchChange: (search: string) => void;
  canAddRoute: boolean;
  onAddRoute: () => void;
}

const PipelineCanvasRoutesToolbar = ({
  search,
  onSearchChange,
  canAddRoute,
  onAddRoute,
}: PipelineCanvasRoutesToolbarProps) => {
  const { sinkIds, setSinkIds } = usePipelineCanvasSelection();
  const sinks = usePipelineCanvasRoutesSinks();

  const sinkOptions = useMemo<SelectInputOption[]>(
    () =>
      sinks.map((sink) => ({
        id: sink.nodeId,
        label: sink.label,
        value: sink.nodeId,
        icon: (
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
  const selectedSinkOptions = sinkOptions.filter((option) => sinkIds.includes(option.id));
  const pinnedOptions = [
    {
      id: PIPELINE_CANVAS_ROUTES_SINKS_PINNED_OPTION_ID,
      label: "All sinks",
      optionIds: sinkOptions.map((option) => option.id),
    },
  ];

  const handleSinksChange = (selected: SelectInputOption[]) =>
    setSinkIds(
      selected.length === sinkOptions.length
        ? []
        : selected.map((option) => option.value as CanvasNode["id"]),
    );

  return (
    <FlexWrapper
      alignItems={AlignItems.CENTER}
      gap={FlexGap.MEDIUM}
      padding={`${PIPELINE_CANVAS_VIEW_SWITCHER_INSET}px`}
      shrink={0}
      fillWidth
    >
      <PipelineCanvasViewSwitcher />
      <TextInput
        value={search}
        onChange={onSearchChange}
        placeholder="Search resources"
        leading={{ icon: MagnifyingGlassIcon }}
        size={InputSize.MEDIUM}
        width={PIPELINE_CANVAS_ROUTES_SEARCH_WIDTH}
      />
      <MultiSelectInput
        options={sinkOptions}
        value={selectedSinkOptions}
        onChange={handleSinksChange}
        placeholder="Sinks"
        variant={InputVariant.TERTIARY}
        size={InputSize.MEDIUM}
        width={PIPELINE_CANVAS_ROUTES_SINK_FILTER_WIDTH}
        pinnedOptions={pinnedOptions}
        renderSelectedText={(selected, placeholder) =>
          selected.length ? pluralize("sink", selected.length, true) : placeholder
        }
      />
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
    </FlexWrapper>
  );
};

export default PipelineCanvasRoutesToolbar;
