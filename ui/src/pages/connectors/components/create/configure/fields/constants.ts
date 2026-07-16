import FieldBoolean from "@/pages/connectors/components/create/configure/fields/FieldBoolean";
import FieldEnum from "@/pages/connectors/components/create/configure/fields/FieldEnum";
import FieldInt from "@/pages/connectors/components/create/configure/fields/FieldInt";
import FieldObject from "@/pages/connectors/components/create/configure/fields/FieldObject";
import FieldSecret from "@/pages/connectors/components/create/configure/fields/FieldSecret";
import FieldString from "@/pages/connectors/components/create/configure/fields/FieldString";
import type { FieldComponent } from "@/pages/connectors/components/create/configure/fields/types";

import { FieldType } from "@/gen/ingestion/v1/common_pb";

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
