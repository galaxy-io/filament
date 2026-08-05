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
import PipelineCanvasNodeIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeIsland";
import { usePipelineCanvasReadOnly } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

const FieldList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 12px;
`;

interface PipelineCanvasNodeConfigIslandProps {
  connector: string;
  kind: ConnectorKind;
  config?: Record<string, JsonValue>;
  onChange: (config: Record<string, JsonValue>) => void;
  isOpen: boolean;
  isSelected?: boolean;
}

const PipelineCanvasNodeConfigIsland = ({
  connector,
  kind,
  config,
  onChange,
  isOpen,
  isSelected,
}: PipelineCanvasNodeConfigIslandProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const spec = useConnectorSpec(connector, kind);

  const configValue = config ?? {};
  const scopedFields = getPipelineScopedFields(spec?.configSchema?.fields ?? []);
  const displayValue = { ...getFieldDefaults(scopedFields), ...configValue };
  const fields = scopedFields.filter((field) => isFieldVisible(field, displayValue));

  if (!isOpen) return null;
  if (fields.length === 0) return null;

  return (
    <PipelineCanvasNodeIsland $isSelected={isSelected}>
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
    </PipelineCanvasNodeIsland>
  );
};

export default PipelineCanvasNodeConfigIsland;
