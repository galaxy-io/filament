import type { FC } from "react";

import { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import ConnectorTile from "@/components/connections/ConnectorTile";
import KeyValueList from "@/components/KeyValueList";
import KeyValueListRow from "@/components/KeyValueListRow";
import { formatPipelineName } from "@/components/pipelines/utils";

import { PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanelItem from "@/pages/pipelines/canvas/panel/overview/PipelineCanvasPanelItem";
import PipelineCanvasPanelResourceSection from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceSection";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  PipelineCanvasNodeType,
  type PipelineCanvasSinkNode,
  type PipelineCanvasSourceNode,
} from "@/pages/pipelines/canvas/types";
import { isConnectionNode } from "@/pages/pipelines/canvas/utils";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";

import { usePipelineParams } from "@/module/hooks";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

import { formatVersion } from "@/utils/format";

const PipelineCanvasPanelOverview: FC = () => {
  const { id } = usePipelineParams();
  const state = usePipelineCanvasState();
  const { selectNode } = usePipelineCanvasSelection();
  const connectionByNodeId = usePipelineCanvasConnections();
  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });
  const previewed = usePipelinePreviewVersion();
  const version = previewed?.version ?? pipelineData.pipeline?.currentVersion?.version;

  const connectionNodes = state.nodes.filter(isConnectionNode);
  const sourceNodes = connectionNodes.filter((node) => node.type === PipelineCanvasNodeType.SOURCE);
  const sinkNodes = connectionNodes.filter((node) => node.type === PipelineCanvasNodeType.SINK);

  const renderNodeItems = (nodes: (PipelineCanvasSourceNode | PipelineCanvasSinkNode)[]) => (
    <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} fillWidth>
      {nodes.map((node) => {
        const connection = connectionByNodeId.get(node.id);
        return (
          <PipelineCanvasPanelItem key={node.id} onClick={() => selectNode(node.id)}>
            <ConnectorTile
              connector={connection?.connector ?? ""}
              kind={PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP[node.type]}
              isDeleted={!!connection?.deletedAt}
            />
            <Text size={TextSize.BODY_SM} lineClamp={1}>
              {connection?.name ?? node.data.connectionId}
            </Text>
          </PipelineCanvasPanelItem>
        );
      })}
    </Flex>
  );

  return (
    <FlexItem grow={1} minHeight={0}>
      <ScrollArea>
        <Flex direction={FlexDirection.COLUMN} gap={8} padding={12}>
          <KeyValueList variant={BoxVariant.SECONDARY}>
            <KeyValueListRow
              label="Name"
              value={
                <Text size={TextSize.BODY_SM}>
                  {pipelineData.pipeline ? formatPipelineName(pipelineData.pipeline) : id}
                </Text>
              }
            />
            <KeyValueListRow
              label="Version"
              value={
                <Text
                  size={TextSize.BODY_SM}
                  variant={previewed ? TextVariant.ERROR : TextVariant.SECONDARY}
                >
                  {formatVersion(version)}
                </Text>
              }
            />
          </KeyValueList>

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
        </Flex>
      </ScrollArea>
    </FlexItem>
  );
};

export default PipelineCanvasPanelOverview;
