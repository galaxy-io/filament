import { create } from "@bufbuild/protobuf";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { GetConnectorRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import {
  getFieldDefaults,
  getPipelineScopedFields,
  isFieldVisible,
} from "@/components/fields/utils";

import type {
  PipelineNodeConfig,
  PipelineNodeConfigState,
} from "@/pages/pipelines/components/node/types";

import { useGetConnectorQuery } from "@/api/queries/connectors";

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
