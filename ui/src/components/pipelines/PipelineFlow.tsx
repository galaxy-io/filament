import type { FC } from "react";

import { FlowArrowIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import {
  ConnectorOverflowTile,
  ConnectorTileShimmer,
  ConnectorTileSize,
} from "@/components/connections/ConnectorTile";
import PipelineFlowTile from "@/components/pipelines/PipelineFlowTile";

const PIPELINE_FLOW_MAX_VISIBLE_SINKS = 3;

export enum PipelineFlowSize {
  SMALL = "SMALL",
  MEDIUM = "MEDIUM",
  LARGE = "LARGE",
}

const PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP: Record<PipelineFlowSize, ConnectorTileSize> = {
  [PipelineFlowSize.SMALL]: ConnectorTileSize.SMALL,
  [PipelineFlowSize.MEDIUM]: ConnectorTileSize.MEDIUM,
  [PipelineFlowSize.LARGE]: ConnectorTileSize.LARGE,
};

const PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP: Record<PipelineFlowSize, number> = {
  [PipelineFlowSize.SMALL]: 12,
  [PipelineFlowSize.MEDIUM]: 16,
  [PipelineFlowSize.LARGE]: 20,
};

interface PipelineFlowProps {
  sourceId?: Connection["id"];
  sinkIds?: Connection["id"][];
  size?: PipelineFlowSize;
  hasEdges?: boolean;
  isLoading?: boolean;
}

const PipelineFlow: FC<PipelineFlowProps> = ({
  sourceId,
  sinkIds = [],
  size = PipelineFlowSize.MEDIUM,
  hasEdges = true,
  isLoading = false,
}) => {
  const visibleSinkIds = sinkIds.slice(0, PIPELINE_FLOW_MAX_VISIBLE_SINKS);
  const overflowCount = sinkIds.length - visibleSinkIds.length;

  const hasSource = !!sourceId;
  const hasSinks = sinkIds.length > 0;
  const isLinked = hasSource && hasSinks && hasEdges;

  const renderSource = () => {
    if (sourceId) {
      return (
        <PipelineFlowTile
          connectionId={sourceId}
          kind={ConnectorKind.SOURCE}
          size={PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size]}
        />
      );
    }
  };

  const renderSinks = () => {
    if (hasSinks) {
      return (
        <Flex alignItems={AlignItems.CENTER} gap={4}>
          {visibleSinkIds.map((sinkId, index) => (
            <PipelineFlowTile
              // biome-ignore lint/suspicious/noArrayIndexKey: two sink nodes can share a connection
              key={`${sinkId}-${index}`}
              connectionId={sinkId}
              kind={ConnectorKind.SINK}
              size={PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size]}
            />
          ))}
          {overflowCount > 0 && (
            <ConnectorOverflowTile
              count={overflowCount}
              size={PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size]}
            />
          )}
        </Flex>
      );
    }
  };

  if (isLoading) {
    return (
      <Flex alignItems={AlignItems.CENTER} gap={8}>
        <ConnectorTileShimmer size={PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size]} />
        <Icon
          component={FlowArrowIcon}
          variant={IconVariant.TERTIARY}
          size={PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP[size]}
          weight={IconWeight.REGULAR}
        />
        <ConnectorTileShimmer size={PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size]} />
      </Flex>
    );
  }

  if (!isLinked) {
    return <Chip label="Invalid pipeline" variant={ChipVariant.ERROR} size={ChipSize.SMALL} />;
  }

  return (
    <Flex alignItems={AlignItems.CENTER} gap={8}>
      {renderSource()}
      <Icon
        component={FlowArrowIcon}
        variant={IconVariant.PRIMARY}
        size={PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP[size]}
        weight={IconWeight.REGULAR}
      />
      {renderSinks()}
    </Flex>
  );
};

export default PipelineFlow;
