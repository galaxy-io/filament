import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Widget, { WidgetSize } from "@galaxy-io/dls/widget/Widget";

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
    <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
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
      <Widget
        isCollapsible
        header="Worker configuration" /* @dls-migrate accordion.padding-other: The body inset follows `size`: remove `padding` (use `isFlush` for 0). */
        padding="16px"
        size={WidgetSize.LARGE}
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
      </Widget>
    </Flex>
  );
};

export default CreatePipelineModalDeliveryAdvanced;
