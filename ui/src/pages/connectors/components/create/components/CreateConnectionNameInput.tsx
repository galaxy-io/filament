import TextInput from "@galaxy-io/dls/inputs/TextInput";

interface CreateConnectionNameInputProps {
  value: string;
  onChange: (value: string) => void;
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
