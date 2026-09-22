import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import {
  TRANSFORM_LITERAL_KIND_TO_ICON_MAP,
  TRANSFORM_LITERAL_KIND_TO_PLACEHOLDER_MAP,
  TRANSFORM_LITERAL_KINDS,
  TRANSFORM_SELECT_ERROR_MARK,
  TRANSFORM_SELECT_SEARCH_THRESHOLD,
} from "@/pages/pipelines/components/transform/constants";
import {
  getTransformAcceptedColumns,
  getTransformLiteralKind,
} from "@/pages/pipelines/components/transform/grammar/catalog";
import {
  createTransformColumnExpr,
  TRANSFORM_EMPTY_EXPR,
} from "@/pages/pipelines/components/transform/grammar/chain";
import PipelineTransformFieldsLiteral from "@/pages/pipelines/components/transform/PipelineTransformFieldsLiteral";
import PipelineTransformFieldsOptionIcon from "@/pages/pipelines/components/transform/PipelineTransformFieldsOptionIcon";
import { usePipelineTransformFieldsEditor } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import {
  TransformExprKind,
  type TransformLeafExpr,
  TransformLiteralKind,
} from "@/pages/pipelines/components/transform/types";
import {
  createTransformBooleanOptions,
  createTransformLiteral,
  filterTransformOptions,
} from "@/pages/pipelines/components/transform/utils";

const COLUMN_OPTION_PREFIX = "column:";
const APPLY_FUNCTION_OPTION_ID = "apply-function";
const NO_VALUE_FORM_OPTION_ID = "no-value-form";
const NO_TYPES: string[] = [];
const BOOLEAN_OPTIONS = createTransformBooleanOptions();

interface PipelineTransformFieldsLeafProps {
  expr: TransformLeafExpr;
  onChange: (expr: TransformLeafExpr) => void;
  placeholder?: string;
  logicalTypes?: string[];
  isLiteralOnly?: boolean;
  isColumnOnly?: boolean;
  isOptional?: boolean;
  onApplyFunction?: () => void;
  isError?: boolean;
}

const PipelineTransformFieldsLeaf = ({
  expr,
  onChange,
  placeholder,
  logicalTypes = NO_TYPES,
  isLiteralOnly = false,
  isColumnOnly = false,
  isOptional = false,
  onApplyFunction,
  isError = false,
}: PipelineTransformFieldsLeafProps) => {
  const { columns, isDisabled } = usePipelineTransformFieldsEditor();
  const literalKind = getTransformLiteralKind(logicalTypes);

  if (isLiteralOnly && logicalTypes.length > 0 && literalKind === undefined) {
    return (
      <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
        No value form for {logicalTypes.join(" / ")}
      </Text>
    );
  }
  const isTyping =
    expr.kind === TransformExprKind.LITERAL && expr.literalKind !== TransformLiteralKind.BOOLEAN;
  if (isLiteralOnly || isTyping) {
    const literal =
      expr.kind === TransformExprKind.LITERAL
        ? expr
        : createTransformLiteral(literalKind ?? TransformLiteralKind.STRING);
    const canClear = isLiteralOnly ? isOptional && expr.kind !== TransformExprKind.EMPTY : true;
    return (
      <PipelineTransformFieldsLiteral
        expr={literal}
        logicalTypes={logicalTypes}
        placeholder={isOptional ? "Optional" : isLiteralOnly ? placeholder : "Enter value"}
        isError={isError}
        onChange={onChange}
        onClear={canClear ? () => onChange(TRANSFORM_EMPTY_EXPR) : undefined}
      />
    );
  }

  const literalKinds = isColumnOnly
    ? []
    : literalKind !== undefined
      ? [literalKind]
      : logicalTypes.length > 0
        ? []
        : TRANSFORM_LITERAL_KINDS;
  const literalOptions: SelectInputOption[] = literalKinds.flatMap((kind) =>
    kind === TransformLiteralKind.BOOLEAN
      ? BOOLEAN_OPTIONS
      : [
          {
            id: kind,
            label:
              logicalTypes.length > 0
                ? "Enter a value"
                : TRANSFORM_LITERAL_KIND_TO_PLACEHOLDER_MAP[kind],
            value: createTransformLiteral(kind),
            icon: (
              <PipelineTransformFieldsOptionIcon icon={TRANSFORM_LITERAL_KIND_TO_ICON_MAP[kind]} />
            ),
          },
        ],
  );
  const selectedColumn = expr.kind === TransformExprKind.COLUMN ? expr.name : undefined;
  const accepted = getTransformAcceptedColumns(columns, logicalTypes).map((column) => column.name);
  const columnNames =
    selectedColumn !== undefined && !accepted.includes(selectedColumn)
      ? [...accepted, selectedColumn]
      : accepted;
  const columnOptions: SelectInputOption[] = columnNames.map((name) => ({
    id: `${COLUMN_OPTION_PREFIX}${name}`,
    label: name,
    value: createTransformColumnExpr(name),
  }));
  const hasNoValueForm =
    !isColumnOnly && logicalTypes.length > 0 && literalKinds.length === 0 && accepted.length === 0;
  const canApplyFunction = onApplyFunction !== undefined && expr.kind !== TransformExprKind.EMPTY;
  const options: SelectInputOption[] = [
    ...literalOptions,
    ...(hasNoValueForm
      ? [
          {
            id: NO_VALUE_FORM_OPTION_ID,
            label: `No value form for ${logicalTypes.join(" / ")}`,
            value: null,
            variant: TextVariant.TERTIARY,
          },
        ]
      : []),
    ...columnOptions,
    ...(canApplyFunction
      ? [{ id: APPLY_FUNCTION_OPTION_ID, label: "Apply a function to this…", value: null }]
      : []),
  ];
  const selectedId =
    expr.kind === TransformExprKind.COLUMN
      ? `${COLUMN_OPTION_PREFIX}${expr.name}`
      : expr.kind === TransformExprKind.LITERAL
        ? (expr.value ?? "")
        : "";

  return (
    <SelectInput
      options={options}
      value={options.find((option) => option.id === selectedId) ?? null}
      onChange={(option) => {
        if (option.id === APPLY_FUNCTION_OPTION_ID) onApplyFunction?.();
        else if (option.id !== NO_VALUE_FORM_OPTION_ID) onChange(option.value as TransformLeafExpr);
      }}
      onSearch={
        literalOptions.length + columnOptions.length > TRANSFORM_SELECT_SEARCH_THRESHOLD
          ? filterTransformOptions
          : undefined
      }
      onReset={() => onChange(TRANSFORM_EMPTY_EXPR)}
      placeholder={placeholder ?? (isOptional ? "Optional" : "Choose a column or value")}
      variant={InputVariant.TERTIARY}
      size={InputSize.MEDIUM}
      error={isError ? TRANSFORM_SELECT_ERROR_MARK : undefined}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export default PipelineTransformFieldsLeaf;
