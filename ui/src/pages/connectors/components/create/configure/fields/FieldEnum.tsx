import { useState } from "react";

import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import DropdownButton from "@galaxy-io/dls/dropdown/DropdownButton";
import DropdownItem from "@galaxy-io/dls/dropdown/DropdownItem";

import Field from "@/pages/connectors/components/create/configure/fields/Field";
import type { FieldComponentProps } from "@/pages/connectors/components/create/configure/fields/types";

const FieldEnum = ({
  field,
  value,
  onChange,
  error,
  isDisabled = false,
  label,
}: FieldComponentProps) => {
  const [isEnumOpen, setIsEnumOpen] = useState(false);

  return (
    <Field label={label} help={field.help} isRequired={field.required} error={error}>
      <Dropdown
        isOpen={isEnumOpen}
        onClose={() => setIsEnumOpen(false)}
        position={DropdownPosition.BOTTOM_START}
        fillWidth
        body={field.enum.map((enumValue) => (
          <DropdownItem
            key={enumValue}
            label={enumValue}
            onClick={() => {
              onChange(enumValue);
              setIsEnumOpen(false);
            }}
          />
        ))}
      >
        <DropdownButton
          label={(value as string) || `Select ${label}...`}
          isOpen={isEnumOpen}
          onClick={() => setIsEnumOpen(true)}
          isDisabled={isDisabled}
          fillWidth
        />
      </Dropdown>
    </Field>
  );
};

export default FieldEnum;
