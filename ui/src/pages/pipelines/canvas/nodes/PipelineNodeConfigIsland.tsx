import { useEffect } from "react";

import type { JsonValue } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useNodeId, useUpdateNodeInternals } from "@xyflow/react";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import Field from "@/components/fields/Field";
import { getPipelineScopedFields, isFieldVisible } from "@/components/fields/utils";

import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";
import PipelineNodeIsland from "@/pages/pipelines/canvas/nodes/PipelineNodeIsland";

const FieldList = styled.div`
  display: flex;
  flex-direction: column;
  gap: 12px;
`;

interface PipelineNodeConfigIslandProps {
  connector: string;
  kind: ConnectorKind;
  config?: Record<string, JsonValue>;
  onChange: (config: Record<string, JsonValue>) => void;
  isSelected?: boolean;
  isDisabled?: boolean;
}

const PipelineNodeConfigIsland = ({
  connector,
  kind,
  config,
  onChange,
  isSelected,
  isDisabled = false,
}: PipelineNodeConfigIslandProps) => {
  const nodeId = useNodeId();
  const updateNodeInternals = useUpdateNodeInternals();
  const spec = useConnectorSpec(connector, kind);

  const configValue = config ?? {};
  const fields = getPipelineScopedFields(spec?.configSchema?.fields ?? []).filter((field) =>
    isFieldVisible(field, configValue),
  );

  // biome-ignore lint/correctness/useExhaustiveDependencies: re-measure on content changes
  useEffect(() => {
    if (nodeId) {
      updateNodeInternals(nodeId);
    }
  }, [fields.length]);

  if (fields.length === 0) return null;

  return (
    <PipelineNodeIsland $isSelected={isSelected}>
      <FieldList className="nodrag">
        {fields.map((field) => (
          <Field
            key={field.name}
            field={field}
            value={configValue[field.name] ?? null}
            onChange={(value) => onChange({ ...configValue, [field.name]: value })}
            isDisabled={isDisabled}
          />
        ))}
      </FieldList>
    </PipelineNodeIsland>
  );
};

export default PipelineNodeConfigIsland;
