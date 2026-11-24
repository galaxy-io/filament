import { ConnectorKind, FieldType } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import {
  getFieldDefaults,
  getPipelineScopedFields,
  getResourceScopedFields,
} from "@/components/fields/utils";

import type { PipelineNodeConfig } from "@/pages/pipelines/components/node/types";
import type { PipelineResourceSinkFieldsState } from "@/pages/pipelines/components/resource/types";

import { createGetConnectorInput, useGetConnectorQuery } from "@/api/queries/connectors";

export const usePipelineResourceSinkFields = (
  connection: Connection | undefined,
  nodeConfig: PipelineNodeConfig,
): PipelineResourceSinkFieldsState => {
  const { data } = useGetConnectorQuery({
    input: createGetConnectorInput(connection?.connector ?? "", ConnectorKind.SINK),
    options: { enabled: !!connection?.connector },
  });
  const allFields = data?.connector?.configSchema?.fields ?? [];
  const fields = getResourceScopedFields(allFields).filter(
    (field) => field.type === FieldType.ENUM || field.type === FieldType.LIST,
  );
  const pipelineFields = getPipelineScopedFields(allFields);
  const values = { ...getFieldDefaults(pipelineFields), ...nodeConfig };
  const pipelineValues = Object.fromEntries(
    pipelineFields
      .filter((field) => values[field.name] !== undefined)
      .map((field) => [field.name, values[field.name]]),
  );
  return { fields, pipelineValues };
};
