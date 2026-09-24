import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { FlowArrowIcon, PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { ConnectorKind, ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import { DiscoverResourcesRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import { PIPELINE_CANVAS_NODE_SINK_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import { canConnectEdge } from "@/pages/pipelines/canvas/graph/rules";
import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanelItem from "@/pages/pipelines/canvas/panel/overview/PipelineCanvasPanelItem";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  type CanvasEdge,
  type CanvasNode,
  isConnectionNode,
  PipelineCanvasNodeType,
} from "@/pages/pipelines/canvas/types";
import { getCanvasEdgeResourceLabel } from "@/pages/pipelines/canvas/utils";
import PipelineResourceCreateForm, {
  type PipelineResourceCreateState,
} from "@/pages/pipelines/components/resource/PipelineResourceCreateForm";
import { usePipelineExecutionMode } from "@/pages/pipelines/hooks/usePipelineExecutionMode";

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
  nodeId?: CanvasNode["id"];
  nodeType?: PipelineCanvasNodeType;
  isOpenInitial?: boolean;
}

const PipelineCanvasPanelResourceSection = ({
  edges,
  nodeId,
  nodeType = PipelineCanvasNodeType.PLACEHOLDER,
  isOpenInitial = true,
}: PipelineCanvasPanelResourceSectionProps) => {
  const state = usePipelineCanvasState();
  const isReadOnly = usePipelineCanvasReadOnly();
  const isContinuous = usePipelineExecutionMode() === ExecutionMode.CONTINUOUS;
  const { connect } = usePipelineCanvasActions();
  const { selectResource } = usePipelineCanvasSelection();
  const connectionByNodeId = usePipelineCanvasConnections();
  const [isOpen, setIsOpen] = useState(isOpenInitial);
  const [isCreating, setIsCreating] = useState(false);

  const connectionNodes = state.nodes.filter(isConnectionNode);
  const sourceNode = connectionNodes.find((node) => node.type === PipelineCanvasNodeType.SOURCE);
  const sinkNodes = connectionNodes.filter(
    (node) =>
      node.type === PipelineCanvasNodeType.SINK &&
      (nodeType !== PipelineCanvasNodeType.SINK || node.id === nodeId),
  );
  const sourceConnectionId = sourceNode?.data.connectionId ?? "";

  const { data: discovered } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, {
      connectionId: sourceConnectionId,
    }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: sourceConnectionId !== "" },
  });
  const discoveredCount = (discovered?.resources ?? []).filter(
    (resource) => resource.isSelectable,
  ).length;

  const canCreate = isContinuous && !isReadOnly && !!sourceNode && sinkNodes.length > 0;

  const getCreateError = ({ resource, sinkId }: PipelineResourceCreateState) =>
    sourceNode &&
    canConnectEdge({ source: sourceNode.id, sourceHandle: resource, target: sinkId }, state.edges)
      ? null
      : "This subject is already routed to that sink.";

  const handleCreate = ({ resource, sinkId }: PipelineResourceCreateState) => {
    if (!sourceNode) return;
    connect({
      source: sourceNode.id,
      sourceHandle: resource,
      target: sinkId,
      targetHandle: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
    });
    setIsCreating(false);
  };

  return (
    <PipelineCanvasPanelSection
      header="Resources"
      isEmpty={edges.length === 0 && !isCreating}
      emptyHeader="No resources"
      emptyMessage={PIPELINE_CANVAS_NODE_TYPE_TO_RESOURCE_EMPTY_MESSAGE_MAP[nodeType]}
      isOpen={isOpen}
      onToggle={() => setIsOpen((prev) => !prev)}
      headerAction={
        canCreate ? (
          <Button
            label="Add resource"
            icon={PlusIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            onClick={() => {
              setIsOpen(true);
              setIsCreating(true);
            }}
            isDisabled={isCreating}
          />
        ) : undefined
      }
      headerActionWidth={128}
    >
      <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
        {isCreating && (
          <>
            <PipelineResourceCreateForm
              sinks={sinkNodes.map((node) => ({
                id: node.id,
                label: connectionByNodeId.get(node.id)?.name ?? node.data.connectionId,
              }))}
              getError={getCreateError}
              onSave={handleCreate}
              onCancel={() => setIsCreating(false)}
            />
            {edges.length > 0 && <HorizontalDivider />}
          </>
        )}
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
