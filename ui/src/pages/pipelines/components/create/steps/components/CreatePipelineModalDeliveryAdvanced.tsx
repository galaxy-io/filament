import type { FC } from "react";

import Fieldset from "@galaxy-io/dls/inputs/Fieldset";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryNodeConfig from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryNodeConfig";
import PipelineWorkerConfigurationEditor from "@/pages/pipelines/components/worker/PipelineWorkerConfigurationEditor";

const CreatePipelineModalDeliveryAdvanced: FC = () => {
  const { sourceConnection, sinks, workerConfiguration, workerConfigurationError } =
    useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12} fillWidth>
      {sourceConnection && (
        <CreatePipelineModalDeliveryNodeConfig
          header="Source configuration"
          connection={sourceConnection}
          kind={ConnectorKind.SOURCE}
        />
      )}
      {sinks.map((sink) => (
        <CreatePipelineModalDeliveryNodeConfig
          key={sink.connection.id}
          header="Sink configuration"
          connection={sink.connection}
          kind={ConnectorKind.SINK}
        />
      ))}
      <Fieldset
        label="Worker configuration"
        description="Pod template merged into every run's worker."
      >
        <PipelineWorkerConfigurationEditor
          value={workerConfiguration}
          error={workerConfigurationError}
          onChange={(payload) =>
            dispatch({
              type: CreatePipelineModalActionType.SET_WORKER_CONFIGURATION,
              payload,
            })
          }
        />
      </Fieldset>
    </Flex>
  );
};

export default CreatePipelineModalDeliveryAdvanced;
