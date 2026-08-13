import { create } from "@bufbuild/protobuf";

import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { GetConnectorRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import Field from "@/components/fields/Field";
import {
  getFieldDefaults,
  getPipelineScopedFields,
  isFieldVisible,
} from "@/components/fields/utils";

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

import { useGetConnectorQuery } from "@/api/queries/connectors";

interface PipelineCanvasPanelNodeDetailProps {
  node: PipelineCanvasSourceNode | PipelineCanvasSinkNode;
}

const PipelineCanvasPanelNodeDetail = ({ node }: PipelineCanvasPanelNodeDetailProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const state = usePipelineCanvasState();
  const { clearSelection, setShowPanel } = usePipelineCanvasSelection();
  const { setNodeConfig } = usePipelineCanvasActions();
  const nodeEdges = state.edges.filter(
    (edge) => edge.source === node.id || edge.target === node.id,
  );
  const connection = usePipelineCanvasConnections().get(node.id);
  const kind = PIPELINE_CANVAS_NODE_TYPE_TO_CONNECTOR_KIND_MAP[node.type];

  const { data } = useGetConnectorQuery({
    input: create(GetConnectorRequestSchema, {
      connector: connection?.connector ?? "",
      kind,
    }),
    options: { enabled: !!connection?.connector },
  });
  const spec = data?.connector;

  const configValue = node.data.config ?? {};
  const scopedFields = getPipelineScopedFields(spec?.configSchema?.fields ?? []);
  const displayValue = { ...getFieldDefaults(scopedFields), ...configValue };
  const fields = scopedFields.filter((field) => isFieldVisible(field, displayValue));

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
          isEmpty={fields.length === 0}
          emptyHeader="No configuration"
          emptyMessage="This connector has no pipeline configuration."
          padding="12px"
        >
          <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
            {fields.map((field) => (
              <Field
                key={field.name}
                field={field}
                value={displayValue[field.name] ?? null}
                variant={InputVariant.TERTIARY}
                onChange={(value) =>
                  setNodeConfig(node.id, {
                    ...configValue,
                    [field.name]: value,
                  })
                }
                isDisabled={isReadOnly}
              />
            ))}
          </FlexWrapper>
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
