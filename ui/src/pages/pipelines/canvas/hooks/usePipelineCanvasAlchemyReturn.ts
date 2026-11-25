import { useEffect } from "react";

import { parseAlchemyReturnSearch } from "@galaxy-io/alchemy-ui/links";

import { TRANSFORM_ALCHEMY_RETURN_CHANNEL } from "@/pages/pipelines/components/transform/constants";
import type { TransformAlchemyReturnMessage } from "@/pages/pipelines/components/transform/types";

import { useFilamentSearchUpdate, usePipelineCanvasSearch } from "@/module/hooks";
import type { PipelineCanvasSearch } from "@/module/schemas";

export const usePipelineCanvasAlchemyReturn = () => {
  const { alchemyRequest, alchemyFunctionId, alchemyFunctionVersion } = usePipelineCanvasSearch();
  const updateSearch = useFilamentSearchUpdate<PipelineCanvasSearch>();

  useEffect(() => {
    if (!alchemyRequest) return;
    const result = parseAlchemyReturnSearch({ alchemyFunctionId, alchemyFunctionVersion });
    if (result) {
      const channel = new BroadcastChannel(TRANSFORM_ALCHEMY_RETURN_CHANNEL);
      const message: TransformAlchemyReturnMessage = {
        request: alchemyRequest,
        functionId: result.alchemyFunctionId,
        version: Number(result.alchemyFunctionVersion),
      };
      channel.postMessage(message);
      channel.close();
    }
    updateSearch(
      (prev) => ({
        ...prev,
        alchemyRequest: undefined,
        alchemyFunctionId: undefined,
        alchemyFunctionVersion: undefined,
      }),
      { replace: true },
    );
    window.close();
  }, [alchemyRequest, alchemyFunctionId, alchemyFunctionVersion, updateSearch]);
};
