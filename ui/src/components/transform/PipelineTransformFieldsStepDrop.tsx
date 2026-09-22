import { PlusIcon, XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { createTransformColumnExpr } from "@/components/transform/grammar/chain";
import { getTransformDropPath } from "@/components/transform/grammar/paths";
import PipelineTransformFieldsLeaf from "@/components/transform/PipelineTransformFieldsLeaf";
import { usePipelineTransformFieldsEditor } from "@/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/components/transform/PipelineTransformFieldsRow";
import {
  type TransformEditableStep,
  TransformExprKind,
  type TransformStep,
  type TransformStepKind,
} from "@/components/transform/types";

type DropStep = Extract<TransformStep, { kind: TransformStepKind.DROP }>;

interface PipelineTransformFieldsStepDropProps {
  step: DropStep;
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
                  onChange({ ...step, names: step.names.filter((_, slot) => slot !== index) })
                }
                isDisabled={isDisabled}
                ariaLabel="Remove column"
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
