import type { FC } from "react";

import CheckboxInput from "@galaxy-io/dls/inputs/CheckboxInput";
import Widget from "@galaxy-io/dls/widget/Widget";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";

const CreatePipelineModalDeliveryFailure: FC = () => {
  const { failOnError } = useCreatePipelineModalState();
  const { setFailOnError } = useCreatePipelineModalActions();

  return (
    <Widget isCollapsible header="Resource failures">
      <CheckboxInput
        label="Fail the run when a resource fails"
        description="Stop every resource at the first failure. When off, the other resources finish and the run ends Partial."
        isChecked={failOnError}
        onChange={setFailOnError}
      />
    </Widget>
  );
};

export default CreatePipelineModalDeliveryFailure;
