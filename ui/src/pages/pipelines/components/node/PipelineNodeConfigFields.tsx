import { create, type JsonValue } from "@bufbuild/protobuf";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import type { InputVariant } from "@galaxy-io/dls/inputs/Input";

import type { ConfigField, ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { GetConnectorRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import Field from "@/components/fields/Field";
import {
  getFieldDefaults,
  getPipelineScopedFields,
  isFieldVisible,
  updateConfigField,
} from "@/components/fields/utils";

import { useGetConnectorQuery } from "@/api/queries/connectors";

export type PipelineNodeConfig = Record<string, JsonValue>;

export interface PipelineNodeConfigState {
  scopedFields: ConfigField[];
  fields: ConfigField[];
  displayValue: PipelineNodeConfig;
}

// Resolves a node's pipeline-scope fields against its connection. Visibility
// spans both scopes: a pipeline field may depend on a connection field.
export const usePipelineNodeConfig = (
  connection: Connection | undefined,
  kind: ConnectorKind,
  config: PipelineNodeConfig,
  defaultSchema?: string,
): PipelineNodeConfigState => {
  const { data } = useGetConnectorQuery({
    input: create(GetConnectorRequestSchema, {
      connector: connection?.connector ?? "",
      kind,
    }),
    options: { enabled: !!connection?.connector },
  });
  const connector = data?.connector;
  const allFields = connector?.configSchema?.fields ?? [];
  const scopedFields = getPipelineScopedFields(allFields);
  const schemaDefaults =
    connector?.schemaField && defaultSchema
      ? { [connector.schemaField]: defaultSchema }
      : undefined;
  const values = { ...connection?.config, ...schemaDefaults, ...config };
  const displayValue = { ...getFieldDefaults(allFields, values), ...values };
  const fields = scopedFields.filter((field) => isFieldVisible(field, displayValue));
  return { scopedFields, fields, displayValue };
};

interface PipelineNodeConfigFieldsProps extends PipelineNodeConfigState {
  config: PipelineNodeConfig;
  onChange: (config: PipelineNodeConfig) => void;
  variant?: InputVariant;
  isDisabled?: boolean;
}

const PipelineNodeConfigFields = ({
  scopedFields,
  fields,
  displayValue,
  config,
  onChange,
  variant,
  isDisabled,
}: PipelineNodeConfigFieldsProps) => (
  <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
    {fields.map((field) => (
      <Field
        key={field.name}
        field={field}
        value={displayValue[field.name] ?? null}
        variant={variant}
        onChange={(value) => onChange(updateConfigField(scopedFields, config, field.name, value))}
        isDisabled={isDisabled}
      />
    ))}
  </FlexWrapper>
);

export default PipelineNodeConfigFields;
