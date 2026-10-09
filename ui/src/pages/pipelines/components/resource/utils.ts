import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";

import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

import {
  READ_MODE_TO_LABEL_MAP,
  WRITE_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";
import {
  type PipelineResourceStatus,
  PipelineResourceStatusField,
} from "@/pages/pipelines/components/resource/types";

export const getReadModeSelectOptions = (modes: ReadMode[]): SelectOption[] =>
  modes.map((mode) => ({ id: String(mode), label: READ_MODE_TO_LABEL_MAP[mode] }));

export const getWriteModeSelectOptions = (modes: WriteMode[]): SelectOption[] =>
  modes.map((mode) => ({ id: String(mode), label: WRITE_MODE_TO_LABEL_MAP[mode] }));

export const getCursorSelectOptions = (columns: ResourceColumn[]): SelectOption[] =>
  columns.map((column) => ({ id: column.name, label: column.name }));

export const getCompatibleWriteModes = (
  writeModes: WriteMode[],
  hasIncrementalRead: boolean,
): WriteMode[] => writeModes.filter((mode) => !hasIncrementalRead || mode !== WriteMode.REPLACE);

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
