import type { FC } from "react";

import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import { PIPELINE_CANVAS_ROUTES_SINK_ISLAND_WIDTH } from "@/pages/pipelines/canvas/routes/constants";
import PipelineCanvasRoutesIsland from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutesIsland";
import type { PipelineCanvasRoute } from "@/pages/pipelines/canvas/routes/types";

interface PipelineCanvasRoutesSinkIslandProps {
  route: PipelineCanvasRoute;
  isSelected: boolean;
  onSelect: () => void;
}

const PipelineCanvasRoutesSinkIsland: FC<PipelineCanvasRoutesSinkIslandProps> = ({
  route,
  isSelected,
  onSelect,
}) => (
  <PipelineCanvasRoutesIsland
    width={PIPELINE_CANVAS_ROUTES_SINK_ISLAND_WIDTH}
    isSelected={isSelected}
    onSelect={onSelect}
  >
    <ConnectorTile
      connector={route.sinkConnection?.connector ?? ""}
      kind={ConnectorKind.SINK}
      size={ConnectorTileSize.SMALL}
      isDeleted={!!route.sinkConnection?.deletedAt}
    />
    <FlexItem minWidth={0}>
      <Text size={TextSize.BODY_SM} lineClamp={1}>
        {route.sinkConnection?.name ?? route.edge.target}
      </Text>
    </FlexItem>
  </PipelineCanvasRoutesIsland>
);

export default PipelineCanvasRoutesSinkIsland;
