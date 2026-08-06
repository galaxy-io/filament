import type { JsonValue } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import Field from "@/components/fields/Field";
import {
  getFieldDefaults,
  getPipelineScopedFields,
  isFieldVisible,
} from "@/components/fields/utils";

import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";
import { PIPELINE_CANVAS_NODE_PADDING } from "@/pages/pipelines/canvas/nodes/constants";
import PipelineCanvasNodeCollapsibleIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeCollapsibleIsland";
import { usePipelineCanvasReadOnly } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

const FieldList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: ${PIPELINE_CANVAS_NODE_PADDING}px ${PIPELINE_CANVAS_NODE_PADDING}px
    ${PIPELINE_CANVAS_NODE_PADDING}px 12px;
`;

interface PipelineCanvasNodeConfigIslandProps {
  connector: string;
  kind: ConnectorKind;
  config?: Record<string, JsonValue>;
  onChange: (config: Record<string, JsonValue>) => void;
  isSelected?: boolean;
  isOpen: boolean;
  onToggle: () => void;
}

const PipelineCanvasNodeConfigIsland = ({
  connector,
  kind,
  config,
  onChange,
  isSelected,
  isOpen,
  onToggle,
}: PipelineCanvasNodeConfigIslandProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const spec = useConnectorSpec(connector, kind);

  const configValue = config ?? {};
  const scopedFields = getPipelineScopedFields(spec?.configSchema?.fields ?? []);
  const displayValue = { ...getFieldDefaults(scopedFields), ...configValue };
  const fields = scopedFields.filter((field) => isFieldVisible(field, displayValue));

  if (fields.length === 0) return null;

  return (
    <PipelineCanvasNodeCollapsibleIsland
      isOpen={isOpen}
      onToggle={onToggle}
      isSelected={isSelected}
      title="Configuration"
    >
      <FieldList className="nodrag">
        {fields.map((field) => (
          <Field
            key={field.name}
            field={field}
            value={displayValue[field.name] ?? null}
            onChange={(value) => onChange({ ...configValue, [field.name]: value })}
            isDisabled={isReadOnly}
          />
        ))}
      </FieldList>
    </PipelineCanvasNodeCollapsibleIsland>
  );
};

export default PipelineCanvasNodeConfigIsland;
