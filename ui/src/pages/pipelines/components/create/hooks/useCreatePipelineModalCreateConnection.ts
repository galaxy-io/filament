import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { useCreatePipelineModalState } from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import { mapCreatePipelineModalSeedToSearch } from "@/pages/pipelines/components/create/utils";

import { useFilamentFlowOpen } from "@/module/hooks";
import { Flow } from "@/module/types";

export const useCreatePipelineModalCreateConnection = (connectorKind: ConnectorKind) => {
  const openFlow = useFilamentFlowOpen();
  const { sourceConnection, sinkConnections } = useCreatePipelineModalState();

  return () =>
    openFlow(Flow.CREATE_CONNECTION, {
      ...mapCreatePipelineModalSeedToSearch({ sourceConnection, sinkConnections }),
      connectorKind,
      returnFlow: Flow.CREATE_PIPELINE,
    });
};
