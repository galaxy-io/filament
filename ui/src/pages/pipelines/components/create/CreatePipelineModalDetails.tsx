import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextAreaInput, { TextAreaSize } from "@galaxy-io/dls/inputs/TextAreaInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

interface CreatePipelineModalDetailsProps {
  name: string;
  nameError?: string;
  description: string;
  onNameChange: (name: string) => void;
  onDescriptionChange: (description: string) => void;
}

const CreatePipelineModalDetails = ({
  name,
  nameError,
  description,
  onNameChange,
  onDescriptionChange,
}: CreatePipelineModalDetailsProps) => {
  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth fillHeight>
      <TextInput
        value={name}
        onChange={onNameChange}
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
        onChange={onDescriptionChange}
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
