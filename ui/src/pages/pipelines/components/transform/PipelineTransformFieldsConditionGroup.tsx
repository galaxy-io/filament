import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { FlexGap, FlexWrap } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { TRANSFORM_EMPTY_EXPR } from "@/pages/pipelines/components/transform/grammar/chain";
import {
  createTransformConditionExpr,
  createTransformConditionGroupExpr,
  getTransformConditionGroup,
  isTransformConditionGroupExpr,
  isTransformGroupCompact,
  type TransformConditionGroup,
} from "@/pages/pipelines/components/transform/grammar/groups";
import PipelineTransformFieldsBox from "@/pages/pipelines/components/transform/PipelineTransformFieldsBox";
import PipelineTransformFieldsConditionRow from "@/pages/pipelines/components/transform/PipelineTransformFieldsConditionRow";
import PipelineTransformFieldsNested from "@/pages/pipelines/components/transform/PipelineTransformFieldsNested";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRemoveButton from "@/pages/pipelines/components/transform/PipelineTransformFieldsRemoveButton";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsRow";
import {
  TransformConditionJoin,
  type TransformExpr,
} from "@/pages/pipelines/components/transform/types";

const JOIN_OPTIONS: SelectInputOption[] = Object.values(TransformConditionJoin).map((join) => ({
  id: join,
  label: join,
  value: join,
}));

interface PipelineTransformFieldsConditionGroupProps {
  where: TransformExpr;
  path: string;
  depth: number;
  onChange: (where: TransformExpr) => void;
}

const PipelineTransformFieldsConditionGroup = ({
  where,
  path,
  depth,
  onChange,
}: PipelineTransformFieldsConditionGroupProps) => {
  const { isDisabled } = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const group = getTransformConditionGroup(where, functionsByName, path);

  if (depth === 0 && !isTransformGroupCompact(group, functionsByName)) {
    return (
      <PipelineTransformFieldsNested
        expr={where}
        basePath={path}
        rootColumn={undefined}
        depth={0}
        onChange={onChange}
      />
    );
  }

  const commit = (next: TransformConditionGroup) => onChange(createTransformConditionExpr(next));
  const setItem = (index: number, expr: TransformExpr) =>
    commit({
      ...group,
      items: group.items.map((item, slot) => (slot === index ? { ...item, expr } : item)),
    });
  const append = (expr: TransformExpr) =>
    commit({ ...group, items: [...group.items, { expr, path: "" }] });
  const canRemove = group.items.length > 1;
  const removeButton = (index: number, label: string) =>
    canRemove ? (
      <PipelineTransformFieldsRemoveButton
        label={label}
        onClick={() => commit({ ...group, items: group.items.filter((_, slot) => slot !== index) })}
      />
    ) : undefined;
  const gutterOf = (index: number) =>
    index === 0 ? (
      "where"
    ) : index === 1 ? (
      <SelectInput
        options={JOIN_OPTIONS}
        value={JOIN_OPTIONS.find((option) => option.id === group.join) ?? null}
        onChange={(option) => commit({ ...group, join: option.id as TransformConditionJoin })}
        onReset={() => commit({ ...group, join: TransformConditionJoin.AND })}
        variant={InputVariant.TERTIARY}
        size={InputSize.SMALL}
        isDisabled={isDisabled}
      />
    ) : (
      group.join
    );

  return (
    <PipelineTransformFieldsBox>
      {group.items.map((item, index) =>
        isTransformConditionGroupExpr(item.expr, functionsByName) ? (
          <PipelineTransformFieldsRow
            // biome-ignore lint/suspicious/noArrayIndexKey: item order is its grammar identity
            key={index}
            variant={PipelineTransformFieldsRowVariant.COND}
            gutter={gutterOf(index)}
            action={removeButton(index, "Remove group")}
            isBoxed
          >
            <PipelineTransformFieldsConditionGroup
              where={item.expr}
              path={item.path}
              depth={depth + 1}
              onChange={(next) => setItem(index, next)}
            />
          </PipelineTransformFieldsRow>
        ) : (
          <PipelineTransformFieldsConditionRow
            // biome-ignore lint/suspicious/noArrayIndexKey: item order is its grammar identity
            key={index}
            expr={item.expr}
            path={item.path}
            gutter={gutterOf(index)}
            action={removeButton(index, "Remove condition")}
            onChange={(next) => setItem(index, next)}
          />
        ),
      )}
      <PipelineTransformFieldsRow
        variant={PipelineTransformFieldsRowVariant.COND}
        gutter={null}
        isAddRow
      >
        <FlexWrapper gap={FlexGap.SMALL} wrap={FlexWrap.WRAP}>
          <Button
            label="Add condition"
            icon={PlusIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={() => append(TRANSFORM_EMPTY_EXPR)}
            isDisabled={isDisabled}
          />
          <Button
            label="Add group"
            icon={PlusIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={() => append(createTransformConditionGroupExpr(group.join))}
            isDisabled={isDisabled}
          />
        </FlexWrapper>
      </PipelineTransformFieldsRow>
    </PipelineTransformFieldsBox>
  );
};

export default PipelineTransformFieldsConditionGroup;
