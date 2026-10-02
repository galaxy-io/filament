import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import { TRANSFORM_NEW_COLUMN_SUFFIX } from "@/pages/pipelines/components/transform/constants";
import { createTransformColumnExpr } from "@/pages/pipelines/components/transform/grammar/chain";
import { getTransformRenamePath } from "@/pages/pipelines/components/transform/grammar/paths";
import PipelineTransformFieldsLeaf from "@/pages/pipelines/components/transform/PipelineTransformFieldsLeaf";
import { usePipelineTransformFieldsEditor } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRemoveButton from "@/pages/pipelines/components/transform/PipelineTransformFieldsRemoveButton";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsRow";
import {
  type TransformEditableStep,
  TransformExprKind,
  type TransformRenamePair,
  type TransformRenameStep,
} from "@/pages/pipelines/components/transform/types";

interface PipelineTransformFieldsStepRenameProps {
  step: TransformRenameStep;
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
            gutter="and"
            action={
              <PipelineTransformFieldsRemoveButton
                label="Remove column"
                onClick={() =>
                  onChange({ ...step, pairs: step.pairs.filter((_, slot) => slot !== index) })
                }
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
          gutter="to"
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
