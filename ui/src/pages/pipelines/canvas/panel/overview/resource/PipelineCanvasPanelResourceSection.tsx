import { create } from "@bufbuild/protobuf";
import { FlowArrowIcon } from "@phosphor-icons/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { DiscoverResourcesRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanelItem from "@/pages/pipelines/canvas/panel/overview/PipelineCanvasPanelItem";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  type CanvasEdge,
  isConnectionNode,
  PipelineCanvasNodeType,
} from "@/pages/pipelines/canvas/types";
import { getCanvasEdgeResourceLabel } from "@/pages/pipelines/canvas/utils";

import { useDiscoverResourcesQuery } from "@/api/queries/connectors";
import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

const PIPELINE_CANVAS_NODE_TYPE_TO_RESOURCE_EMPTY_MESSAGE_MAP: Record<
  PipelineCanvasNodeType,
  string
> = {
  [PipelineCanvasNodeType.SOURCE]: "Connect this source to a sink to route resources.",
  [PipelineCanvasNodeType.SINK]: "Connect a source to this sink to route resources.",
  [PipelineCanvasNodeType.PLACEHOLDER]: "Connect a source to a sink to route resources.",
};

interface PipelineCanvasPanelResourceSectionProps {
  edges: CanvasEdge[];
  nodeType?: PipelineCanvasNodeType;
  isOpenInitial?: boolean;
}

const PipelineCanvasPanelResourceSection = ({
  edges,
  nodeType = PipelineCanvasNodeType.PLACEHOLDER,
  isOpenInitial,
}: PipelineCanvasPanelResourceSectionProps) => {
  const state = usePipelineCanvasState();
  const { selectResource } = usePipelineCanvasSelection();
  const connectionByNodeId = usePipelineCanvasConnections();

  const connectionNodes = state.nodes.filter(isConnectionNode);

  const sourceConnectionId =
    connectionNodes.find((node) => node.type === PipelineCanvasNodeType.SOURCE)?.data
      .connectionId ?? "";
  const { data: discovered } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, {
      connectionId: sourceConnectionId,
    }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: sourceConnectionId !== "" },
  });
  const discoveredCount = (discovered?.resources ?? []).filter(
    (resource) => resource.isSelectable,
  ).length;

  return (
    <PipelineCanvasPanelSection
      header="Resources"
      isEmpty={edges.length === 0}
      emptyHeader="No resources"
      emptyMessage={PIPELINE_CANVAS_NODE_TYPE_TO_RESOURCE_EMPTY_MESSAGE_MAP[nodeType]}
      isOpenInitial={isOpenInitial}
    >
      <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
        {edges.map((edge) => {
          const sourceConnection = connectionByNodeId.get(edge.source);
          const sinkConnection = connectionByNodeId.get(edge.target);
          const { label, isNamedResource } = getCanvasEdgeResourceLabel(
            getCanvasEdgeResource(edge),
            discoveredCount,
          );
          return (
            <PipelineCanvasPanelItem key={edge.id} onClick={() => selectResource(edge.id)}>
              <FlexWrapper
                alignItems={AlignItems.CENTER}
                justifyContent={JustifyContent.SPACE_BETWEEN}
                gap={FlexGap.SMALL}
                minWidth={0}
                fillWidth
              >
                <FlexItem minWidth={0}>
                  <Text size={TextSize.BODY_SM} isMonospace={isNamedResource} isEllipsis>
                    {label}
                  </Text>
                </FlexItem>
                <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL} shrink={0}>
                  <ConnectorTile
                    connector={sourceConnection?.connector ?? ""}
                    kind={ConnectorKind.SOURCE}
                    size={ConnectorTileSize.SMALL}
                    isDeleted={!!sourceConnection?.deletedAt}
                  />
                  <Icon component={FlowArrowIcon} variant={IconVariant.SECONDARY} size={14} />
                  <ConnectorTile
                    connector={sinkConnection?.connector ?? ""}
                    kind={ConnectorKind.SINK}
                    size={ConnectorTileSize.SMALL}
                    isDeleted={!!sinkConnection?.deletedAt}
                  />
                </FlexWrapper>
              </FlexWrapper>
            </PipelineCanvasPanelItem>
          );
        })}
      </FlexWrapper>
    </PipelineCanvasPanelSection>
  );
};

export default PipelineCanvasPanelResourceSection;
