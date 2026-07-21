import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import type { CreateConnectionRequest } from "@/gen/ingestion/v1/connections_pb";

interface CreateConnectionConfigureNameInputProps {
  value: CreateConnectionRequest["name"];
  onChange: (value: CreateConnectionRequest["name"]) => void;
  error?: string | null;
  isDisabled?: boolean;
}

const CreateConnectionConfigureNameInput = ({
  value,
  onChange,
  error,
  isDisabled = false,
}: CreateConnectionConfigureNameInputProps) => {
  return (
    <TextInput
      value={value}
      onChange={onChange}
      size={InputSize.LARGE}
      placeholder="Enter connection name..."
      label="Name"
      isRequired
      error={error ?? undefined}
      isDisabled={isDisabled}
      fillWidth
      autoFocus
    />
  );
};

export default CreateConnectionConfigureNameInput;
