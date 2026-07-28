import { useEffect, useRef } from "react";

import type { JsonValue } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useNodeId, useUpdateNodeInternals } from "@xyflow/react";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import ConfigFieldRenderer from "@/components/fields/ConfigFieldRenderer";
import { getPipelineScopedFields, isFieldVisible } from "@/components/fields/utils";

import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";
import { Island } from "@/pages/pipelines/canvas/nodes/PipelineNode";

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
  defaultSchema?: string;
  isSelected?: boolean;
  isDisabled?: boolean;
}

const PipelineNodeConfigIsland = ({
  connector,
  kind,
  config,
  onChange,
  defaultSchema,
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

  // Populate the schema field with its derived default once per open, as soon
  // as the spec is loaded, so the destination is visible and editable. The
  // effect re-runs when the spec resolves but populates only once.
  const schemaField = spec?.schemaField;
  const populated = useRef(false);
  // biome-ignore lint/correctness/useExhaustiveDependencies: populate once per open
  useEffect(() => {
    if (populated.current || !schemaField || !defaultSchema || isDisabled) return;
    populated.current = true;
    const current = configValue[schemaField];
    if (typeof current === "string" && current !== "") return;
    onChange({ ...configValue, [schemaField]: defaultSchema });
  }, [schemaField, defaultSchema]);

  if (fields.length === 0) return null;

  return (
    <Island $isSelected={isSelected}>
      <FieldList className="nodrag">
        {fields.map((field) => (
          <ConfigFieldRenderer
            key={field.name}
            field={field}
            value={configValue[field.name] ?? null}
            onChange={(value) => onChange({ ...configValue, [field.name]: value })}
            isDisabled={isDisabled}
          />
        ))}
      </FieldList>
    </Island>
  );
};

export default PipelineNodeConfigIsland;
