import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import type { Resource } from "@/gen/ingestion/v1/connectors_pb";
import { ValidateTransformRequestSchema } from "@/gen/ingestion/v1/transformations_pb";

import { groupTransformIssuesByStep } from "@/components/transform/grammar/paths";
import { serializeTransformDefinition } from "@/components/transform/grammar/serialize";
import {
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/components/transform/PipelineTransformFieldsProvider";
import type { TransformStep } from "@/components/transform/types";

import { useValidateTransformQuery } from "@/api/queries/transforms";

const NO_STEPS: TransformStep[] = [];

export const usePipelineTransformFieldsIssues = (
  resource: Resource["name"],
): Map<TransformStep["id"], string[]> => {
  const { stepsByResource } = usePipelineTransformFieldsState();
  const { sourceConnectionId, grammarVersion } = usePipelineTransformFieldsEnvironment();
  const steps = stepsByResource.get(resource) ?? NO_STEPS;
  const definition = useMemo(
    () => serializeTransformDefinition(new Map([[resource, steps]]), grammarVersion),
    [grammarVersion, resource, steps],
  );
  const { data } = useValidateTransformQuery({
    input: create(ValidateTransformRequestSchema, {
      sourceConnectionId,
      resource,
      transform: definition,
    }),
    options: { enabled: sourceConnectionId !== "" && definition !== undefined },
  });
  return useMemo(
    () =>
      new Map(
        [...groupTransformIssuesByStep(data?.issues ?? [])].flatMap(([index, messages]) => {
          const step = steps[index];
          return step ? [[step.id, messages]] : [];
        }),
      ),
    [data, steps],
  );
};
