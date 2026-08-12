import { FlowArrowIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile, {
  ConnectorOverflowTile,
  ConnectorTileShimmer,
  ConnectorTileSize,
} from "@/pages/connectors/components/ConnectorTile";

const PIPELINE_FLOW_MAX_VISIBLE_SINKS = 3;

export enum PipelineFlowSize {
  SMALL = "SMALL",
  MEDIUM = "MEDIUM",
  LARGE = "LARGE",
}

export interface PipelineFlowConnection {
  connectionId: Connection["id"];
  connector: Connection["connector"];
  isDeleted?: boolean;
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
  source?: PipelineFlowConnection;
  sinks?: PipelineFlowConnection[];
  size?: PipelineFlowSize;
  hasEdges?: boolean;
  isLoading?: boolean;
}

const PipelineFlow = ({
  source,
  sinks = [],
  size = PipelineFlowSize.MEDIUM,
  hasEdges = true,
  isLoading = false,
}: PipelineFlowProps) => {
  const navigate = useNavigate();

  const visibleSinks = sinks.slice(0, PIPELINE_FLOW_MAX_VISIBLE_SINKS);
  const overflowCount = sinks.length - visibleSinks.length;

  const hasSource = !!source && source.connectionId.length > 0;
  const hasSinks = sinks.length > 0;
  const isLinked = hasSource && hasSinks && hasEdges;

  const handleConnectionClick = (connectionId: Connection["id"], e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId }),
    });
  };

  const renderSource = () => {
    if (hasSource) {
      return (
        <ConnectorTile
          connector={source.connector}
          kind={ConnectorKind.SOURCE}
          onClick={(e) => handleConnectionClick(source.connectionId, e)}
          size={PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size]}
          isDeleted={source.isDeleted}
        />
      );
    }
  };

  const renderSinks = () => {
    if (hasSinks) {
      return (
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL}>
          {visibleSinks.map((sink, index) => (
            <ConnectorTile
              // biome-ignore lint/suspicious/noArrayIndexKey: two sink nodes can share a connection
              key={`${sink.connectionId}-${index}`}
              connector={sink.connector}
              kind={ConnectorKind.SINK}
              size={PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size]}
              onClick={(e) => handleConnectionClick(sink.connectionId, e)}
              isDeleted={sink.isDeleted}
            />
          ))}
          {overflowCount > 0 && (
            <ConnectorOverflowTile
              count={overflowCount}
              size={PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size]}
            />
          )}
        </FlexWrapper>
      );
    }
  };

  if (isLoading) {
    return (
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
        <ConnectorTileShimmer size={PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size]} />
        <Icon
          component={FlowArrowIcon}
          variant={IconVariant.TERTIARY}
          size={PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP[size]}
          weight={IconWeight.REGULAR}
        />
        <ConnectorTileShimmer size={PIPELINE_FLOW_SIZE_TO_CONNECTOR_TILE_SIZE_MAP[size]} />
      </FlexWrapper>
    );
  }

  if (!isLinked) {
    return <Chip label="Invalid pipeline" variant={ChipVariant.WARNING} size={ChipSize.SMALL} />;
  }

  return (
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
      {renderSource()}
      <Icon
        component={FlowArrowIcon}
        variant={IconVariant.PRIMARY}
        size={PIPELINE_FLOW_SIZE_TO_ICON_SIZE_MAP[size]}
        weight={IconWeight.REGULAR}
      />
      {renderSinks()}
    </FlexWrapper>
  );
};

export default PipelineFlow;
