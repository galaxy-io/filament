import type { Requirement, ValidatePipelineResponse } from "@/gen/ingestion/v1/capabilities_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { stripDeletedName } from "@/utils/format";

const getBlockingRequirementMessages = (requirements: Requirement[]): string[] =>
  requirements
    .filter((requirement) => requirement.blocking)
    .map((requirement) => requirement.message);

export const getPipelineValidationErrors = (
  validation: ValidatePipelineResponse | undefined,
): string[] => [
  ...new Set([
    ...(validation?.errors ?? []).map((error) => error.message),
    ...(validation?.edges ?? []).flatMap((edge) => [
      ...edge.errors.map((error) => error.message),
      ...getBlockingRequirementMessages(edge.requirements),
      ...edge.resources.flatMap((resource) =>
        getBlockingRequirementMessages(resource.requirements),
      ),
    ]),
  ]),
];

export const formatPipelineName = (pipeline: Pipeline, includeDeleted = false): string => {
  const name = includeDeleted ? pipeline.name : stripDeletedName(pipeline.name);
  if (name) {
    return name.replace(/->/g, "→");
  }
  return pipeline.id;
};
