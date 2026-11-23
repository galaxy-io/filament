import { useEffect, useRef } from "react";

import { useQueryClient } from "@tanstack/react-query";

import { AlchemyVersionStatus } from "@/gen/ingestion/v1/alchemy_pb";

import {
  type TransformEditableStep,
  TransformStepKind,
} from "@/pages/pipelines/components/transform/types";

import {
  createGetAlchemyFunctionInput,
  createListAlchemyFunctionsQueryKey,
  useGetAlchemyFunctionQuery,
} from "@/api/queries/alchemy";
import { createValidateTransformQueryKey } from "@/api/queries/transforms";

export const usePipelineTransformFieldsAlchemyBuild = (step: TransformEditableStep) => {
  const pinned =
    step.kind === TransformStepKind.ALCHEMY && step.function !== "" && step.version > 0
      ? step
      : undefined;
  const queryClient = useQueryClient();
  const { data } = useGetAlchemyFunctionQuery({
    input: createGetAlchemyFunctionInput(pinned?.function ?? "", pinned?.version ?? 0),
    options: { enabled: pinned !== undefined },
  });
  const isBuilding = pinned !== undefined && data?.versionStatus === AlchemyVersionStatus.BUILDING;
  const wasBuilding = useRef(isBuilding);

  useEffect(() => {
    if (wasBuilding.current && !isBuilding) {
      void queryClient.invalidateQueries({ queryKey: createValidateTransformQueryKey() });
      void queryClient.invalidateQueries({ queryKey: createListAlchemyFunctionsQueryKey() });
    }
    wasBuilding.current = isBuilding;
  }, [isBuilding, queryClient]);

  return isBuilding ? "Building" : null;
};
