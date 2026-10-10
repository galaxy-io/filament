import type { FC } from "react";

import Widget from "@galaxy-io/dls/widget/Widget";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import CreatePipelineModalDeliveryNodeConfig from "@/pages/pipelines/components/create/steps/components/CreatePipelineModalDeliveryNodeConfig";
import PipelineWorkerConfigurationEditor from "@/pages/pipelines/components/worker/PipelineWorkerConfigurationEditor";

const CreatePipelineModalDeliveryAdvanced: FC = () => {
  const { sourceConnection, sinks, workerConfiguration, workerConfigurationError } =
    useCreatePipelineModalState();
  const { setWorkerConfiguration } = useCreatePipelineModalActions();

  return (
    <>
      {sourceConnection && (
        <CreatePipelineModalDeliveryNodeConfig
          header={`${sourceConnection.name} configuration`}
          connection={sourceConnection}
          kind={ConnectorKind.SOURCE}
        />
      )}
      {sinks.map((sink) => (
        <CreatePipelineModalDeliveryNodeConfig
          key={sink.connection.id}
          header={`${sink.connection.name} configuration`}
          connection={sink.connection}
          kind={ConnectorKind.SINK}
        />
      ))}
      <Widget isCollapsible header="Worker configuration">
        <PipelineWorkerConfigurationEditor
          value={workerConfiguration}
          error={workerConfigurationError}
          onChange={setWorkerConfiguration}
        />
      </Widget>
    </>
  );
};

export default CreatePipelineModalDeliveryAdvanced;
