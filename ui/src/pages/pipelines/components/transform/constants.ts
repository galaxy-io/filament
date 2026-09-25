import {
  ArrowLineDownIcon,
  ArrowLineUpIcon,
  ArrowsDownUpIcon,
  ArrowsLeftRightIcon,
  ArrowsMergeIcon,
  CalendarBlankIcon,
  CalendarDotIcon,
  CalendarDotsIcon,
  CalendarIcon,
  CircleDashedIcon,
  DivideIcon,
  EqualsIcon,
  EraserIcon,
  GreaterThanIcon,
  GreaterThanOrEqualIcon,
  HashIcon,
  IntersectIcon,
  LessThanIcon,
  LessThanOrEqualIcon,
  LinkSimpleIcon,
  MagnifyingGlassIcon,
  MinusIcon,
  NotEqualsIcon,
  PencilLineIcon,
  type Icon as PhosphorIcon,
  PlusIcon,
  PlusMinusIcon,
  ProhibitIcon,
  RulerIcon,
  ScissorsIcon,
  SelectionIcon,
  TextAaIcon,
  TextAUnderlineIcon,
  TextTIcon,
  ToggleLeftIcon,
  UniteIcon,
  XIcon,
} from "@phosphor-icons/react";

import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import {
  type TransformAction,
  TransformActionKind,
  TransformConditionJoin,
  TransformLiteralKind,
  TransformStepKind,
} from "@/pages/pipelines/components/transform/types";

export const TRANSFORM_HANDLE = 28;
export const TRANSFORM_GUTTER = 36;
export const TRANSFORM_COND_GUTTER = 56;
export const TRANSFORM_ACTION = 28;
export const TRANSFORM_ADD_ROW = 24;
export const TRANSFORM_GAP = 8;
export const TRANSFORM_BOX_PAD = 6;
export const TRANSFORM_HEADER_PADDING_X = 12;
export const TRANSFORM_HEADER_PADDING_Y = 10;
export const TRANSFORM_BODY_INSET = TRANSFORM_HEADER_PADDING_X + TRANSFORM_HANDLE + TRANSFORM_GAP;
export const TRANSFORM_CONDITION_STACK_WIDTH = 300;
export const TRANSFORM_VALIDATION_DEBOUNCE_MS = 250;
export const TRANSFORM_SELECT_SEARCH_THRESHOLD = 8;
export const TRANSFORM_DRAG_DISTANCE = 4;
export const TRANSFORM_SELECT_ERROR_MARK = " ";
export const TRANSFORM_NEW_COLUMN_SUFFIX = "_v2";
export const TRANSFORM_RAW_STEP_LABEL = "Defined outside the builder";

export const TRANSFORM_LITERAL_KINDS = Object.values(TransformLiteralKind);

export const TRANSFORM_LITERAL_KIND_TO_LABEL_MAP: Record<TransformLiteralKind, string> = {
  [TransformLiteralKind.STRING]: "Set text",
  [TransformLiteralKind.NUMBER]: "Set number",
  [TransformLiteralKind.BOOLEAN]: "Set true/false",
};
export const TRANSFORM_LITERAL_KIND_TO_PLACEHOLDER_MAP: Record<TransformLiteralKind, string> = {
  [TransformLiteralKind.STRING]: "Enter text",
  [TransformLiteralKind.NUMBER]: "Enter a number",
  [TransformLiteralKind.BOOLEAN]: "Choose true or false",
};
export const TRANSFORM_LITERAL_KIND_TO_ICON_MAP: Record<TransformLiteralKind, PhosphorIcon> = {
  [TransformLiteralKind.STRING]: TextTIcon,
  [TransformLiteralKind.NUMBER]: HashIcon,
  [TransformLiteralKind.BOOLEAN]: ToggleLeftIcon,
};

export const TRANSFORM_STEP_KIND_TO_ACTION_MAP: Record<
  TransformStepKind.RENAME | TransformStepKind.DROP,
  TransformAction
> = {
  [TransformStepKind.RENAME]: { kind: TransformActionKind.RENAME },
  [TransformStepKind.DROP]: { kind: TransformActionKind.DROP },
};

export const TRANSFORM_CONDITION_JOIN_TO_OPPOSITE_MAP: Record<
  TransformConditionJoin,
  TransformConditionJoin
> = {
  [TransformConditionJoin.AND]: TransformConditionJoin.OR,
  [TransformConditionJoin.OR]: TransformConditionJoin.AND,
};

export const TRANSFORM_FUNCTION_TO_ICON_MAP = new Map<TransformFunction["name"], PhosphorIcon>([
  ["add", PlusIcon],
  ["sub", MinusIcon],
  ["mul", XIcon],
  ["div", DivideIcon],
  ["eq", EqualsIcon],
  ["neq", NotEqualsIcon],
  ["gt", GreaterThanIcon],
  ["gte", GreaterThanOrEqualIcon],
  ["lt", LessThanIcon],
  ["lte", LessThanOrEqualIcon],
  ["to_date", CalendarBlankIcon],
  ["lower", TextAaIcon],
  ["upper", TextAUnderlineIcon],
  ["trim", EraserIcon],
  ["length", RulerIcon],
  ["replace", ArrowsLeftRightIcon],
  ["substring", ScissorsIcon],
  ["concat", LinkSimpleIcon],
  ["regex_match", MagnifyingGlassIcon],
  ["regex_extract", SelectionIcon],
  ["regex_replace", PencilLineIcon],
  ["abs", PlusMinusIcon],
  ["floor", ArrowLineDownIcon],
  ["ceil", ArrowLineUpIcon],
  ["round", ArrowsDownUpIcon],
  ["and", IntersectIcon],
  ["or", UniteIcon],
  ["not", ProhibitIcon],
  ["is_null", CircleDashedIcon],
  ["coalesce", ArrowsMergeIcon],
  ["year", CalendarIcon],
  ["month", CalendarDotsIcon],
  ["day", CalendarDotIcon],
]);

export const TRANSFORM_INTEGER_TYPES = new Set(["int16", "int32", "int64"]);
export const TRANSFORM_NUMBER_TYPES = new Set([
  ...TRANSFORM_INTEGER_TYPES,
  "float32",
  "float64",
  "decimal",
]);
