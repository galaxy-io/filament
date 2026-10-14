import { type FC, useState } from "react";

import { FlowArrowIcon, PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { ConnectorKind, ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";

import { PIPELINE_CANVAS_NODE_SINK_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import { canConnectEdge } from "@/pages/pipelines/canvas/graph/rules";
import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { PIPELINE_CANVAS_NODE_TYPE_TO_RESOURCE_EMPTY_MESSAGE_MAP } from "@/pages/pipelines/canvas/panel/constants";
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
  PipelineCanvasNodeType,
} from "@/pages/pipelines/canvas/types";
import { isConnectionNode } from "@/pages/pipelines/canvas/utils";
import PipelineResourceCreateForm from "@/pages/pipelines/components/resource/PipelineResourceCreateForm";
import type { PipelineResourceCreateState } from "@/pages/pipelines/components/resource/types";
import PipelineTransformFieldsMarker from "@/pages/pipelines/components/transform/PipelineTransformFieldsMarker";
import { usePipelineExecutionMode } from "@/pages/pipelines/hooks/usePipelineExecutionMode";

interface PipelineCanvasPanelResourceSectionProps {
  edges: CanvasEdge[];
  nodeId?: CanvasNode["id"];
  nodeType?: PipelineCanvasNodeType;
  isOpenInitial?: boolean;
}

interface PipelineCanvasPanelResourceSectionState {
  isOpen: boolean;
  isCreating: boolean;
}

const DEFAULT_STATE: PipelineCanvasPanelResourceSectionState = {
  isOpen: true,
  isCreating: false,
};

const PipelineCanvasPanelResourceSection: FC<PipelineCanvasPanelResourceSectionProps> = ({
  edges,
  nodeId,
  nodeType = PipelineCanvasNodeType.PLACEHOLDER,
  isOpenInitial = true,
}) => {
  const state = usePipelineCanvasState();
  const isReadOnly = usePipelineCanvasReadOnly();
  const isContinuous = usePipelineExecutionMode() === ExecutionMode.CONTINUOUS;
  const { connect } = usePipelineCanvasActions();
  const { selectResource } = usePipelineCanvasSelection();
  const connectionByNodeId = usePipelineCanvasConnections();
  const [localState, setLocalState] = useState<PipelineCanvasPanelResourceSectionState>(() => ({
    ...DEFAULT_STATE,
    isOpen: isOpenInitial,
  }));

  const connectionNodes = state.nodes.filter(isConnectionNode);
  const sourceNode = connectionNodes.find((node) => node.type === PipelineCanvasNodeType.SOURCE);
  const sinkNodes = connectionNodes.filter(
    (node) =>
      node.type === PipelineCanvasNodeType.SINK &&
      (nodeType !== PipelineCanvasNodeType.SINK || node.id === nodeId),
  );

  const canCreate = isContinuous && !isReadOnly && !!sourceNode && sinkNodes.length > 0;

  const getCreateError = ({ resource, sinkId }: PipelineResourceCreateState) =>
    sourceNode &&
    canConnectEdge({ source: sourceNode.id, sourceHandle: resource, target: sinkId }, state.edges)
      ? undefined
      : "This resource is already routed to that sink.";

  const handleCreate = ({ resource, sinkId }: PipelineResourceCreateState) => {
    if (!sourceNode) return;
    connect([
      {
        source: sourceNode.id,
        sourceHandle: resource,
        target: sinkId,
        targetHandle: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
      },
    ]);
    setLocalState((prev) => ({ ...prev, isCreating: false }));
  };

  return (
    <PipelineCanvasPanelSection
      header="Resources"
      isEmpty={edges.length === 0 && !localState.isCreating}
      emptyHeader="No resources"
      emptyMessage={PIPELINE_CANVAS_NODE_TYPE_TO_RESOURCE_EMPTY_MESSAGE_MAP[nodeType]}
      isOpen={localState.isOpen}
      onToggle={() => setLocalState((prev) => ({ ...prev, isOpen: !prev.isOpen }))}
      trailing={
        canCreate ? (
          <Button
            label="Add resource"
            icon={PlusIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            onClick={() => setLocalState({ isOpen: true, isCreating: true })}
            isDisabled={localState.isCreating}
          />
        ) : undefined
      }
    >
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} fillWidth>
        {localState.isCreating && (
          <>
            <PipelineResourceCreateForm
              sinks={sinkNodes.map((node) => ({
                id: node.id,
                label: connectionByNodeId.get(node.id)?.name ?? node.data.connectionId,
              }))}
              getError={getCreateError}
              onSave={handleCreate}
              onCancel={() => setLocalState((prev) => ({ ...prev, isCreating: false }))}
            />
            {edges.length > 0 && <Divider />}
          </>
        )}
        {edges.map((edge) => {
          const sourceConnection = connectionByNodeId.get(edge.source);
          const sinkConnection = connectionByNodeId.get(edge.target);
          return (
            <PipelineCanvasPanelItem key={edge.id} onClick={() => selectResource(edge.id)}>
              <Flex
                alignItems={AlignItems.CENTER}
                justifyContent={JustifyContent.SPACE_BETWEEN}
                gap={8}
                minWidth={0}
                fillWidth
              >
                <FlexItem minWidth={0}>
                  <Text size={TextSize.BODY_SM} family={FontFamily.MONO} lineClamp={1}>
                    {getCanvasEdgeResource(edge)}
                  </Text>
                </FlexItem>
                <Flex alignItems={AlignItems.CENTER} gap={8} shrink={0}>
                  {edge.data?.transform && <PipelineTransformFieldsMarker />}
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
                </Flex>
              </Flex>
            </PipelineCanvasPanelItem>
          );
        })}
      </Flex>
    </PipelineCanvasPanelSection>
  );
};

export default PipelineCanvasPanelResourceSection;
