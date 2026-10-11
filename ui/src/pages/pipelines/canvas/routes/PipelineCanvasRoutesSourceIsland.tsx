import type { FC } from "react";

import { styled } from "@linaria/react";
import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";

import { PIPELINE_CANVAS_ROUTES_SOURCE_ISLAND_WIDTH } from "@/pages/pipelines/canvas/routes/constants";
import PipelineCanvasRoutesIsland from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesIsland";
import type { PipelineCanvasRoute } from "@/pages/pipelines/canvas/routes/types";

const AddSinkSlot = styled.div`
  flex-shrink: 0;
  visibility: hidden;

  [data-island]:hover &,
  &:focus-within {
    visibility: visible;
  }
`;

interface PipelineCanvasRoutesSourceIslandProps {
  route: PipelineCanvasRoute;
  isSelected: boolean;
  onSelect: () => void;
  onAddSink: (() => void) | undefined;
}

const PipelineCanvasRoutesSourceIsland: FC<PipelineCanvasRoutesSourceIslandProps> = ({
  route,
  isSelected,
  onSelect,
  onAddSink,
}) => (
  <PipelineCanvasRoutesIsland
    width={PIPELINE_CANVAS_ROUTES_SOURCE_ISLAND_WIDTH}
    isSelected={isSelected}
    onSelect={onSelect}
  >
    <ConnectorTile
      connector={route.sourceConnection?.connector ?? ""}
      kind={ConnectorKind.SOURCE}
      size={ConnectorTileSize.SMALL}
      isDeleted={!!route.sourceConnection?.deletedAt}
    />
    <FlexItem shrink={0}>
      <Text size={TextSize.BODY_SM}>{route.sourceConnection?.name ?? route.edge.source}</Text>
    </FlexItem>
    <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} family={FontFamily.MONO}>
      /
    </Text>
    <FlexItem grow={1} minWidth={0}>
      <Text
        size={TextSize.BODY_SM}
        weight={TextWeight.MEDIUM}
        family={route.isNamedResource ? FontFamily.MONO : FontFamily.SANS}
        lineClamp={1}
      >
        {route.resourceLabel}
      </Text>
    </FlexItem>
    {onAddSink && (
      <AddSinkSlot>
        <Button
          icon={PlusIcon}
          ariaLabel="Route to another sink"
          tooltip="Route to another sink"
          variant={ButtonVariant.TERTIARY}
          size={ButtonSize.SMALL}
          onClick={(event) => {
            event.stopPropagation();
            onAddSink();
          }}
        />
      </AddSinkSlot>
    )}
  </PipelineCanvasRoutesIsland>
);

export default PipelineCanvasRoutesSourceIsland;
