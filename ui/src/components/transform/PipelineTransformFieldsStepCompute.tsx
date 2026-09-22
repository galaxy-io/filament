import { Fragment } from "react";

import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { TRANSFORM_NEW_COLUMN_SUFFIX } from "@/components/transform/constants";
import { chainOf, chainToExpr, EMPTY_EXPR } from "@/components/transform/grammar/chain";
import { getTransformOutputPath } from "@/components/transform/grammar/paths";
import PipelineTransformFieldsChain from "@/components/transform/PipelineTransformFieldsChain";
import PipelineTransformFieldsConditionGroup from "@/components/transform/PipelineTransformFieldsConditionGroup";
import PipelineTransformFieldsConditionScope from "@/components/transform/PipelineTransformFieldsConditionScope";
import { usePipelineTransformFieldsEditor } from "@/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/components/transform/PipelineTransformFieldsRow";
import PipelineTransformFieldsSubject from "@/components/transform/PipelineTransformFieldsSubject";
import {
  type TransformComputeOutput,
  type TransformComputeStep,
  type TransformEditableStep,
  TransformExprKind,
} from "@/components/transform/types";
import { isTransformOutputNameIssue } from "@/components/transform/utils";

const WHERE_PATH = "where";

interface PipelineTransformFieldsStepComputeProps {
  step: TransformComputeStep;
  onChange: (step: TransformEditableStep) => void;
}

const PipelineTransformFieldsStepCompute = ({
  step,
  onChange,
}: PipelineTransformFieldsStepComputeProps) => {
  const { errors, isDisabled } = usePipelineTransformFieldsEditor();
  const isMatchingRows = step.where !== null;
  const setOutput = (index: number, output: TransformComputeOutput) =>
    onChange({
      ...step,
      outputs: step.outputs.map((candidate, slot) => (slot === index ? output : candidate)),
    });

  return (
    <>
      {step.outputs.map((output, index) => {
        const chain = chainOf(output.expr);
        const rootColumn =
          chain.root.kind === TransformExprKind.COLUMN ? chain.root.name : undefined;
        const outputPath = getTransformOutputPath(output, isMatchingRows);
        const isNameError = (errors.get(outputPath) ?? []).some(isTransformOutputNameIssue);
        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: output order is its grammar identity
          <Fragment key={index}>
            {index > 0 && (
              <PipelineTransformFieldsRow
                variant={PipelineTransformFieldsRowVariant.STEP}
                action={
                  <Button
                    icon={XIcon}
                    variant={ButtonVariant.TERTIARY}
                    size={ButtonSize.MEDIUM}
                    onClick={() =>
                      onChange({
                        ...step,
                        outputs: step.outputs.filter((_, slot) => slot !== index),
                      })
                    }
                    isDisabled={isDisabled}
                    ariaLabel="Remove output"
                  />
                }
              >
                <PipelineTransformFieldsSubject
                  step={step}
                  outputIndex={index}
                  onChange={onChange}
                />
              </PipelineTransformFieldsRow>
            )}
            <PipelineTransformFieldsChain
              chain={chain}
              basePath={outputPath}
              rootColumn={rootColumn}
              hasSubject
              depth={0}
              onChange={(calls) =>
                setOutput(index, { ...output, expr: chainToExpr(chain.root, calls) })
              }
            />
            <PipelineTransformFieldsRow
              variant={PipelineTransformFieldsRowVariant.STEP}
              gutter={
                <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                  as
                </Text>
              }
            >
              <TextInput
                value={output.name}
                onChange={(name) => setOutput(index, { ...output, name })}
                placeholder={
                  rootColumn
                    ? `${rootColumn}${isMatchingRows ? "" : TRANSFORM_NEW_COLUMN_SUFFIX}`
                    : "Output column"
                }
                isError={isNameError}
                variant={InputVariant.TERTIARY}
                size={InputSize.MEDIUM}
                isDisabled={isDisabled}
                fillWidth
              />
            </PipelineTransformFieldsRow>
          </Fragment>
        );
      })}
      <PipelineTransformFieldsRow
        variant={PipelineTransformFieldsRowVariant.STEP}
        gutter={
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            on
          </Text>
        }
      >
        <PipelineTransformFieldsConditionScope
          isMatching={isMatchingRows}
          onChange={(isMatching) =>
            onChange({ ...step, where: isMatching ? (step.where ?? EMPTY_EXPR) : null })
          }
        />
      </PipelineTransformFieldsRow>
      {step.where !== null && (
        <PipelineTransformFieldsConditionGroup
          where={step.where}
          path={WHERE_PATH}
          depth={0}
          onChange={(where) => onChange({ ...step, where })}
        />
      )}
    </>
  );
};

export default PipelineTransformFieldsStepCompute;
