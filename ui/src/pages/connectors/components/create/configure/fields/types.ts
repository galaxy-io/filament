import type { JsonValue } from "@bufbuild/protobuf";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";

export interface FieldComponentProps {
  field: ConfigField;
  value: JsonValue;
  onChange: (value: JsonValue) => void;
  error?: string;
  isDisabled?: boolean;
  label: string;
}

export type FieldComponent = React.FC<FieldComponentProps>;
