import { Fragment } from "react";

import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import { TRANSFORM_NEW_COLUMN_SUFFIX } from "@/pages/pipelines/components/transform/constants";
import {
  createTransformChainExpr,
  getTransformChain,
  getTransformRootColumn,
  TRANSFORM_EMPTY_EXPR,
} from "@/pages/pipelines/components/transform/grammar/chain";
import { getTransformOutputPath } from "@/pages/pipelines/components/transform/grammar/paths";
import PipelineTransformFieldsChain from "@/pages/pipelines/components/transform/PipelineTransformFieldsChain";
import PipelineTransformFieldsConditionGroup from "@/pages/pipelines/components/transform/PipelineTransformFieldsConditionGroup";
import PipelineTransformFieldsConditionScope from "@/pages/pipelines/components/transform/PipelineTransformFieldsConditionScope";
import { usePipelineTransformFieldsEditor } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRemoveButton from "@/pages/pipelines/components/transform/PipelineTransformFieldsRemoveButton";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsRow";
import PipelineTransformFieldsSubject from "@/pages/pipelines/components/transform/PipelineTransformFieldsSubject";
import type {
  TransformComputeOutput,
  TransformComputeStep,
  TransformEditableStep,
} from "@/pages/pipelines/components/transform/types";

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
        const chain = getTransformChain(output.expr);
        const rootColumn = getTransformRootColumn(output.expr);
        const outputPath = getTransformOutputPath(output, isMatchingRows);
        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: output order is its grammar identity
          <Fragment key={index}>
            {index > 0 && (
              <PipelineTransformFieldsRow
                variant={PipelineTransformFieldsRowVariant.STEP}
                action={
                  <PipelineTransformFieldsRemoveButton
                    label="Remove output"
                    onClick={() =>
                      onChange({
                        ...step,
                        outputs: step.outputs.filter((_, slot) => slot !== index),
                      })
                    }
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
                setOutput(index, { ...output, expr: createTransformChainExpr(chain.root, calls) })
              }
            />
            <PipelineTransformFieldsRow
              variant={PipelineTransformFieldsRowVariant.STEP}
              gutter="as"
            >
              <TextInput
                value={output.name}
                onChange={(name) => setOutput(index, { ...output, name })}
                placeholder={
                  rootColumn
                    ? `${rootColumn}${isMatchingRows ? "" : TRANSFORM_NEW_COLUMN_SUFFIX}`
                    : "Output column"
                }
                isError={errors.has(outputPath)}
                variant={InputVariant.TERTIARY}
                size={InputSize.MEDIUM}
                isDisabled={isDisabled}
                fillWidth
              />
            </PipelineTransformFieldsRow>
          </Fragment>
        );
      })}
      <PipelineTransformFieldsRow variant={PipelineTransformFieldsRowVariant.STEP} gutter="on">
        <PipelineTransformFieldsConditionScope
          isMatching={isMatchingRows}
          onChange={(isMatching) =>
            onChange({ ...step, where: isMatching ? (step.where ?? TRANSFORM_EMPTY_EXPR) : null })
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
