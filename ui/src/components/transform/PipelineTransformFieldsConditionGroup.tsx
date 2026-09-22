import { PlusIcon, XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { FlexGap, FlexWrap } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { EMPTY_EXPR } from "@/components/transform/grammar/chain";
import {
  createTransformConditionGroup,
  groupOf,
  groupToExpr,
  isTransformGroupCompact,
  type TransformConditionGroup,
  type TransformConditionItem,
  TransformConditionItemKind,
} from "@/components/transform/grammar/groups";
import PipelineTransformFieldsBox from "@/components/transform/PipelineTransformFieldsBox";
import PipelineTransformFieldsConditionRow from "@/components/transform/PipelineTransformFieldsConditionRow";
import PipelineTransformFieldsNested from "@/components/transform/PipelineTransformFieldsNested";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/components/transform/PipelineTransformFieldsRow";
import { TransformConditionJoin, type TransformExpr } from "@/components/transform/types";

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
  const group = groupOf(where, functionsByName, path);

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

  const commit = (next: TransformConditionGroup) => onChange(groupToExpr(next));
  const setItem = (index: number, item: TransformConditionItem) =>
    commit({
      ...group,
      items: group.items.map((candidate, slot) => (slot === index ? item : candidate)),
    });
  const removeItem = (index: number) =>
    commit({ ...group, items: group.items.filter((_, slot) => slot !== index) });
  const canRemove = group.items.length > 1;
  const gutterOf = (index: number) =>
    index === 0 ? (
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        where
      </Text>
    ) : index === 1 ? (
      <SelectInput
        options={JOIN_OPTIONS}
        value={JOIN_OPTIONS.find((option) => option.id === group.join) ?? null}
        onChange={(option) => commit({ ...group, join: option.value as TransformConditionJoin })}
        onReset={() => commit({ ...group, join: TransformConditionJoin.AND })}
        variant={InputVariant.TERTIARY}
        size={InputSize.SMALL}
        isDisabled={isDisabled}
      />
    ) : (
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        {group.join}
      </Text>
    );

  return (
    <PipelineTransformFieldsBox>
      {group.items.map((item, index) =>
        item.kind === TransformConditionItemKind.GROUP ? (
          <PipelineTransformFieldsRow
            key={item.path || `group-${index}`}
            variant={PipelineTransformFieldsRowVariant.COND}
            gutter={gutterOf(index)}
            action={
              canRemove ? (
                <Button
                  icon={XIcon}
                  variant={ButtonVariant.TERTIARY}
                  size={ButtonSize.MEDIUM}
                  onClick={() => removeItem(index)}
                  isDisabled={isDisabled}
                  ariaLabel="Remove group"
                />
              ) : undefined
            }
            isBoxed
          >
            <PipelineTransformFieldsConditionGroup
              where={groupToExpr(item.group)}
              path={item.path}
              depth={depth + 1}
              onChange={(next) =>
                setItem(index, {
                  kind: TransformConditionItemKind.GROUP,
                  group: groupOf(next, functionsByName, item.path),
                  path: item.path,
                })
              }
            />
          </PipelineTransformFieldsRow>
        ) : (
          <PipelineTransformFieldsConditionRow
            key={item.path || `condition-${index}`}
            expr={item.expr}
            path={item.path}
            gutter={gutterOf(index)}
            onChange={(next) =>
              setItem(index, {
                kind: TransformConditionItemKind.CONDITION,
                expr: next,
                path: item.path,
              })
            }
            onRemove={canRemove ? () => removeItem(index) : undefined}
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
            onClick={() =>
              commit({
                ...group,
                items: [
                  ...group.items,
                  { kind: TransformConditionItemKind.CONDITION, expr: EMPTY_EXPR, path: "" },
                ],
              })
            }
            isDisabled={isDisabled}
          />
          <Button
            label="Add group"
            icon={PlusIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={() =>
              commit({
                ...group,
                items: [
                  ...group.items,
                  {
                    kind: TransformConditionItemKind.GROUP,
                    group: createTransformConditionGroup(group.join),
                    path: "",
                  },
                ],
              })
            }
            isDisabled={isDisabled}
          />
        </FlexWrapper>
      </PipelineTransformFieldsRow>
    </PipelineTransformFieldsBox>
  );
};

export default PipelineTransformFieldsConditionGroup;
