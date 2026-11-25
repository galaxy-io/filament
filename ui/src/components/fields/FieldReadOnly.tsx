import type { FC } from "react";

import CopyInput from "@galaxy-io/dls/inputs/CopyInput";

import type { FieldComponentProps } from "@/components/fields/types";

const FieldReadOnly: FC<FieldComponentProps> = ({ field, value, variant, label }) => (
  <CopyInput
    label={label}
    labelTooltip={field.help || undefined}
    value={String(value)}
    variant={variant}
    fillWidth
  />
);

export default FieldReadOnly;
