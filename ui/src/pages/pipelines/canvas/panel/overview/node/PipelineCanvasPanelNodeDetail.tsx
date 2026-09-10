import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import ConnectionDrawerKeyValueRow from "@/pages/connectors/components/drawer/ConnectionDrawerKeyValueRow";
import ConnectionDrawerList from "@/pages/connectors/components/drawer/ConnectionDrawerList";
import { PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanelResourceSection from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceSection";
import PipelineCanvasPanelBody from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelBody";
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
import PipelineNodeConfigFields, {
  usePipelineNodeConfig,
} from "@/pages/pipelines/components/node/PipelineNodeConfigFields";

import { normalizeIdentifier } from "@/utils/naming";

interface PipelineCanvasPanelNodeDetailProps {
  node: PipelineCanvasSourceNode | PipelineCanvasSinkNode;
}

const PipelineCanvasPanelNodeDetail = ({ node }: PipelineCanvasPanelNodeDetailProps) => {
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
    ? normalizeIdentifier(upstreamConnection.name) || undefined
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
      <PipelineCanvasPanelBody>
        <ConnectionDrawerList>
          <ConnectionDrawerKeyValueRow
            label="Connector"
            value={<Text size={TextSize.BODY_SM}>{connection?.connector ?? "—"}</Text>}
          />
          <ConnectionDrawerKeyValueRow
            label="Kind"
            value={<ConnectionKindChip kind={kind} size={ChipSize.SMALL} />}
          />
        </ConnectionDrawerList>

        <PipelineCanvasPanelSection
          header="Configuration"
          isEmpty={nodeConfig.fields.length === 0}
          emptyHeader="No configuration"
          emptyMessage="This connector has no pipeline configuration."
          padding="12px"
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
          nodeType={node.type}
          isOpenInitial={false}
        />
      </PipelineCanvasPanelBody>
    </>
  );
};

export default PipelineCanvasPanelNodeDetail;
