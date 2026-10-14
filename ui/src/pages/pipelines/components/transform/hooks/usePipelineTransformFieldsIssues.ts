import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { ValidateTransformRequestSchema } from "@/gen/ingestion/v1/transformations_pb";

import { groupTransformIssuesByStep } from "@/pages/pipelines/components/transform/grammar/paths";
import { serializeTransformDefinition } from "@/pages/pipelines/components/transform/grammar/serialize";
import {
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import type { TransformStep } from "@/pages/pipelines/components/transform/types";

import { useValidateTransformQuery } from "@/api/queries/transforms";

export const usePipelineTransformFieldsIssues = (): Map<TransformStep["id"], string[]> => {
  const { steps } = usePipelineTransformFieldsState();
  const { resource, sourceConnectionId, grammarVersion } = usePipelineTransformFieldsEnvironment();
  const definition = useMemo(
    () => serializeTransformDefinition(resource, steps, grammarVersion),
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
