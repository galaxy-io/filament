import { type FC, useState } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import MultiSelectInput, { MultiSelectInputVariant } from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";

import type { ConfigField } from "@/gen/ingestion/v1/common_pb";
import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

import { PIPELINE_RESOURCE_PARTITION_BY_FIELD } from "@/pages/pipelines/components/resource/constants";
import {
  getPartitionByOptions,
  getPartitionColumn,
  getPartitionTimeColumn,
} from "@/pages/pipelines/components/resource/utils";

interface PipelineResourceSinkFieldListProps {
  field: ConfigField;
  value: JsonValue | undefined;
  columns: ResourceColumn[];
  label: string;
  ariaLabel: string;
  isCompact: boolean;
  isDisabled: boolean;
  isReadOnly: boolean;
  onChange: (value: JsonValue | undefined) => void;
}

const PipelineResourceSinkFieldList: FC<PipelineResourceSinkFieldListProps> = ({
  field,
  value,
  columns,
  label,
  ariaLabel,
  isCompact,
  isDisabled,
  isReadOnly,
  onChange,
}) => {
  const selected = Array.isArray(value)
    ? value.filter((entry): entry is string => typeof entry === "string")
    : [];
  const suggested =
    field.name === PIPELINE_RESOURCE_PARTITION_BY_FIELD ? getPartitionByOptions(columns) : [];
  const timeColumns = selected.map(getPartitionTimeColumn);
  const options: SelectOption[] = [
    ...suggested.map((option) => {
      const column = getPartitionTimeColumn(option.id);
      return column && timeColumns.includes(column) && !selected.includes(option.id)
        ? { ...option, isDisabled: true, disabledReason: "One time transform per column" }
        : option;
    }),
    ...selected
      .filter((entry) => !suggested.some((option) => option.id === entry))
      .map((entry) => ({ id: entry, label: entry })),
  ];

  const [error, setError] = useState<string>();
  const isPartitionField = field.name === PIPELINE_RESOURCE_PARTITION_BY_FIELD;

  const handleChange = (entries: string[]) => {
    setError(undefined);
    onChange(entries.length > 0 ? entries : undefined);
  };

  const handleCreate = (entry: string) => {
    const column = getPartitionColumn(entry);
    const isKnownColumn =
      columns.length === 0 || columns.some((candidate) => candidate.name === column);
    if (isPartitionField && (column === undefined || !isKnownColumn)) {
      setError("Use a column or transform(column), such as day(created_at)");
      return;
    }
    handleChange([...selected, entry]);
  };

  return (
    <Box fillWidth minWidth={0}>
      <MultiSelectInput
        options={options}
        value={selected}
        onChange={handleChange}
        onCreate={handleCreate}
        error={error}
        label={isCompact ? undefined : label}
        labelTooltip={isCompact ? undefined : field.help || undefined}
        variant={isCompact ? MultiSelectInputVariant.BASE : MultiSelectInputVariant.TERTIARY}
        placeholder="None"
        isDisabled={isDisabled}
        isReadOnly={isReadOnly}
        ariaLabel={ariaLabel}
        isSearchable
        isClearable
        fillWidth
      />
    </Box>
  );
};

export default PipelineResourceSinkFieldList;
