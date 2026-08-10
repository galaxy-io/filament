import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextAreaInput, { TextAreaSize } from "@galaxy-io/dls/inputs/TextAreaInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import {
  useCreatePipelineModalActions,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";

const CreatePipelineModalDetails = () => {
  const { effectiveName, nameError, description } = useCreatePipelineModalState();
  const { setName, setDescription } = useCreatePipelineModalActions();

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth fillHeight>
      <TextInput
        value={effectiveName}
        onChange={setName}
        size={InputSize.LARGE}
        placeholder="Enter pipeline name..."
        label="Name"
        isRequired
        error={nameError}
        fillWidth
        autoFocus
      />
      <TextAreaInput
        value={description}
        onChange={setDescription}
        size={TextAreaSize.LARGE}
        placeholder="Enter an optional description..."
        label="Description"
        fillHeight
        fillWidth
      />
    </FlexWrapper>
  );
};

export default CreatePipelineModalDetails;
