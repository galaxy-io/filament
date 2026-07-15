import { CreateConnectionRequest } from "@/gen/ingestion/v1/connections_pb";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

interface CreateConnectionNameInputProps {
  value: CreateConnectionRequest["name"];
  onChange: (value: CreateConnectionRequest["name"]) => void;
  error?: string | null;
  isDisabled?: boolean;
}

const CreateConnectionNameInput = ({
  value,
  onChange,
  error,
  isDisabled = false,
}: CreateConnectionNameInputProps) => {
  return (
    <TextInput
      value={value}
      onChange={onChange}
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

export default CreateConnectionNameInput;
