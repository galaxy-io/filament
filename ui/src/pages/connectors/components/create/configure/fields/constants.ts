import { FieldType } from "@/gen/ingestion/v1/common_pb";

import BoolField from "./BoolField";
import EnumField from "./EnumField";
import IntField from "./IntField";
import ObjectField from "./ObjectField";
import SecretField from "./SecretField";
import StringField from "./StringField";
import type { FieldComponent } from "./types";

export const FIELD_COMPONENTS: Partial<Record<FieldType, FieldComponent>> = {
  [FieldType.STRING]: StringField,
  [FieldType.UNSPECIFIED]: StringField,
  [FieldType.DURATION]: StringField,
  [FieldType.INT]: IntField,
  [FieldType.BOOL]: BoolField,
  [FieldType.SECRET]: SecretField,
  [FieldType.ENUM]: EnumField,
  [FieldType.OBJECT]: ObjectField,
};
