import { FieldType } from "@/gen/ingestion/v1/common_pb";

import FieldBoolean from "@/components/fields/FieldBoolean";
import FieldEnum from "@/components/fields/FieldEnum";
import FieldInt from "@/components/fields/FieldInt";
import FieldObject from "@/components/fields/FieldObject";
import FieldSecret from "@/components/fields/FieldSecret";
import FieldString from "@/components/fields/FieldString";
import type { FieldComponent } from "@/components/fields/types";

export const FIELD_TYPE_TO_FIELD_COMPONENT_MAP: Partial<Record<FieldType, FieldComponent>> = {
  [FieldType.STRING]: FieldString,
  [FieldType.UNSPECIFIED]: FieldString,
  [FieldType.DURATION]: FieldString,
  [FieldType.INT]: FieldInt,
  [FieldType.BOOL]: FieldBoolean,
  [FieldType.SECRET]: FieldSecret,
  [FieldType.ENUM]: FieldEnum,
  [FieldType.OBJECT]: FieldObject,
};
