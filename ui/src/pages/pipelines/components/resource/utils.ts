import { create } from "@bufbuild/protobuf";

import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import {
  type ResourceColumnSelection,
  ResourceColumnSelectionSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import {
  READ_MODE_TO_LABEL_MAP,
  WRITE_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";
import { READ_MODE_TO_WRITE_MODES_MAP } from "@/pages/pipelines/components/resource/constants";
import {
  type PipelineResourceStatus,
  PipelineResourceStatusField,
} from "@/pages/pipelines/components/resource/types";

import { createEnumSelectOptions } from "@/utils/select";

export const getReadModeSelectOptions = (modes: ReadMode[], selected: ReadMode): SelectOption[] =>
  createEnumSelectOptions(
    selected === ReadMode.UNSPECIFIED || modes.includes(selected) ? modes : [...modes, selected],
    READ_MODE_TO_LABEL_MAP,
  );

export const getWriteModeSelectOptions = (
  modes: WriteMode[],
  selected: WriteMode,
): SelectOption[] =>
  createEnumSelectOptions(
    selected === WriteMode.UNSPECIFIED || modes.includes(selected) ? modes : [...modes, selected],
    WRITE_MODE_TO_LABEL_MAP,
  );

export const getCursorSelectOptions = (columns: ResourceColumn[]): SelectOption[] =>
  columns.map((column) => ({ id: column.name, label: column.name }));

export const getSharedModes = <TMode>(sets: TMode[][]): TMode[] =>
  sets.length
    ? sets
        .slice(1)
        .reduce((common, modes) => common.filter((mode) => modes.includes(mode)), sets[0])
    : [];

export const getCompatibleWriteModes = (writeModes: WriteMode[], readModes: ReadMode[]) =>
  writeModes.filter((mode) =>
    readModes.every((readMode) => READ_MODE_TO_WRITE_MODES_MAP[readMode].includes(mode)),
  );

export const getCursorOptions = (columns: ResourceColumn[]): ResourceColumn[] =>
  columns
    .filter((column) => column.isCursorEligible)
    .sort((left, right) => {
      if (left.recommendationRank === right.recommendationRank) return 0;
      if (left.recommendationRank === 0) return 1;
      if (right.recommendationRank === 0) return -1;
      return left.recommendationRank - right.recommendationRank;
    });

export const getRecommendedCursor = (columns: ResourceColumn[]): ResourceColumn["name"] =>
  columns.find((column) => column.isCursorRecommended)?.name ?? "";

export const getDefaultCursor = (columns: ResourceColumn[]): ResourceColumn["name"] => {
  const recommended = getRecommendedCursor(columns);
  if (recommended !== "") return recommended;
  const eligible = getCursorOptions(columns);
  return eligible.length === 1 ? eligible[0].name : "";
};

export const getLockedColumnNames = ({
  primaryKey,
  cursor,
  isCursorLocked,
}: {
  primaryKey: ResourceColumn["name"][];
  cursor: ResourceColumn["name"];
  isCursorLocked: boolean;
}): ResourceColumn["name"][] => (isCursorLocked && cursor ? [...primaryKey, cursor] : primaryKey);

export const getEffectiveColumnSelection = ({
  stored,
  columns,
  locked,
}: {
  stored: ResourceColumn["name"][] | undefined;
  columns: ResourceColumn[] | undefined;
  locked: ResourceColumn["name"][];
}): ResourceColumn["name"][] => {
  const known = columns?.map((column) => column.name);
  if (stored === undefined) return known ?? [];
  const kept = known ? stored.filter((name) => known.includes(name)) : stored;
  const order = known ?? [...kept, ...locked];
  return order.filter((name) => kept.includes(name) || locked.includes(name));
};

export const createResourceColumnSelections = ({
  resource,
  selected,
  columns,
}: {
  resource: Resource["name"];
  selected: ResourceColumn["name"][];
  columns: ResourceColumn[];
}): ResourceColumnSelection[] =>
  selected.length === 0 || selected.length === columns.length
    ? []
    : [create(ResourceColumnSelectionSchema, { resource, columns: selected })];

export const addColumnToSelections = (
  selections: ResourceColumnSelection[],
  resource: Resource["name"],
  column: ResourceColumn["name"],
): ResourceColumnSelection[] =>
  column === ""
    ? selections
    : selections.map((selection) =>
        selection.resource === resource && !selection.columns.includes(column)
          ? create(ResourceColumnSelectionSchema, {
              resource,
              columns: [...selection.columns, column],
            })
          : selection,
      );

export const getDefaultReadMode = ({
  columns,
  readModeOptions,
  managedIncremental,
  fallback,
}: {
  columns: ResourceColumn[];
  readModeOptions: ReadMode[];
  managedIncremental: boolean;
  fallback: ReadMode;
}): ReadMode =>
  (managedIncremental || getRecommendedCursor(columns) !== "") &&
  readModeOptions.includes(ReadMode.INCREMENTAL)
    ? ReadMode.INCREMENTAL
    : fallback;

interface PipelineResourceStatusInput {
  resource: Resource["name"];
  readMode: ReadMode;
  readModeOptions: ReadMode[];
  cursorField: ResourceColumn["name"];
  cursorOptions: ResourceColumn[];
  isCursorKnown: boolean;
  managedIncremental: boolean;
  hasPrimaryKey: boolean;
  needsPrimaryKey: boolean;
}

export const getPipelineResourceStatus = ({
  resource,
  readMode,
  readModeOptions,
  cursorField,
  cursorOptions,
  isCursorKnown,
  managedIncremental,
  hasPrimaryKey,
  needsPrimaryKey,
}: PipelineResourceStatusInput): PipelineResourceStatus | undefined => {
  if (!readModeOptions.includes(readMode)) {
    return {
      resource,
      field: PipelineResourceStatusField.READ_MODE,
      message: `${resource} does not support the selected read mode`,
      isBlocking: true,
    };
  }
  if (readMode === ReadMode.INCREMENTAL && !managedIncremental && !cursorField) {
    if (cursorOptions.length) {
      return {
        resource,
        field: PipelineResourceStatusField.CURSOR,
        message: `Incremental reads require a cursor column for ${resource}`,
        isBlocking: true,
      };
    }
    if (isCursorKnown) {
      return {
        resource,
        field: PipelineResourceStatusField.READ_MODE,
        message: `${resource} has no usable cursor column; read it in full instead`,
        isBlocking: true,
      };
    }
  }
  if (needsPrimaryKey && !hasPrimaryKey) {
    return {
      resource,
      field: PipelineResourceStatusField.WRITE_MODE,
      message: `${resource} has no primary key and will use append; repeated rows are retained`,
      isBlocking: false,
    };
  }
  return undefined;
};
