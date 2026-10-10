import { create, equals, fromJson, type JsonValue, toJson } from "@bufbuild/protobuf";

import { type WorkerConfiguration, WorkerConfigurationSchema } from "@/gen/ingestion/v1/common_pb";

import { PIPELINE_WORKER_CONFIGURATION_DEFAULT_TEXT } from "@/pages/pipelines/components/worker/constants";

interface ParsedWorkerConfiguration {
  configuration?: WorkerConfiguration;
  error?: string;
}

const stripEmptyValues = (values: Record<string, string>): Record<string, string> =>
  Object.fromEntries(Object.entries(values).filter(([, value]) => value.trim() !== ""));

const isEmpty = (configuration: WorkerConfiguration | undefined) =>
  !configuration ||
  (Object.keys(configuration.resources?.requests ?? {}).length === 0 &&
    Object.keys(configuration.resources?.limits ?? {}).length === 0 &&
    Object.keys(configuration.nodeSelector).length === 0 &&
    configuration.tolerations.length === 0);

export const formatWorkerConfiguration = (
  configuration: WorkerConfiguration | undefined,
): string => {
  if (isEmpty(configuration)) return PIPELINE_WORKER_CONFIGURATION_DEFAULT_TEXT;
  const json = toJson(WorkerConfigurationSchema, configuration as WorkerConfiguration) as Record<
    string,
    unknown
  >;
  return JSON.stringify(
    {
      resources: {
        requests: configuration?.resources?.requests ?? {},
        limits: configuration?.resources?.limits ?? {},
      },
      nodeSelector: json.nodeSelector ?? {},
      tolerations: json.tolerations ?? [],
    },
    null,
    2,
  );
};

export const parseWorkerConfiguration = (text: string): ParsedWorkerConfiguration => {
  if (text.trim() === "") return {};
  let parsed: JsonValue;
  try {
    parsed = JSON.parse(text) as JsonValue;
  } catch {
    return { error: "Enter valid JSON" };
  }
  if (parsed === null || typeof parsed !== "object" || Array.isArray(parsed)) {
    return { error: "Enter a JSON object" };
  }
  try {
    const configuration = fromJson(WorkerConfigurationSchema, parsed);
    if (configuration.resources) {
      configuration.resources.requests = stripEmptyValues(configuration.resources.requests);
      configuration.resources.limits = stripEmptyValues(configuration.resources.limits);
    }
    return isEmpty(configuration) ? {} : { configuration };
  } catch (error) {
    return { error: error instanceof Error ? error.message : "Invalid worker configuration" };
  }
};

export const workerConfigurationEquals = (
  left: WorkerConfiguration | undefined,
  right: WorkerConfiguration | undefined,
): boolean =>
  equals(
    WorkerConfigurationSchema,
    left ?? create(WorkerConfigurationSchema),
    right ?? create(WorkerConfigurationSchema),
  );
