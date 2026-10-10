import type { FC } from "react";

import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { EMPTY_VALUE } from "@galaxy-io/dls/utils/format";

import ConnectionKindChip from "@/components/connections/ConnectionKindChip";
import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";
import KeyValueList from "@/components/KeyValueList";
import KeyValueListRow from "@/components/KeyValueListRow";

import { PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanelResourceSection from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceSection";
import PipelineCanvasPanelHeader from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelHeader";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type {
  PipelineCanvasSinkNode,
  PipelineCanvasSourceNode,
} from "@/pages/pipelines/canvas/types";
import { PipelineCanvasNodeType } from "@/pages/pipelines/canvas/types";
import { usePipelineNodeConfig } from "@/pages/pipelines/components/node/hooks/usePipelineNodeConfig";
import PipelineNodeConfigFields from "@/pages/pipelines/components/node/PipelineNodeConfigFields";

import { formatIdentifier } from "@/utils/naming";

interface PipelineCanvasPanelNodeDetailProps {
  node: PipelineCanvasSourceNode | PipelineCanvasSinkNode;
}

const PipelineCanvasPanelNodeDetail: FC<PipelineCanvasPanelNodeDetailProps> = ({ node }) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const state = usePipelineCanvasState();
  const { clearSelection, setShowPanel } = usePipelineCanvasSelection();
  const { setNodeConfig, removeNode } = usePipelineCanvasActions();
  const nodeEdges = state.edges.filter(
    (edge) => edge.source === node.id || edge.target === node.id,
  );
  const connections = usePipelineCanvasConnections();
  const connection = connections.get(node.id);
  const kind = PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP[node.type];

  const upstreamConnections = new Map(
    state.edges
      .filter((edge) => edge.target === node.id)
      .flatMap((edge) => {
        const upstreamNode = state.nodes.find((candidate) => candidate.id === edge.source);
        const upstreamConnection = connections.get(edge.source);
        return upstreamNode?.type === PipelineCanvasNodeType.SOURCE && upstreamConnection
          ? [[upstreamConnection.id, upstreamConnection] as const]
          : [];
      }),
  );
  const upstreamConnection =
    upstreamConnections.size === 1 ? upstreamConnections.values().next().value : undefined;
  const defaultSchema = upstreamConnection
    ? formatIdentifier(upstreamConnection.name) || undefined
    : undefined;

  const configValue = node.data.config ?? {};
  const nodeConfig = usePipelineNodeConfig(connection, kind, configValue, defaultSchema);

  return (
    <>
      <PipelineCanvasPanelHeader
        title={connection?.name ?? node.data.connectionId}
        tile={
          <ConnectorTile
            connector={connection?.connector ?? ""}
            kind={kind}
            size={ConnectorTileSize.MEDIUM}
            isDeleted={!!connection?.deletedAt}
          />
        }
        onBack={clearSelection}
        onClose={() => setShowPanel(false)}
        onDelete={
          isReadOnly
            ? undefined
            : () => {
                removeNode(node.id);
                clearSelection();
              }
        }
      />
      <FlexItem grow={1} minHeight={0}>
        <ScrollArea>
          <Flex direction={FlexDirection.COLUMN} gap={8} padding={12}>
            <KeyValueList variant={BoxVariant.SECONDARY}>
              <KeyValueListRow
                label="Connector"
                value={<Text size={TextSize.BODY_SM}>{connection?.connector ?? EMPTY_VALUE}</Text>}
              />
              <KeyValueListRow
                label="Kind"
                value={<ConnectionKindChip kind={kind} size={ChipSize.SMALL} />}
              />
            </KeyValueList>

            <PipelineCanvasPanelSection
              header="Configuration"
              isEmpty={nodeConfig.fields.length === 0}
              emptyHeader="No configuration"
              emptyMessage="This connector has no pipeline configuration."
              hasInset
            >
              <PipelineNodeConfigFields
                {...nodeConfig}
                config={configValue}
                onChange={(config) => setNodeConfig(node.id, config)}
                variant={InputVariant.TERTIARY}
                isDisabled={isReadOnly}
              />
            </PipelineCanvasPanelSection>

            <PipelineCanvasPanelResourceSection
              edges={nodeEdges}
              nodeId={node.id}
              nodeType={node.type}
            />
          </Flex>
        </ScrollArea>
      </FlexItem>
    </>
  );
};

export default PipelineCanvasPanelNodeDetail;
