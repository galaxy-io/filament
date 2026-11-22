import { useEffect, useRef } from "react";

import { useQueryClient } from "@tanstack/react-query";

import type { AlchemyFunction } from "@/gen/ingestion/v1/alchemy_pb";

import { TRANSFORM_ALCHEMY_RETURN_CHANNEL } from "@/pages/pipelines/components/transform/constants";
import { parseAlchemyReturnMessage } from "@/pages/pipelines/components/transform/utils";

import { createListAlchemyFunctionsQueryKey } from "@/api/queries/alchemy";

export const usePipelineTransformFieldsAlchemyReturn = (
  request: string | null,
  onReturn: (functionId: AlchemyFunction["id"], version: number) => void,
) => {
  const queryClient = useQueryClient();
  const onReturnRef = useRef(onReturn);

  useEffect(() => {
    onReturnRef.current = onReturn;
  });

  useEffect(() => {
    if (!request) return;
    const channel = new BroadcastChannel(TRANSFORM_ALCHEMY_RETURN_CHANNEL);
    channel.onmessage = ({ data }: MessageEvent<unknown>) => {
      const message = parseAlchemyReturnMessage(data);
      if (message?.request !== request) return;
      void queryClient.invalidateQueries({ queryKey: createListAlchemyFunctionsQueryKey() });
      onReturnRef.current(message.functionId, message.version);
    };
    return () => channel.close();
  }, [request, queryClient]);
};
