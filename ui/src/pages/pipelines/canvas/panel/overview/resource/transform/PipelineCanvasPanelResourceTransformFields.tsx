import { type CSSProperties, useEffect } from "react";

import { PlusIcon, XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { Resource } from "@/gen/ingestion/v1/connectors_pb";
import type { TransformFunction } from "@/gen/ingestion/v1/transformations_pb";

import {
  createEmptyTransformExpression,
  createTransformStepDefaultState,
  TRANSFORM_FLOW_SOURCE_WIDTH,
  TRANSFORM_STEP_KIND_OPTIONS,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/constants";
import PipelineCanvasPanelResourceTransformExpression, {
  TransformExpressionSourceField,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/PipelineCanvasPanelResourceTransformExpression";
import {
  type TransformColumn,
  type TransformComputeOutput,
  type TransformExpression,
  TransformRowScope,
  TransformStepKind,
  type TransformStepState,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/types";
import {
  getCompatibleTransformFunctions,
  getTransformArgumentSpec,
  getTransformConditionJoin,
  getTransformExpressionInfo,
  getTransformOutputName,
  isTransformConditionOperator,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/utils";

interface PipelineCanvasPanelResourceTransformFieldsProps {
  state: TransformStepState;
  resources: Resource["name"][];
  columns: TransformColumn[];
  functionsByName: Map<string, TransformFunction>;
  onChange: (partial: Partial<TransformStepState>) => void;
  isDisabled?: boolean;
}

const getColumnOptions = (columns: TransformColumn[]): SelectInputOption[] =>
  columns.map((column) => ({ id: column.name, label: column.name, value: column.name }));

const filterOptions = (term: string, options: SelectInputOption[]) =>
  options.filter((option) => option.label.toLowerCase().includes(term.toLowerCase()));

const CONTROL_GROUP_STYLE: CSSProperties = {
  border: 0,
  margin: 0,
  minWidth: 0,
  padding: 0,
  width: "100%",
};

interface ColumnSelectProps {
  label?: string;
  value: string;
  columns: TransformColumn[];
  onChange: (value: string) => void;
  placeholder?: string;
  error?: string;
  size?: InputSize;
  isDisabled?: boolean;
}

const ColumnSelect = ({
  label,
  value,
  columns,
  onChange,
  placeholder = "Choose a column",
  error,
  size = InputSize.LARGE,
  isDisabled = false,
}: ColumnSelectProps) => {
  const options = getColumnOptions(columns);

  useEffect(() => {
    if (!isDisabled && value === "" && columns.length === 1) onChange(columns[0].name);
  }, [columns, isDisabled, onChange, value]);

  return (
    <SelectInput
      label={label}
      options={options}
      value={options.find((option) => option.id === value) ?? null}
      onChange={(option) => onChange(option.value as string)}
      onSearch={filterOptions}
      variant={InputVariant.TERTIARY}
      placeholder={placeholder}
      error={error}
      size={size}
      isDisabled={isDisabled}
      fillWidth
    />
  );
};

interface ConditionGroup {
  mode: "and" | "or";
  conditions: TransformExpression[];
}

const splitConditionGroup = (
  expression: TransformExpression,
  functionsByName: Map<string, TransformFunction>,
): ConditionGroup | null => {
  const firstJoin = expression.calls.findIndex((call) => {
    const join = getTransformConditionJoin(functionsByName.get(call.name), functionsByName);
    return (join === "and" || join === "or") && call.args.length === 1;
  });
  if (firstJoin === -1) return { mode: "and", conditions: [expression] };

  const joins = expression.calls.slice(firstJoin);
  const mode = getTransformConditionJoin(
    functionsByName.get(joins[0]?.name ?? ""),
    functionsByName,
  );
  if (
    (mode !== "and" && mode !== "or") ||
    !joins.every(
      (call) =>
        getTransformConditionJoin(functionsByName.get(call.name), functionsByName) === mode &&
        call.args.length === 1,
    )
  ) {
    return null;
  }

  return {
    mode,
    conditions: [
      { ...expression, calls: expression.calls.slice(0, firstJoin) },
      ...joins.map((call) => call.args[0]),
    ],
  };
};

const joinConditionGroup = (
  conditions: TransformExpression[],
  mode: ConditionGroup["mode"],
  functionsByName: Map<string, TransformFunction>,
): TransformExpression => {
  const [first = createEmptyTransformExpression(), ...rest] = conditions;
  const joinFunction = [...functionsByName.values()].find(
    (fn) => getTransformConditionJoin(fn, functionsByName) === mode,
  );
  if (!joinFunction) return first;
  return {
    ...first,
    calls: [
      ...first.calls,
      ...rest.map((condition) => ({ name: joinFunction.name, args: [condition] })),
    ],
  };
};

const isCompactCondition = (
  expression: TransformExpression,
  functionsByName: Map<string, TransformFunction>,
): boolean => {
  if (expression.source.kind === "empty") return expression.calls.length === 0;
  if (expression.source.kind !== "column" || expression.calls.length > 1) return false;
  const call = expression.calls[0];
  if (!call) return true;
  const fn = functionsByName.get(call.name);
  return (
    isTransformConditionOperator(fn, functionsByName) &&
    call.args.every((argument) => argument.calls.length === 0)
  );
};

const lowerFirst = (value: string): string =>
  value === "" ? value : value.charAt(0).toLowerCase() + value.slice(1);

interface CompactConditionProps {
  expression: TransformExpression;
  columns: TransformColumn[];
  functionsByName: Map<string, TransformFunction>;
  onChange: (expression: TransformExpression) => void;
  onRemove?: () => void;
  isDisabled: boolean;
}

const CompactCondition = ({
  expression,
  columns,
  functionsByName,
  onChange,
  onRemove,
  isDisabled,
}: CompactConditionProps) => {
  const inputExpression = { ...expression, calls: [] };
  const inputInfo = getTransformExpressionInfo(inputExpression, columns, functionsByName);
  const selectedCall = expression.calls.length === 1 ? expression.calls[0] : undefined;
  const selectedFunction = selectedCall ? functionsByName.get(selectedCall.name) : undefined;
  const compatibleFunctions = getCompatibleTransformFunctions(inputInfo, functionsByName).filter(
    (fn) => isTransformConditionOperator(fn, functionsByName),
  );
  const availableFunctions =
    selectedFunction &&
    !compatibleFunctions.some((candidate) => candidate.name === selectedFunction.name)
      ? [...compatibleFunctions, selectedFunction]
      : compatibleFunctions;
  const operatorOptions: SelectInputOption[] = [
    ...(inputInfo.complete && inputInfo.type === "bool"
      ? [{ id: "direct", label: "is true", value: "direct" }]
      : []),
    ...availableFunctions.map((fn) => ({
      id: fn.name,
      label: lowerFirst(fn.displayName || fn.name),
      value: fn.name,
    })),
  ];
  const selectedOperator =
    selectedCall === undefined
      ? inputInfo.complete && inputInfo.type === "bool"
        ? (operatorOptions.find((option) => option.id === "direct") ?? null)
        : null
      : (operatorOptions.find((option) => option.id === selectedCall.name) ?? null);
  const argumentCount = selectedFunction
    ? Math.max(selectedFunction.args.length - 1, selectedCall?.args.length ?? 0)
    : 0;
  const isOperatorDisabled = isDisabled || !inputInfo.complete;

  const handleOperatorChange = (name: string) => {
    if (name === "direct") {
      onChange({ ...expression, calls: [] });
      return;
    }
    const fn = functionsByName.get(name);
    if (!fn) return;
    const keepComparisonOperand = Boolean(
      selectedCall !== undefined &&
        functionsByName.get(selectedCall.name)?.operatorSymbol &&
        fn.operatorSymbol,
    );
    onChange({
      ...expression,
      calls: [
        {
          name,
          args:
            keepComparisonOperand && selectedCall
              ? selectedCall.args
              : fn.args.slice(1).map(() => createEmptyTransformExpression()),
        },
      ],
    });
  };

  const renderArgument = (index: number) => {
    if (!selectedFunction || !selectedCall) return null;
    const spec = getTransformArgumentSpec(selectedFunction, index + 1);
    const argument = selectedCall.args[index] ?? createEmptyTransformExpression();
    const logicalTypes = spec?.isLiteral
      ? spec.logicalTypes
      : selectedFunction.sameType && inputInfo.complete && !inputInfo.isLiteral
        ? [inputInfo.type]
        : (spec?.logicalTypes ?? []);
    return (
      <TransformExpressionSourceField
        source={argument.source}
        columns={columns}
        onChange={(source) => {
          const args = Array.from(
            { length: Math.max(argumentCount, index + 1) },
            (_, slot) => selectedCall.args[slot] ?? createEmptyTransformExpression(),
          );
          args[index] = { source, calls: [] };
          onChange({ ...expression, calls: [{ ...selectedCall, args }] });
        }}
        placeholder="Value"
        logicalTypes={logicalTypes}
        isLiteralOnly={spec?.isLiteral}
        isColumnOnly={spec?.isColumn}
        isOptional={spec?.isOptional}
        size={InputSize.MEDIUM}
        isDisabled={isDisabled}
      />
    );
  };

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
      <FlexWrapper alignItems={AlignItems.END} gap={8} fillWidth>
        <FlexItem basis={TRANSFORM_FLOW_SOURCE_WIDTH} shrink={0} minWidth={0}>
          <ColumnSelect
            value={expression.source.kind === "column" ? expression.source.column : ""}
            columns={columns}
            onChange={(column) => onChange({ ...expression, source: { kind: "column", column } })}
            placeholder="Choose a column"
            size={InputSize.MEDIUM}
            isDisabled={isDisabled}
          />
        </FlexItem>
        <FlexItem grow={1} basis={0} minWidth={0}>
          <fieldset
            aria-label={
              inputInfo.complete
                ? "Condition operator"
                : "Condition operator unavailable until a column is selected"
            }
            aria-disabled={isOperatorDisabled}
            disabled={isOperatorDisabled}
            style={CONTROL_GROUP_STYLE}
          >
            <SelectInput
              options={operatorOptions}
              value={selectedOperator}
              onChange={(option) => handleOperatorChange(option.value as string)}
              onSearch={filterOptions}
              variant={InputVariant.TERTIARY}
              placeholder="Choose an operator"
              size={InputSize.MEDIUM}
              isDisabled={isOperatorDisabled}
              fillWidth
            />
          </fieldset>
        </FlexItem>
        {argumentCount > 0 ? (
          <FlexItem grow={1} basis={0} minWidth={0}>
            {renderArgument(0)}
          </FlexItem>
        ) : (
          <FlexItem grow={1} basis={0} minWidth={0} />
        )}
        {onRemove && (
          <Button
            icon={XIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.MEDIUM}
            onClick={onRemove}
            isDisabled={isDisabled}
            ariaLabel="Remove condition"
          />
        )}
      </FlexWrapper>
      {Array.from({ length: Math.max(0, argumentCount - 1) }, (_, offset) => (
        <FlexWrapper
          // biome-ignore lint/suspicious/noArrayIndexKey: argument positions are their grammar identity
          key={`${selectedCall?.name ?? "condition"}:${offset + 1}`}
          fillWidth
        >
          {renderArgument(offset + 1)}
        </FlexWrapper>
      ))}
    </FlexWrapper>
  );
};

interface LegacyOutputProps {
  output: TransformComputeOutput;
  index: number;
  columns: TransformColumn[];
  functionsByName: Map<string, TransformFunction>;
  onChange: (output: TransformComputeOutput) => void;
  isDisabled: boolean;
}

const LegacyOutput = ({
  output,
  index,
  columns,
  functionsByName,
  onChange,
  isDisabled,
}: LegacyOutputProps) => (
  <Widget variant={WidgetVariant.SECONDARY_ALT} noHover padding="12px" fillWidth>
    <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
      <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
        Existing output {index + 1}
      </Text>
      <PipelineCanvasPanelResourceTransformExpression
        expression={output.expression}
        columns={columns}
        functionsByName={functionsByName}
        onChange={(expression) => onChange({ ...output, expression })}
        label="Expression"
        sourceLabel="Starting value"
        isDisabled={isDisabled}
      />
      <TextInput
        label="As"
        value={output.name}
        onChange={(name) => onChange({ ...output, name })}
        variant={InputVariant.TERTIARY}
        size={InputSize.LARGE}
        isDisabled={isDisabled}
        fillWidth
      />
    </FlexWrapper>
  </Widget>
);

const PipelineCanvasPanelResourceTransformFields = ({
  state,
  resources,
  columns,
  functionsByName,
  onChange,
  isDisabled = false,
}: PipelineCanvasPanelResourceTransformFieldsProps) => {
  const resourceOptions: SelectInputOption[] = resources.map((resource) => ({
    id: resource,
    label: resource,
    value: resource,
  }));
  const firstRename = state.renames[0] ?? { source: "", target: "" };
  const firstDrop = state.drops[0] ?? "";
  const firstOutput = state.outputs[0] ?? {
    name: "",
    expression: createEmptyTransformExpression(),
  };
  const computeColumn =
    firstOutput.expression.source.kind === "column"
      ? firstOutput.expression.source.column
      : firstOutput.name;
  // Empty is the normal pre-column authoring state. Only an existing
  // literal-root expression needs the generic, lossless editor.
  const usesColumnFlow = firstOutput.expression.source.kind !== "literal";
  const selectedColumn =
    state.kind === TransformStepKind.RENAME
      ? firstRename.source
      : state.kind === TransformStepKind.DROP
        ? firstDrop
        : computeColumn;
  const outputInfo = getTransformExpressionInfo(firstOutput.expression, columns, functionsByName);
  const outputName = getTransformOutputName(firstOutput);
  const outputTarget = columns.find((column) => column.name === outputName);
  const asError =
    state.rowScope !== TransformRowScope.MATCHING || !outputInfo.complete
      ? undefined
      : outputTarget === undefined
        ? "Matching rows can only update an existing column."
        : outputTarget.logicalType !== outputInfo.type
          ? `${outputName} is ${outputTarget.logicalType}; this expression returns ${outputInfo.type}.`
          : undefined;
  const conditionGroup = splitConditionGroup(state.where, functionsByName);
  const useCompactConditions =
    conditionGroup?.conditions.every((condition) =>
      isCompactCondition(condition, functionsByName),
    ) ?? false;

  const updateOutput = (index: number, output: TransformComputeOutput) =>
    onChange({
      outputs: state.outputs.map((candidate, slot) => (slot === index ? output : candidate)),
    });

  const handleOperationChange = (kind: TransformStepKind) => {
    const partial: Partial<TransformStepState> = { kind };
    if (selectedColumn !== "") {
      if (kind === TransformStepKind.RENAME && firstRename.source === "") {
        partial.renames = [{ ...firstRename, source: selectedColumn }];
      }
      if (kind === TransformStepKind.DROP && firstDrop === "") {
        partial.drops = [selectedColumn];
      }
      if (kind === TransformStepKind.COMPUTE && firstOutput.expression.source.kind === "empty") {
        partial.outputs = [
          {
            ...firstOutput,
            expression: {
              ...firstOutput.expression,
              source: { kind: "column", column: selectedColumn },
            },
          },
        ];
      }
    }
    onChange(partial);
  };

  const scopeItems: SwitcherInputItem[] = [
    {
      id: TransformRowScope.ALL,
      label: "All rows",
      onClick: () => !isDisabled && onChange({ rowScope: TransformRowScope.ALL }),
    },
    {
      id: TransformRowScope.MATCHING,
      label: "Matching rows",
      onClick: () => !isDisabled && onChange({ rowScope: TransformRowScope.MATCHING }),
    },
  ];
  const conditionModeOptions: SelectInputOption[] = [
    { id: "and", label: "All conditions", value: "and" },
    { id: "or", label: "Any condition", value: "or" },
  ];

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
      {resources.length > 1 && (
        <SelectInput
          label="Resource"
          options={resourceOptions}
          value={resourceOptions.find((option) => option.id === state.resource) ?? null}
          onChange={(option) => {
            const next = createTransformStepDefaultState(option.value as string);
            onChange({ ...next, kind: state.kind });
          }}
          variant={InputVariant.TERTIARY}
          placeholder="Choose a resource"
          size={InputSize.LARGE}
          isDisabled={isDisabled}
          fillWidth
        />
      )}

      {state.kind === TransformStepKind.RENAME && (
        <ColumnSelect
          label="Column"
          value={firstRename.source}
          columns={columns}
          onChange={(source) => onChange({ renames: [{ ...firstRename, source }] })}
          placeholder={state.resource === "" ? "Choose a resource first" : "Choose a column"}
          isDisabled={isDisabled || state.resource === ""}
        />
      )}
      {state.kind === TransformStepKind.DROP && (
        <ColumnSelect
          label="Column"
          value={firstDrop}
          columns={columns}
          onChange={(name) => onChange({ drops: [name] })}
          placeholder={state.resource === "" ? "Choose a resource first" : "Choose a column"}
          isDisabled={isDisabled || state.resource === ""}
        />
      )}
      {state.kind === TransformStepKind.COMPUTE && (
        <ColumnSelect
          label="Column"
          value={computeColumn}
          columns={columns}
          onChange={(column) =>
            updateOutput(0, {
              ...firstOutput,
              expression: {
                ...firstOutput.expression,
                source: { kind: "column", column },
              },
            })
          }
          placeholder={state.resource === "" ? "Choose a resource first" : "Choose a column"}
          isDisabled={isDisabled || state.resource === ""}
        />
      )}

      <SelectInput
        label="Operation"
        options={TRANSFORM_STEP_KIND_OPTIONS}
        value={TRANSFORM_STEP_KIND_OPTIONS.find((option) => option.id === state.kind) ?? null}
        onChange={(option) => handleOperationChange(option.value as TransformStepKind)}
        variant={InputVariant.TERTIARY}
        size={InputSize.LARGE}
        isDisabled={isDisabled}
        fillWidth
      />

      {state.kind === TransformStepKind.RENAME && (
        <TextInput
          label="New column name"
          value={firstRename.target}
          onChange={(target) => onChange({ renames: [{ ...firstRename, target }] })}
          placeholder={firstRename.source === "" ? "Enter a name" : `${firstRename.source}_v2`}
          variant={InputVariant.TERTIARY}
          size={InputSize.LARGE}
          isDisabled={isDisabled}
          fillWidth
        />
      )}

      {state.kind === TransformStepKind.COMPUTE && (
        <>
          {usesColumnFlow ? (
            <PipelineCanvasPanelResourceTransformExpression
              expression={firstOutput.expression}
              columns={columns}
              functionsByName={functionsByName}
              onChange={(expression) => updateOutput(0, { ...firstOutput, expression })}
              label="Expression"
              showSource={false}
              isColumnFlow
              isDisabled={isDisabled || computeColumn === ""}
            />
          ) : (
            <PipelineCanvasPanelResourceTransformExpression
              expression={firstOutput.expression}
              columns={columns}
              functionsByName={functionsByName}
              onChange={(expression) => updateOutput(0, { ...firstOutput, expression })}
              label="Expression"
              sourceLabel="Starting value"
              isDisabled={isDisabled}
            />
          )}

          <TextInput
            label="As"
            value={firstOutput.name}
            onChange={(name) => updateOutput(0, { ...firstOutput, name })}
            placeholder={computeColumn}
            error={asError}
            variant={InputVariant.TERTIARY}
            size={InputSize.LARGE}
            isDisabled={isDisabled}
            fillWidth
          />

          {state.outputs.slice(1).map((output, offset) => {
            const index = offset + 1;
            return (
              <LegacyOutput
                key={index}
                output={output}
                index={index}
                columns={columns}
                functionsByName={functionsByName}
                onChange={(next) => updateOutput(index, next)}
                isDisabled={isDisabled}
              />
            );
          })}

          <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              Rows
            </Text>
            <SwitcherInput
              items={scopeItems}
              selectedId={state.rowScope}
              variant={InputVariant.TERTIARY}
              size={InputSize.MEDIUM}
            />
          </FlexWrapper>

          {state.rowScope === TransformRowScope.MATCHING && (
            <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
              {useCompactConditions && conditionGroup ? (
                <>
                  {conditionGroup.conditions.length > 1 && (
                    <SelectInput
                      options={conditionModeOptions}
                      value={
                        conditionModeOptions.find((option) => option.id === conditionGroup.mode) ??
                        null
                      }
                      onChange={(option) => {
                        const mode = option.value;
                        if (!isDisabled && conditionGroup && (mode === "and" || mode === "or")) {
                          onChange({
                            where: joinConditionGroup(
                              conditionGroup.conditions,
                              mode,
                              functionsByName,
                            ),
                          });
                        }
                      }}
                      variant={InputVariant.TERTIARY}
                      size={InputSize.MEDIUM}
                      width={180}
                      isDisabled={isDisabled}
                    />
                  )}
                  {conditionGroup.conditions.map((condition, index) => (
                    <CompactCondition
                      // biome-ignore lint/suspicious/noArrayIndexKey: condition order is its grammar identity
                      key={index}
                      expression={condition}
                      columns={columns}
                      functionsByName={functionsByName}
                      onChange={(next) => {
                        const conditions = conditionGroup.conditions.map((candidate, slot) =>
                          slot === index ? next : candidate,
                        );
                        onChange({
                          where: joinConditionGroup(
                            conditions,
                            conditionGroup.mode,
                            functionsByName,
                          ),
                        });
                      }}
                      onRemove={
                        conditionGroup.conditions.length > 1
                          ? () => {
                              const conditions = conditionGroup.conditions.filter(
                                (_, slot) => slot !== index,
                              );
                              onChange({
                                where: joinConditionGroup(
                                  conditions,
                                  conditionGroup.mode,
                                  functionsByName,
                                ),
                              });
                            }
                          : undefined
                      }
                      isDisabled={isDisabled}
                    />
                  ))}
                  <FlexWrapper justifyContent={JustifyContent.END} fillWidth>
                    <Button
                      label="Add condition"
                      icon={PlusIcon}
                      variant={ButtonVariant.SECONDARY}
                      size={ButtonSize.MEDIUM}
                      onClick={() =>
                        onChange({
                          where: joinConditionGroup(
                            [...conditionGroup.conditions, createEmptyTransformExpression()],
                            conditionGroup.mode,
                            functionsByName,
                          ),
                        })
                      }
                      isDisabled={isDisabled}
                    />
                  </FlexWrapper>
                </>
              ) : (
                <PipelineCanvasPanelResourceTransformExpression
                  expression={state.where}
                  columns={columns}
                  functionsByName={functionsByName}
                  onChange={(where: TransformExpression) => onChange({ where })}
                  sourceLabel="Starting value"
                  depth={1}
                  isDisabled={isDisabled}
                />
              )}
              <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY}>
                Other rows stay unchanged.
              </Text>
            </FlexWrapper>
          )}
        </>
      )}
    </FlexWrapper>
  );
};

export default PipelineCanvasPanelResourceTransformFields;
