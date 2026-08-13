import type { JsonValue } from "@bufbuild/protobuf";

import type { InputVariant } from "@galaxy-io/dls/inputs/Input";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";

export interface FieldComponentProps {
  field: ConfigField;
  value: JsonValue;
  onChange: (value: JsonValue) => void;
  variant?: InputVariant;
  error?: string;
  isDisabled?: boolean;
  label: string;
  hasStoredSecret?: boolean;
}

export type FieldComponent = React.FC<FieldComponentProps>;
