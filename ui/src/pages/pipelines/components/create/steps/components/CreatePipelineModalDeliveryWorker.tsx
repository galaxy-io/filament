import type { FC } from "react";

import Widget from "@galaxy-io/dls/widget/Widget";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";
import PipelineWorkerConfigurationEditor from "@/pages/pipelines/components/worker/PipelineWorkerConfigurationEditor";

const CreatePipelineModalDeliveryWorker: FC = () => {
  const { workerConfiguration, workerConfigurationError } = useCreatePipelineModalState();
  const { setWorkerConfiguration } = useCreatePipelineModalActions();

  return (
    <Widget isCollapsible header="Worker configuration">
      <PipelineWorkerConfigurationEditor
        value={workerConfiguration}
        error={workerConfigurationError}
        onChange={setWorkerConfiguration}
      />
    </Widget>
  );
};

export default CreatePipelineModalDeliveryWorker;
