import Accordion, { AccordionSize } from "@galaxy-io/dls/accordion/Accordion";
import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryNodeConfig from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryNodeConfig";
import PipelineWorkerConfigurationEditor from "@/pages/pipelines/components/worker/PipelineWorkerConfigurationEditor";

const CreatePipelineModalDeliveryAdvanced = () => {
  const { sourceConnection, sinks, workerConfiguration, workerConfigurationError } =
    useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.MEDIUM} fillWidth>
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
      <Accordion header="Worker configuration" padding="16px" size={AccordionSize.LARGE}>
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
      </Accordion>
    </FlexWrapper>
  );
};

export default CreatePipelineModalDeliveryAdvanced;
