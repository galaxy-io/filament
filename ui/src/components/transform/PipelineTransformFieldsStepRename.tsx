import { PlusIcon, XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { TRANSFORM_NEW_COLUMN_SUFFIX } from "@/components/transform/constants";
import { createTransformColumnExpr } from "@/components/transform/grammar/chain";
import { getTransformRenamePath } from "@/components/transform/grammar/paths";
import PipelineTransformFieldsLeaf from "@/components/transform/PipelineTransformFieldsLeaf";
import { usePipelineTransformFieldsEditor } from "@/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/components/transform/PipelineTransformFieldsRow";
import {
  type TransformEditableStep,
  TransformExprKind,
  type TransformRenamePair,
  type TransformStep,
  type TransformStepKind,
} from "@/components/transform/types";

type RenameStep = Extract<TransformStep, { kind: TransformStepKind.RENAME }>;

interface PipelineTransformFieldsStepRenameProps {
  step: RenameStep;
  onChange: (step: TransformEditableStep) => void;
}

const PipelineTransformFieldsStepRename = ({
  step,
  onChange,
}: PipelineTransformFieldsStepRenameProps) => {
  const { errors, isDisabled } = usePipelineTransformFieldsEditor();
  const setPair = (index: number, pair: TransformRenamePair) =>
    onChange({
      ...step,
      pairs: step.pairs.map((candidate, slot) => (slot === index ? pair : candidate)),
    });

  return (
    <>
      {step.pairs.flatMap((pair, index) => [
        index > 0 && (
          <PipelineTransformFieldsRow
            // biome-ignore lint/suspicious/noArrayIndexKey: pair order is its grammar identity
            key={`column-${index}`}
            variant={PipelineTransformFieldsRowVariant.STEP}
            gutter={
              <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                and
              </Text>
            }
            action={
              <Button
                icon={XIcon}
                variant={ButtonVariant.TERTIARY}
                size={ButtonSize.MEDIUM}
                onClick={() =>
                  onChange({ ...step, pairs: step.pairs.filter((_, slot) => slot !== index) })
                }
                isDisabled={isDisabled}
                ariaLabel="Remove column"
              />
            }
          >
            <PipelineTransformFieldsLeaf
              expr={createTransformColumnExpr(pair.from)}
              onChange={(next) =>
                setPair(index, {
                  ...pair,
                  from: next.kind === TransformExprKind.COLUMN ? next.name : "",
                })
              }
              placeholder="Choose a column"
              isColumnOnly
            />
          </PipelineTransformFieldsRow>
        ),
        <PipelineTransformFieldsRow
          // biome-ignore lint/suspicious/noArrayIndexKey: pair order is its grammar identity
          key={`to-${index}`}
          variant={PipelineTransformFieldsRowVariant.STEP}
          gutter={
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
              to
            </Text>
          }
        >
          <TextInput
            value={pair.to}
            onChange={(to) => setPair(index, { ...pair, to })}
            placeholder={pair.from ? `${pair.from}${TRANSFORM_NEW_COLUMN_SUFFIX}` : "New name"}
            isError={errors.has(getTransformRenamePath(pair.from))}
            variant={InputVariant.TERTIARY}
            size={InputSize.MEDIUM}
            isDisabled={isDisabled}
            fillWidth
          />
        </PipelineTransformFieldsRow>,
      ])}
      <PipelineTransformFieldsRow variant={PipelineTransformFieldsRowVariant.STEP} isAddRow>
        <Button
          label="Add column"
          icon={PlusIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          onClick={() => onChange({ ...step, pairs: [...step.pairs, { from: "", to: "" }] })}
          isDisabled={isDisabled}
        />
      </PipelineTransformFieldsRow>
    </>
  );
};

export default PipelineTransformFieldsStepRename;
