import { create } from "@bufbuild/protobuf";
import { useParams } from "@tanstack/react-router";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import ConnectionDrawerKeyValueRow from "@/pages/connectors/components/drawer/ConnectionDrawerKeyValueRow";
import ConnectionDrawerList from "@/pages/connectors/components/drawer/ConnectionDrawerList";
import { PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanelItem from "@/pages/pipelines/canvas/panel/overview/PipelineCanvasPanelItem";
import PipelineCanvasPanelResourceSection from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceSection";
import PipelineCanvasPanelBody from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelBody";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  isConnectionNode,
  PipelineCanvasNodeType,
  type PipelineCanvasSinkNode,
  type PipelineCanvasSourceNode,
} from "@/pages/pipelines/canvas/types";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

const PipelineCanvasPanelOverview = () => {
  const { id } = useParams({ from: "/pipelines/$id" });
  const state = usePipelineCanvasState();
  const { selectNode } = usePipelineCanvasSelection();
  const connectionByNodeId = usePipelineCanvasConnections();
  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const previewed = usePipelinePreviewVersion();
  const version = previewed?.version ?? pipelineData.pipeline?.currentVersion?.version;

  const connectionNodes = state.nodes.filter(isConnectionNode);
  const sourceNodes = connectionNodes.filter((node) => node.type === PipelineCanvasNodeType.SOURCE);
  const sinkNodes = connectionNodes.filter((node) => node.type === PipelineCanvasNodeType.SINK);

  const renderNodeItems = (nodes: (PipelineCanvasSourceNode | PipelineCanvasSinkNode)[]) => (
    <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
      {nodes.map((node) => {
        const connection = connectionByNodeId.get(node.id);
        return (
          <PipelineCanvasPanelItem key={node.id} onClick={() => selectNode(node.id)}>
            <ConnectorTile
              connector={connection?.connector ?? ""}
              kind={PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP[node.type]}
              isDeleted={!!connection?.deletedAt}
            />
            <Text size={TextSize.BODY_SM} isEllipsis>
              {connection?.name ?? node.data.connectionId}
            </Text>
          </PipelineCanvasPanelItem>
        );
      })}
    </FlexWrapper>
  );

  return (
    <PipelineCanvasPanelBody>
      <ConnectionDrawerList>
        <ConnectionDrawerKeyValueRow
          label="Name"
          value={
            <Text size={TextSize.BODY_SM}>
              {pipelineData.pipeline ? formatPipelineName(pipelineData.pipeline) : id}
            </Text>
          }
        />
        <ConnectionDrawerKeyValueRow
          label="Version"
          value={
            <Text
              size={TextSize.BODY_SM}
              variant={previewed ? TextVariant.ERROR : TextVariant.SECONDARY}
            >
              {version ? `Version ${version}` : "—"}
            </Text>
          }
        />
      </ConnectionDrawerList>

      <PipelineCanvasPanelSection
        header="Source"
        isEmpty={sourceNodes.length === 0}
        emptyHeader="No source"
        emptyMessage="This pipeline has no source connection."
      >
        {renderNodeItems(sourceNodes)}
      </PipelineCanvasPanelSection>

      <PipelineCanvasPanelSection
        header="Sinks"
        isEmpty={sinkNodes.length === 0}
        emptyHeader="No sinks"
        emptyMessage="This pipeline has no sink connections."
      >
        {renderNodeItems(sinkNodes)}
      </PipelineCanvasPanelSection>

      <PipelineCanvasPanelResourceSection edges={state.edges} />
    </PipelineCanvasPanelBody>
  );
};

export default PipelineCanvasPanelOverview;
