import { match } from "ts-pattern";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";

import {
  TRANSFORM_LITERAL_KIND_TO_PLACEHOLDER_MAP,
  TRANSFORM_STEP_KIND_TO_ACTION_MAP,
} from "@/pages/pipelines/components/transform/constants";
import {
  createTransformChainExpr,
  createTransformColumnExpr,
  getTransformChain,
} from "@/pages/pipelines/components/transform/grammar/chain";
import {
  getTransformChainPaths,
  getTransformDropPath,
  getTransformOutputPath,
} from "@/pages/pipelines/components/transform/grammar/paths";
import PipelineTransformFieldsActionPicker from "@/pages/pipelines/components/transform/PipelineTransformFieldsActionPicker";
import PipelineTransformFieldsLeaf from "@/pages/pipelines/components/transform/PipelineTransformFieldsLeaf";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import {
  TransformActionKind,
  type TransformEditableStep,
  TransformExprKind,
  type TransformLeafExpr,
  TransformStepKind,
} from "@/pages/pipelines/components/transform/types";
import {
  applyTransformAction,
  getTransformChainAction,
  getTransformExprType,
  getTransformStepColumn,
} from "@/pages/pipelines/components/transform/utils";

interface PipelineTransformFieldsSubjectProps {
  step: TransformEditableStep;
  outputIndex: number;
  onChange: (step: TransformEditableStep) => void;
}

const PipelineTransformFieldsSubject = ({
  step,
  outputIndex,
  onChange,
}: PipelineTransformFieldsSubjectProps) => {
  const editor = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const output = step.kind === TransformStepKind.COMPUTE ? step.outputs[outputIndex] : undefined;
  const chain = getTransformChain(
    output ? output.expr : createTransformColumnExpr(getTransformStepColumn(step)),
  );
  const action =
    step.kind === TransformStepKind.COMPUTE
      ? getTransformChainAction(chain)
      : TRANSFORM_STEP_KIND_TO_ACTION_MAP[step.kind];
  const outputPath =
    step.kind === TransformStepKind.COMPUTE && output
      ? getTransformOutputPath(output, step.where !== null)
      : "";
  const paths = getTransformChainPaths(outputPath, chain);
  const rootType = getTransformExprType(chain.root, paths.root, editor);
  const isRootError = output
    ? editor.errors.has(paths.root)
    : step.kind === TransformStepKind.DROP && editor.errors.has(getTransformDropPath(0));
  const isActionError =
    output !== undefined &&
    (chain.calls.length > 0 ? editor.errors.has(paths.calls[0]) : editor.errors.has(outputPath));
  const literalKind = action.kind === TransformActionKind.LITERAL ? action.literalKind : undefined;
  const isColumnOnly = literalKind === undefined && action.kind !== TransformActionKind.FUNCTION;

  const setRoot = (root: TransformLeafExpr) => {
    const name = root.kind === TransformExprKind.COLUMN ? root.name : "";
    onChange(
      match(step)
        .with({ kind: TransformStepKind.RENAME }, (rename) => ({
          ...rename,
          pairs: rename.pairs.map((pair, index) => (index === 0 ? { ...pair, from: name } : pair)),
        }))
        .with({ kind: TransformStepKind.DROP }, (drop) => ({
          ...drop,
          names: drop.names.map((candidate, index) => (index === 0 ? name : candidate)),
        }))
        .with({ kind: TransformStepKind.COMPUTE }, (compute) => ({
          ...compute,
          outputs: compute.outputs.map((candidate, index) =>
            index === outputIndex
              ? { ...candidate, expr: createTransformChainExpr(root, chain.calls) }
              : candidate,
          ),
        }))
        .exhaustive(),
    );
  };

  return (
    <FlexWrapper gap={FlexGap.SMALL} fillWidth>
      <FlexItem grow={1} basis={0} minWidth={0}>
        <PipelineTransformFieldsActionPicker
          root={chain.root}
          rootType={rootType}
          action={action}
          offersKinds={outputIndex === 0 && chain.root.kind !== TransformExprKind.LITERAL}
          onChange={(next) =>
            onChange(
              applyTransformAction(step, outputIndex, next, {
                functionsByName,
                columns: editor.columns,
                rootType,
              }),
            )
          }
          isError={isActionError}
        />
      </FlexItem>
      <FlexItem grow={1} basis={0} minWidth={0}>
        <PipelineTransformFieldsLeaf
          expr={chain.root}
          onChange={setRoot}
          placeholder={
            literalKind
              ? TRANSFORM_LITERAL_KIND_TO_PLACEHOLDER_MAP[literalKind]
              : isColumnOnly
                ? "Choose a column"
                : "Choose a column or value"
          }
          logicalTypes={
            action.kind === TransformActionKind.FUNCTION
              ? functionsByName.get(action.fn)?.args[0]?.logicalTypes
              : undefined
          }
          isColumnOnly={isColumnOnly}
          isLiteralOnly={literalKind !== undefined}
          isError={isRootError}
        />
      </FlexItem>
    </FlexWrapper>
  );
};

export default PipelineTransformFieldsSubject;
