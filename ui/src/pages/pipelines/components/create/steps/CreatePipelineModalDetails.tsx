import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextAreaInput from "@galaxy-io/dls/inputs/TextAreaInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";

const CreatePipelineModalDetails = () => {
  const { effectiveName, nameError, description } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={16} fillWidth fillHeight>
      <TextInput
        value={effectiveName}
        onChange={(name) =>
          dispatch({ type: CreatePipelineModalActionType.SET_NAME, payload: name })
        }
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
        onChange={(nextDescription) =>
          dispatch({
            type: CreatePipelineModalActionType.SET_DESCRIPTION,
            payload: nextDescription,
          })
        }
        size={InputSize.LARGE}
        placeholder="Enter an optional description..."
        label="Description"
        fillHeight
        fillWidth
      />
    </FlexWrapper>
  );
};

export default CreatePipelineModalDetails;
