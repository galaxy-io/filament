import type { FC } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import { type ConfigField, FieldType } from "@/gen/ingestion/v1/common_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

import { formatFieldLabel } from "@/components/fields/utils";

import PipelineResourceSinkFieldEnum from "@/pages/pipelines/components/resource/PipelineResourceSinkFieldEnum";
import PipelineResourceSinkFieldList from "@/pages/pipelines/components/resource/PipelineResourceSinkFieldList";

interface PipelineResourceSinkFieldProps {
  field: ConfigField;
  value: JsonValue | undefined;
  pipelineValue: JsonValue | undefined;
  columns: ResourceColumn[];
  resourceName: Resource["displayName"];
  isCompact?: boolean;
  isDisabled?: boolean;
  isReadOnly?: boolean;
  onChange: (value: JsonValue | undefined) => void;
}

const PipelineResourceSinkField: FC<PipelineResourceSinkFieldProps> = ({
  field,
  value,
  pipelineValue,
  columns,
  resourceName,
  isCompact = false,
  isDisabled = false,
  isReadOnly = false,
  onChange,
}) => {
  const label = formatFieldLabel(field.name);
  const ariaLabel = `${label} for ${resourceName}`;

  if (field.type === FieldType.LIST) {
    return (
      <PipelineResourceSinkFieldList
        field={field}
        value={value}
        columns={columns}
        label={label}
        ariaLabel={ariaLabel}
        isCompact={isCompact}
        isDisabled={isDisabled}
        isReadOnly={isReadOnly}
        onChange={onChange}
      />
    );
  }

  return (
    <PipelineResourceSinkFieldEnum
      field={field}
      value={value}
      pipelineValue={pipelineValue}
      label={label}
      ariaLabel={ariaLabel}
      isCompact={isCompact}
      isDisabled={isDisabled}
      isReadOnly={isReadOnly}
      onChange={onChange}
    />
  );
};

export default PipelineResourceSinkField;
