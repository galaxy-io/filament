import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import { createTransformColumnExpr } from "@/pages/pipelines/components/transform/grammar/chain";
import { getTransformDropPath } from "@/pages/pipelines/components/transform/grammar/paths";
import PipelineTransformFieldsLeaf from "@/pages/pipelines/components/transform/PipelineTransformFieldsLeaf";
import { usePipelineTransformFieldsEditor } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRemoveButton from "@/pages/pipelines/components/transform/PipelineTransformFieldsRemoveButton";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsRow";
import {
  type TransformDropStep,
  type TransformEditableStep,
  TransformExprKind,
} from "@/pages/pipelines/components/transform/types";

interface PipelineTransformFieldsStepDropProps {
  step: TransformDropStep;
  onChange: (step: TransformEditableStep) => void;
}

const PipelineTransformFieldsStepDrop = ({
  step,
  onChange,
}: PipelineTransformFieldsStepDropProps) => {
  const { errors, isDisabled } = usePipelineTransformFieldsEditor();

  return (
    <>
      {step.names.map((name, index) =>
        index === 0 ? null : (
          <PipelineTransformFieldsRow
            // biome-ignore lint/suspicious/noArrayIndexKey: name order is its grammar identity
            key={index}
            variant={PipelineTransformFieldsRowVariant.STEP}
            gutter="and"
            action={
              <PipelineTransformFieldsRemoveButton
                label="Remove column"
                onClick={() =>
                  onChange({ ...step, names: step.names.filter((_, slot) => slot !== index) })
                }
              />
            }
          >
            <PipelineTransformFieldsLeaf
              expr={createTransformColumnExpr(name)}
              onChange={(next) =>
                onChange({
                  ...step,
                  names: step.names.map((candidate, slot) =>
                    slot === index
                      ? next.kind === TransformExprKind.COLUMN
                        ? next.name
                        : ""
                      : candidate,
                  ),
                })
              }
              placeholder="Choose a column"
              isColumnOnly
              isError={errors.has(getTransformDropPath(index))}
            />
          </PipelineTransformFieldsRow>
        ),
      )}
      <PipelineTransformFieldsRow variant={PipelineTransformFieldsRowVariant.STEP} isAddRow>
        <Button
          label="Add column"
          icon={PlusIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          onClick={() => onChange({ ...step, names: [...step.names, ""] })}
          isDisabled={isDisabled}
        />
      </PipelineTransformFieldsRow>
    </>
  );
};

export default PipelineTransformFieldsStepDrop;
