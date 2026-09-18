import { type CSSProperties, useEffect, useMemo, useRef, useState } from "react";

import { PlusIcon, XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Input, { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import {
  createEmptyTransformExpression,
  TRANSFORM_COMPARISON_FUNCTION_WIDTH,
  TRANSFORM_COMPARISON_SYMBOLS,
  TRANSFORM_FLOW_SOURCE_WIDTH,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/constants";
import type {
  TransformColumn,
  TransformExpression,
  TransformExpressionSource,
  TransformFunctionCall,
  TransformLiteralKind,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";
import {
  getCompatibleTransformFunctions,
  getTransformArgumentSpec,
  getTransformExpressionInfo,
  getTransformLiteralKind,
  isIntegerLogicalType,
  isTransformExpressionEmpty,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/utils";

const COLUMN_OPTION_PREFIX = "column:";
const LITERAL_OPTION_PREFIX = "literal:";
const TRUE_OPTION_ID = "literal:boolean:true";
const FALSE_OPTION_ID = "literal:boolean:false";
const EMPTY_LOGICAL_TYPES: string[] = [];
const INLINE_CONTROL_GROUP_STYLE: CSSProperties = {
  border: 0,
  margin: 0,
  minWidth: 0,
  padding: 0,
  width: "100%",
};

const capitalize = (name: string) => name.charAt(0).toUpperCase() + name.slice(1);

const emptyLiteralSource = (
  literalKind: TransformLiteralKind,
): Extract<TransformExpressionSource, { kind: "literal" }> => ({
  kind: "literal",
  literalKind,
  value: null,
});

const describeSource = (source: TransformExpressionSource): string => {
  switch (source.kind) {
    case "column":
      return source.column;
    case "literal":
      return source.value === null ? "value" : JSON.stringify(source.value);
    case "empty":
      return "Choose a column";
  }
};

const getArgumentLabel = (fn: TransformFunction, index: number, fallback: string): string => {
  if (["eq", "neq", "gt", "gte", "lt", "lte"].includes(fn.name) && index === 1) {
    return "Compare with";
  }
  if (fn.name === "concat") return "Append";
  if (fn.name === "replace" && index === 1) return "Find";
  if ((fn.name === "replace" || fn.name === "regex_replace") && index === 2) {
    return "Replace with";
  }
  if (fn.name === "substring" && index === 1) return "Start position";
  return fallback;
};

const getArgumentPlaceholder = (label: string, isOptional: boolean): string | undefined => {
  if (isOptional) return "Optional";
  if (label === "Compare with") return "Choose a column or value";
  return undefined;
};

const isUnfilledDraftError = (error: string): boolean =>
  error === "Choose a column or value." ||
  error === "Enter a value." ||
  error === "Choose a function." ||
  error.endsWith("needs another argument.") ||
  /: (Choose a column or value|Enter a value|Choose a function)\.$/.test(error);

/** The DLS trailing-input action has no aria-label prop, so scope its existing button here. */
const useTrailingActionLabel = (label: string | undefined) => {
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!label) return;
    const action = inputRef.current?.parentElement?.querySelector<HTMLButtonElement>("button");
    if (!action) return;
    action.setAttribute("aria-label", label);
    return () => action.removeAttribute("aria-label");
  }, [label]);

  return inputRef;
};

interface TransformLiteralFieldProps {
  source: Extract<TransformExpressionSource, { kind: "literal" }>;
  label?: string;
  placeholder?: string;
  logicalTypes: string[];
  onChange: (source: TransformExpressionSource) => void;
  onClear?: () => void;
  clearActionLabel?: string;
  size: InputSize;
  isDisabled: boolean;
}

const TransformLiteralField = ({
  source,
  label,
  placeholder,
  logicalTypes,
  onChange,
  onClear,
  clearActionLabel,
  size,
  isDisabled,
}: TransformLiteralFieldProps) => {
  const inputRef = useTrailingActionLabel(onClear ? clearActionLabel : undefined);
  const integerOnly =
    logicalTypes.length > 0 && logicalTypes.every((type) => isIntegerLogicalType(type));
  const numericError =
    source.literalKind === "number" &&
    typeof source.value === "number" &&
    integerOnly &&
    !Number.isSafeInteger(source.value)
      ? "Enter a whole number."
      : undefined;

  if (source.literalKind === "boolean") {
    const options: SelectInputOption[] = [
      { id: TRUE_OPTION_ID, label: "true", value: true },
      { id: FALSE_OPTION_ID, label: "false", value: false },
    ];
    return (
      <SelectInput
        label={label}
        options={options}
        value={source.value === true ? options[0] : source.value === false ? options[1] : null}
        onChange={(option) => onChange({ ...source, value: option.value as boolean })}
        placeholder={placeholder ?? "Choose true or false"}
        variant={InputVariant.TERTIARY}
        size={size}
        isDisabled={isDisabled}
        fillWidth
      />
    );
  }

  if (source.literalKind === "number") {
    return (
      <TransformNumberLiteralInput
        source={source}
        label={label}
        placeholder={placeholder}
        integerOnly={integerOnly}
        error={numericError}
        onChange={onChange}
        onClear={onClear}
        clearActionLabel={clearActionLabel}
        size={size}
        isDisabled={isDisabled}
      />
    );
  }

  return (
    <TextInput
      ref={inputRef}
      label={label}
      value={typeof source.value === "string" ? source.value : ""}
      onChange={(value) => onChange({ ...source, value })}
      placeholder={placeholder}
      trailing={onClear ? { icon: XIcon, onClick: onClear } : undefined}
      variant={InputVariant.TERTIARY}
      size={size}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

interface TransformNumberLiteralInputProps {
  source: Extract<TransformExpressionSource, { kind: "literal" }>;
  label?: string;
  placeholder?: string;
  integerOnly: boolean;
  error?: string;
  onChange: (source: TransformExpressionSource) => void;
  onClear?: () => void;
  clearActionLabel?: string;
  size: InputSize;
  isDisabled: boolean;
}

/** DLS NumberInput maps an empty DOM value to zero; keep the raw text so empty stays unset. */
const TransformNumberLiteralInput = ({
  source,
  label,
  placeholder,
  integerOnly,
  error,
  onChange,
  onClear,
  clearActionLabel,
  size,
  isDisabled,
}: TransformNumberLiteralInputProps) => {
  const inputRef = useTrailingActionLabel(onClear ? clearActionLabel : undefined);
  const [raw, setRaw] = useState(source.value === null ? "" : String(source.value));
  const emittedValue = useRef(source.value);
  const parsedRaw = raw.trim() === "" ? null : Number(raw);
  const rawError =
    raw !== "" && (raw.trim() === "" || !Number.isFinite(parsedRaw))
      ? "Enter a valid number."
      : parsedRaw !== null && integerOnly && !Number.isSafeInteger(parsedRaw)
        ? "Enter a whole number."
        : undefined;

  useEffect(() => {
    if (source.value === emittedValue.current) return;
    emittedValue.current = source.value;
    setRaw(source.value === null ? "" : String(source.value));
  }, [source.value]);

  return (
    <Input<string>
      ref={inputRef}
      type="text"
      parse={(value) => value}
      label={label}
      value={raw}
      onChange={(value) => {
        setRaw(value);
        if (value === "") {
          emittedValue.current = null;
          onChange({ ...source, value: null });
          return;
        }
        const parsed = value.trim() === "" ? Number.NaN : Number(value);
        const next = Number.isFinite(parsed) ? parsed : null;
        emittedValue.current = next;
        onChange({ ...source, value: next });
      }}
      placeholder={placeholder}
      error={error ?? rawError}
      trailing={onClear ? { icon: XIcon, onClick: onClear } : undefined}
      variant={InputVariant.TERTIARY}
      size={size}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

export interface TransformExpressionSourceFieldProps {
  source: TransformExpressionSource;
  columns: TransformColumn[];
  onChange: (source: TransformExpressionSource) => void;
  label?: string;
  placeholder?: string;
  logicalTypes?: string[];
  isLiteralOnly?: boolean;
  isColumnOnly?: boolean;
  isOptional?: boolean;
  clearActionLabel?: string;
  size?: InputSize;
  isDisabled?: boolean;
}

/** A source leaf: a column, or one of the literal forms the compiler can actually coerce. */
export const TransformExpressionSourceField = ({
  source,
  columns,
  onChange,
  label,
  placeholder,
  logicalTypes = EMPTY_LOGICAL_TYPES,
  isLiteralOnly = false,
  isColumnOnly = false,
  isOptional = false,
  clearActionLabel,
  size = InputSize.MEDIUM,
  isDisabled = false,
}: TransformExpressionSourceFieldProps) => {
  const declaredLiteralKind = getTransformLiteralKind(logicalTypes);
  const literalKinds = useMemo<TransformLiteralKind[]>(() => {
    if (isColumnOnly) return [];
    if (declaredLiteralKind) return [declaredLiteralKind];
    if (logicalTypes.length > 0) return [];
    return ["string", "number", "boolean"];
  }, [declaredLiteralKind, isColumnOnly, logicalTypes]);
  const columnOptions = useMemo<SelectInputOption[]>(
    () =>
      columns
        .filter((column) => logicalTypes.length === 0 || logicalTypes.includes(column.logicalType))
        .map((column) => ({
          id: `${COLUMN_OPTION_PREFIX}${column.name}`,
          label: column.name,
          value: { kind: "column", column: column.name } satisfies TransformExpressionSource,
        })),
    [columns, logicalTypes],
  );
  const literalOptions = useMemo<SelectInputOption[]>(
    () =>
      literalKinds.flatMap((literalKind) => {
        if (literalKind === "boolean") {
          return [
            {
              id: TRUE_OPTION_ID,
              label: "true",
              value: {
                kind: "literal",
                literalKind,
                value: true,
              } satisfies TransformExpressionSource,
            },
            {
              id: FALSE_OPTION_ID,
              label: "false",
              value: {
                kind: "literal",
                literalKind,
                value: false,
              } satisfies TransformExpressionSource,
            },
          ];
        }
        return [
          {
            id: `${LITERAL_OPTION_PREFIX}${literalKind}`,
            label:
              logicalTypes.length > 0
                ? "Enter a value"
                : literalKind === "string"
                  ? "Enter text"
                  : "Enter a number",
            value: emptyLiteralSource(literalKind),
          },
        ];
      }),
    [literalKinds, logicalTypes.length],
  );
  const options = useMemo(
    () => [...literalOptions, ...columnOptions],
    [columnOptions, literalOptions],
  );

  useEffect(() => {
    if (
      !isDisabled &&
      !isLiteralOnly &&
      !isOptional &&
      source.kind === "empty" &&
      options.length === 1
    ) {
      onChange(options[0].value as TransformExpressionSource);
    }
  }, [isDisabled, isLiteralOnly, isOptional, onChange, options, source.kind]);

  if (isLiteralOnly && logicalTypes.length > 0 && !declaredLiteralKind) {
    return (
      <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
        {label ?? "Value"} does not support a literal for this type.
      </Text>
    );
  }

  if (isLiteralOnly) {
    const literalKind =
      source.kind === "literal" ? source.literalKind : (declaredLiteralKind ?? "string");
    const literal = source.kind === "literal" ? source : emptyLiteralSource(literalKind);
    return (
      <TransformLiteralField
        source={literal}
        label={label}
        placeholder={isOptional ? "Optional" : placeholder}
        logicalTypes={logicalTypes}
        onChange={onChange}
        onClear={
          isOptional && source.kind !== "empty" ? () => onChange({ kind: "empty" }) : undefined
        }
        clearActionLabel={clearActionLabel ?? `Clear ${label ?? "optional value"}`}
        size={size}
        isDisabled={isDisabled}
      />
    );
  }

  if (source.kind === "literal" && source.literalKind !== "boolean") {
    return (
      <TransformLiteralField
        source={source}
        label={label}
        placeholder={isOptional ? "Optional" : placeholder}
        logicalTypes={logicalTypes}
        onChange={onChange}
        onClear={() => onChange({ kind: "empty" })}
        clearActionLabel={clearActionLabel ?? `Change ${label ?? "value"} source`}
        size={size}
        isDisabled={isDisabled}
      />
    );
  }

  const selectedId =
    source.kind === "column"
      ? `${COLUMN_OPTION_PREFIX}${source.column}`
      : source.kind === "literal" && source.literalKind === "boolean"
        ? source.value === true
          ? TRUE_OPTION_ID
          : source.value === false
            ? FALSE_OPTION_ID
            : ""
        : "";

  return (
    <SelectInput
      label={label}
      options={options}
      value={options.find((option) => option.id === selectedId) ?? null}
      onChange={(option) => onChange(option.value as TransformExpressionSource)}
      onSearch={(term, candidates) =>
        candidates.filter((option) => option.label.toLowerCase().includes(term.toLowerCase()))
      }
      variant={InputVariant.TERTIARY}
      placeholder={placeholder ?? (isOptional ? "Optional" : "Choose a column or value")}
      size={size}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

const initialArguments = (fn: TransformFunction): TransformExpression[] => {
  const args: TransformExpression[] = [];
  for (let index = 1; index < fn.args.length; index++) {
    const spec = fn.args[index];
    if (spec.isVariadic) {
      args.push(createEmptyTransformExpression());
      break;
    }
    args.push(createEmptyTransformExpression());
  }
  return args;
};

const minimumSlotCount = (fn: TransformFunction): number => {
  const variadic = fn.args[fn.args.length - 1]?.isVariadic === true;
  const fixed = Math.max(0, fn.args.length - 1 - (variadic ? 1 : 0));
  return fixed + (variadic && fn.args.length > 1 ? 1 : 0);
};

/** True when a visible number field already owns the actionable validation message. */
const hasInlineNumericValidationError = (
  expression: TransformExpression,
  columns: TransformColumn[],
  functionsByName: Map<string, TransformFunction>,
): boolean =>
  expression.calls.some((call, callIndex) => {
    const fn = functionsByName.get(call.name);
    if (!fn) return false;
    const inputInfo = getTransformExpressionInfo(
      { ...expression, calls: expression.calls.slice(0, callIndex) },
      columns,
      functionsByName,
    );

    return call.args.some((argument, argumentIndex) => {
      const spec = getTransformArgumentSpec(fn, argumentIndex + 1);
      const expectedTypes = spec?.isLiteral
        ? spec.logicalTypes
        : fn.sameType && inputInfo.complete && !inputInfo.isLiteral
          ? [inputInfo.type]
          : (spec?.logicalTypes ?? []);
      const hasIntegerError =
        argument.source.kind === "literal" &&
        argument.source.literalKind === "number" &&
        typeof argument.source.value === "number" &&
        expectedTypes.length > 0 &&
        expectedTypes.every(isIntegerLogicalType) &&
        !Number.isSafeInteger(argument.source.value);
      return hasIntegerError || hasInlineNumericValidationError(argument, columns, functionsByName);
    });
  });

interface TransformFunctionCallBlockProps {
  call: TransformFunctionCall;
  callIndex: number;
  inputExpression: TransformExpression;
  columns: TransformColumn[];
  functionsByName: Map<string, TransformFunction>;
  onChange: (call: TransformFunctionCall) => void;
  onRemove: () => void;
  isDisabled: boolean;
  depth: number;
  isColumnFlow: boolean;
  canRemove: boolean;
}

const TransformFunctionCallBlock = ({
  call,
  callIndex,
  inputExpression,
  columns,
  functionsByName,
  onChange,
  onRemove,
  isDisabled,
  depth,
  isColumnFlow,
  canRemove,
}: TransformFunctionCallBlockProps) => {
  const inputInfo = getTransformExpressionInfo(inputExpression, columns, functionsByName);
  const compatible = getCompatibleTransformFunctions(inputInfo, functionsByName);
  const selectedFunction = functionsByName.get(call.name);
  const available =
    selectedFunction && !compatible.some((candidate) => candidate.name === selectedFunction.name)
      ? [...compatible, selectedFunction]
      : compatible;
  const options: SelectInputOption[] = available.map((fn) => ({
    id: fn.name,
    label: fn.displayName || fn.name,
    value: fn.name,
  }));
  const fn = selectedFunction;
  const slots = fn ? Math.max(minimumSlotCount(fn), call.args.length) : call.args.length;
  const variadic = fn ? fn.args[fn.args.length - 1]?.isVariadic === true : false;
  const comparisonSymbol = fn ? TRANSFORM_COMPARISON_SYMBOLS.get(fn.name) : undefined;
  const isInlineComparison = isColumnFlow && comparisonSymbol !== undefined && slots === 1;
  const selected = options.find((option) => option.id === call.name) ?? null;
  const displayedSelection =
    isInlineComparison && selected ? { ...selected, label: comparisonSymbol } : selected;

  const handleSelect = (name: string) => {
    const next = functionsByName.get(name);
    if (!next) return;
    const keepComparisonOperand =
      TRANSFORM_COMPARISON_SYMBOLS.has(call.name) && TRANSFORM_COMPARISON_SYMBOLS.has(name);
    onChange({
      name,
      args: keepComparisonOperand ? call.args : initialArguments(next),
    });
  };

  const handleArgumentChange = (index: number, expression: TransformExpression) => {
    const args = Array.from(
      { length: Math.max(slots, index + 1) },
      (_, slot) => call.args[slot] ?? createEmptyTransformExpression(),
    );
    args[index] = expression;
    onChange({ ...call, args });
  };

  const first = fn?.args[0];
  const implicitLabel = first
    ? `${capitalize(first.name)}${first.isVariadic ? " 1" : ""}: ${
        callIndex === 0 ? describeSource(inputExpression.source) : "previous result"
      }`
    : "Input";

  const functionSelect = (
    <SelectInput
      label={!isColumnFlow && callIndex === 0 ? "Function" : undefined}
      options={options}
      value={displayedSelection}
      onChange={(option) => handleSelect(option.value as string)}
      onSearch={(term, candidates) =>
        candidates.filter((option) => option.label.toLowerCase().includes(term.toLowerCase()))
      }
      variant={InputVariant.TERTIARY}
      placeholder={inputInfo.complete ? "Choose a function" : "Complete the input first"}
      size={InputSize.MEDIUM}
      isDisabled={isDisabled || !inputInfo.complete}
      fillWidth
    />
  );

  const removeButton = canRemove ? (
    <Button
      icon={XIcon}
      variant={ButtonVariant.SECONDARY}
      size={ButtonSize.MEDIUM}
      onClick={onRemove}
      isDisabled={isDisabled}
      ariaLabel={`Remove ${fn?.displayName || call.name || "function"}`}
    />
  ) : isColumnFlow ? (
    <Button
      icon={XIcon}
      variant={ButtonVariant.SECONDARY}
      size={ButtonSize.MEDIUM}
      onClick={() => undefined}
      isDisabled
      ariaLabel="No function to remove"
    />
  ) : null;

  const argumentFields = (
    <>
      {Array.from({ length: slots }, (_, index) => {
        const spec = fn ? getTransformArgumentSpec(fn, index + 1) : undefined;
        const argument = call.args[index] ?? createEmptyTransformExpression();
        const expectedTypes = spec?.isLiteral
          ? spec.logicalTypes
          : fn?.sameType && inputInfo.complete && !inputInfo.isLiteral
            ? [inputInfo.type]
            : (spec?.logicalTypes ?? []);
        const isExtra = spec === undefined;
        const isVariadicSlot = spec?.isVariadic === true;
        const fallbackLabel = spec
          ? `${capitalize(spec.name)}${isVariadicSlot ? ` ${index + 2}` : ""}`
          : `Extra argument ${index + 2}`;
        const label = fn ? getArgumentLabel(fn, index + 1, fallbackLabel) : fallbackLabel;
        return (
          <FlexWrapper
            // biome-ignore lint/suspicious/noArrayIndexKey: argument positions are their grammar identity
            key={`${call.name || "call"}:${index}`}
            alignItems={AlignItems.END}
            gap={8}
            fillWidth
          >
            <FlexItem grow={1} minWidth={0}>
              <PipelineCanvasPanelResourceTransformExpression
                expression={argument}
                columns={columns}
                functionsByName={functionsByName}
                onChange={(expression) => handleArgumentChange(index, expression)}
                label={label}
                sourceLabel={label}
                sourcePlaceholder={getArgumentPlaceholder(label, spec?.isOptional === true)}
                logicalTypes={expectedTypes}
                isLiteralOnly={spec?.isLiteral}
                isColumnOnly={spec?.isColumn}
                isOptional={spec?.isOptional}
                allowFunctions={false}
                isColumnFlow={isColumnFlow}
                depth={depth + 1}
                isDisabled={isDisabled}
              />
            </FlexItem>
            {(isVariadicSlot || isExtra) && (
              <Button
                icon={XIcon}
                variant={ButtonVariant.SECONDARY}
                size={ButtonSize.MEDIUM}
                onClick={() =>
                  onChange({ ...call, args: call.args.filter((_, slot) => slot !== index) })
                }
                isDisabled={isDisabled}
                ariaLabel={`Remove ${label.toLowerCase()}`}
              />
            )}
          </FlexWrapper>
        );
      })}
      {variadic && fn && (
        <Button
          label={fn.name === "concat" ? "Add part" : "Add value"}
          icon={PlusIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          onClick={() =>
            onChange({ ...call, args: [...call.args, createEmptyTransformExpression()] })
          }
          isDisabled={isDisabled}
        />
      )}
    </>
  );

  if (isInlineComparison && fn) {
    const spec = getTransformArgumentSpec(fn, 1);
    const argument = call.args[0] ?? createEmptyTransformExpression();
    const expectedTypes =
      fn.sameType && inputInfo.complete && !inputInfo.isLiteral
        ? [inputInfo.type]
        : (spec?.logicalTypes ?? []);
    const inputLabel = describeSource(inputExpression.source);
    const inputOption: SelectInputOption = { id: inputLabel, label: inputLabel, value: inputLabel };
    const functionName = fn.displayName || fn.name;
    const functionAccessibleName =
      callIndex === 0
        ? `${functionName} function, left input ${inputLabel}`
        : `${functionName} function, left input is the preceding result`;

    return (
      <FlexWrapper alignItems={AlignItems.END} gap={8} fillWidth>
        <FlexItem basis={TRANSFORM_FLOW_SOURCE_WIDTH} shrink={0} minWidth={0}>
          {callIndex === 0 && (
            <SelectInput
              options={[inputOption]}
              value={inputOption}
              onChange={() => undefined}
              variant={InputVariant.TERTIARY}
              size={InputSize.MEDIUM}
              isDisabled
              fillWidth
            />
          )}
        </FlexItem>
        <FlexItem basis={TRANSFORM_COMPARISON_FUNCTION_WIDTH} shrink={0} minWidth={0}>
          <fieldset aria-label={functionAccessibleName} style={INLINE_CONTROL_GROUP_STYLE}>
            {functionSelect}
          </fieldset>
        </FlexItem>
        <FlexItem grow={1} basis={0} minWidth={0}>
          <fieldset
            aria-label={`${functionName} comparison value`}
            style={INLINE_CONTROL_GROUP_STYLE}
          >
            <PipelineCanvasPanelResourceTransformExpression
              expression={argument}
              columns={columns}
              functionsByName={functionsByName}
              onChange={(expression) => handleArgumentChange(0, expression)}
              sourcePlaceholder="Value"
              sourceClearActionLabel={`Change ${functionName} comparison value source`}
              logicalTypes={expectedTypes}
              isLiteralOnly={spec?.isLiteral}
              isColumnOnly={spec?.isColumn}
              isOptional={spec?.isOptional}
              allowFunctions={false}
              isColumnFlow
              depth={depth + 1}
              isDisabled={isDisabled}
            />
          </fieldset>
        </FlexItem>
        <FlexItem width={28} shrink={0}>
          {removeButton}
        </FlexItem>
      </FlexWrapper>
    );
  }

  if (isColumnFlow) {
    const inputLabel = describeSource(inputExpression.source);
    const inputOption: SelectInputOption = { id: inputLabel, label: inputLabel, value: inputLabel };
    return (
      <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
        <FlexWrapper alignItems={AlignItems.END} gap={8} fillWidth>
          <FlexItem basis={TRANSFORM_FLOW_SOURCE_WIDTH} shrink={0} minWidth={0}>
            {callIndex === 0 && (
              <SelectInput
                options={[inputOption]}
                value={inputOption}
                onChange={() => undefined}
                variant={InputVariant.TERTIARY}
                size={InputSize.MEDIUM}
                isDisabled
                fillWidth
              />
            )}
          </FlexItem>
          <FlexItem grow={1} basis={0} minWidth={0}>
            {functionSelect}
          </FlexItem>
          <FlexItem width={28} shrink={0}>
            {removeButton}
          </FlexItem>
        </FlexWrapper>
        <FlexWrapper alignItems={AlignItems.START} gap={8} fillWidth>
          <FlexItem basis={TRANSFORM_FLOW_SOURCE_WIDTH} shrink={0} minWidth={0} />
          <FlexItem grow={1} basis={0} minWidth={0}>
            <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
              {argumentFields}
            </FlexWrapper>
          </FlexItem>
          <FlexItem width={28} shrink={0} />
        </FlexWrapper>
      </FlexWrapper>
    );
  }

  const content = (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={8} fillWidth>
        <FlexItem grow={1} minWidth={0}>
          {functionSelect}
        </FlexItem>
        {removeButton}
      </FlexWrapper>
      {fn && (
        <>
          <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY}>
            {fn.description}
          </Text>
          {!isColumnFlow && (
            <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY} isMonospace>
              {implicitLabel}
            </Text>
          )}
        </>
      )}
      {argumentFields}
    </FlexWrapper>
  );

  return (
    <Widget variant={WidgetVariant.SECONDARY} noHover padding="12px" fillWidth>
      {content}
    </Widget>
  );
};

interface PipelineCanvasPanelResourceTransformExpressionProps {
  expression: TransformExpression;
  columns: TransformColumn[];
  functionsByName: Map<string, TransformFunction>;
  onChange: (expression: TransformExpression) => void;
  label?: string;
  sourceLabel?: string;
  sourcePlaceholder?: string;
  sourceClearActionLabel?: string;
  logicalTypes?: string[];
  isLiteralOnly?: boolean;
  isColumnOnly?: boolean;
  isOptional?: boolean;
  showSource?: boolean;
  allowFunctions?: boolean;
  isColumnFlow?: boolean;
  depth?: number;
  isDisabled?: boolean;
}

/** A recursively editable expression with a compact, scoped block per chained function. */
const PipelineCanvasPanelResourceTransformExpression = ({
  expression,
  columns,
  functionsByName,
  onChange,
  label,
  sourceLabel,
  sourcePlaceholder,
  sourceClearActionLabel,
  logicalTypes = EMPTY_LOGICAL_TYPES,
  isLiteralOnly = false,
  isColumnOnly = false,
  isOptional = false,
  showSource = true,
  allowFunctions = true,
  isColumnFlow = false,
  depth = 0,
  isDisabled = false,
}: PipelineCanvasPanelResourceTransformExpressionProps) => {
  const info = getTransformExpressionInfo(expression, columns, functionsByName);
  const hasInlineFieldValidationError = hasInlineNumericValidationError(
    expression,
    columns,
    functionsByName,
  );
  const handleCallChange = (index: number, call: TransformFunctionCall) => {
    onChange({
      ...expression,
      calls: expression.calls.map((candidate, slot) => (slot === index ? call : candidate)),
    });
  };
  const handleRemove = (index: number) => {
    onChange({ ...expression, calls: expression.calls.filter((_, slot) => slot !== index) });
  };
  const handleAdd = () => {
    onChange({ ...expression, calls: [...expression.calls, { name: "", args: [] }] });
  };
  const displayedCalls =
    isColumnFlow && allowFunctions && expression.calls.length === 0
      ? [{ name: "", args: [] }]
      : expression.calls;
  const hasIncompleteFunction =
    expression.calls.length > 0 && expression.calls[expression.calls.length - 1].name === "";
  const sourceField = (
    <TransformExpressionSourceField
      source={expression.source}
      columns={columns}
      onChange={(source) => onChange({ ...expression, source })}
      label={sourceLabel ?? label}
      placeholder={sourcePlaceholder}
      clearActionLabel={sourceClearActionLabel}
      logicalTypes={logicalTypes}
      isLiteralOnly={isLiteralOnly}
      isColumnOnly={isColumnOnly}
      isOptional={isOptional}
      size={depth === 0 ? InputSize.LARGE : InputSize.MEDIUM}
      isDisabled={isDisabled}
    />
  );

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
      {label && (!showSource || sourceLabel !== label) && (
        <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
          {label}
        </Text>
      )}
      {showSource && sourceField}
      {displayedCalls.map((call, index) => {
        const inputExpression = { ...expression, calls: expression.calls.slice(0, index) };
        const isInitialPicker = expression.calls.length === 0;
        return (
          <FlexWrapper
            // biome-ignore lint/suspicious/noArrayIndexKey: chain position is the function call's grammar identity
            key={`${call.name || "function"}:${index}`}
            direction={FlexDirection.COLUMN}
            gap={8}
            fillWidth
          >
            <TransformFunctionCallBlock
              call={call}
              callIndex={index}
              inputExpression={inputExpression}
              columns={columns}
              functionsByName={functionsByName}
              onChange={(next) =>
                isInitialPicker
                  ? onChange({ ...expression, calls: [next] })
                  : handleCallChange(index, next)
              }
              onRemove={() => handleRemove(index)}
              isDisabled={isDisabled}
              depth={depth}
              isColumnFlow={isColumnFlow}
              canRemove={!isInitialPicker}
            />
          </FlexWrapper>
        );
      })}
      {allowFunctions && !isLiteralOnly && !isColumnFlow && (
        <Button
          label="Add next function"
          icon={PlusIcon}
          variant={ButtonVariant.SECONDARY}
          size={depth === 0 ? ButtonSize.MEDIUM : ButtonSize.SMALL}
          onClick={handleAdd}
          isDisabled={
            isDisabled ||
            expression.source.kind === "empty" ||
            (expression.calls.length > 0 && !info.complete)
          }
        />
      )}
      {allowFunctions &&
        !isLiteralOnly &&
        isColumnFlow &&
        expression.calls.length > 0 &&
        !hasIncompleteFunction && (
          <FlexWrapper justifyContent={JustifyContent.END} fillWidth>
            <Button
              label="Add function"
              icon={PlusIcon}
              variant={ButtonVariant.SECONDARY}
              size={ButtonSize.MEDIUM}
              onClick={handleAdd}
              isDisabled={isDisabled || !info.complete}
            />
          </FlexWrapper>
        )}
      {!isColumnFlow &&
        !info.complete &&
        !isTransformExpressionEmpty(expression) &&
        info.error &&
        !hasInlineFieldValidationError && (
          <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
            {info.error}
          </Text>
        )}
      {isColumnFlow &&
        !info.complete &&
        expression.calls.length > 0 &&
        info.error &&
        !hasInlineFieldValidationError &&
        !isUnfilledDraftError(info.error) && (
          <Text size={TextSize.CAPTION} variant={TextVariant.ERROR}>
            {info.error}
          </Text>
        )}
    </FlexWrapper>
  );
};

export default PipelineCanvasPanelResourceTransformExpression;
