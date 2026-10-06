import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextAreaInput, { TextAreaInputSize } from "@galaxy-io/dls/inputs/TextAreaInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { CreatePipelineModalActionType } from "@/pages/pipelines/components/create/actions";
import {
  useCreatePipelineModalDispatch,
  useCreatePipelineModalState,
} from "@/pages/pipelines/components/create/CreatePipelineModalProvider";

const CreatePipelineModalDetails = () => {
  const { effectiveName, nameError, description } = useCreatePipelineModalState();
  const dispatch = useCreatePipelineModalDispatch();

  return (
    <Flex
      alignItems={AlignItems.START}
      direction={FlexDirection.COLUMN}
      gap={16}
      fillWidth
      height="100%"
    >
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
        size={TextAreaInputSize.LARGE}
        placeholder="Enter an optional description..."
        label="Description"
        /* @dls-migrate textareainput.fillHeight: Size the field in rows (`minRows`, `maxRows`) or let it grow with `isAutoGrow`. */ fillHeight
        fillWidth
      />
    </Flex>
  );
};

export default CreatePipelineModalDetails;
