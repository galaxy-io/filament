import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";

import {
  TRANSFORM_ACTION_TO_LITERAL_KIND_MAP,
  TRANSFORM_COPY_ACTION,
  TRANSFORM_LITERAL_KIND_TO_PLACEHOLDER_MAP,
} from "@/components/transform/constants";
import { chainOf, chainToExpr, EMPTY_EXPR } from "@/components/transform/grammar/chain";
import {
  getTransformChainPaths,
  getTransformOutputPath,
} from "@/components/transform/grammar/paths";
import PipelineTransformFieldsActionPicker from "@/components/transform/PipelineTransformFieldsActionPicker";
import PipelineTransformFieldsLeaf from "@/components/transform/PipelineTransformFieldsLeaf";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/components/transform/PipelineTransformFieldsProvider";
import {
  type TransformEditableStep,
  type TransformExpr,
  TransformExprKind,
  type TransformLeafExpr,
  TransformStepKind,
} from "@/components/transform/types";
import {
  convertTransformStep,
  createTransformLiteral,
  getTransformExprAction,
  getTransformExprType,
  getTransformStepAction,
  getTransformSubjectErrors,
  replaceTransformChainCall,
  toTransformComputeStep,
} from "@/components/transform/utils";

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
  const isMatchingRows = step.kind === TransformStepKind.COMPUTE && step.where !== null;
  const { root, action } = output
    ? getTransformExprAction(output.expr)
    : getTransformStepAction(step);
  const chain = chainOf(output?.expr ?? EMPTY_EXPR);
  const rootPath = output
    ? getTransformChainPaths(getTransformOutputPath(output, isMatchingRows), chain).root
    : "";
  const { isRootError, isActionError } = getTransformSubjectErrors(
    step,
    outputIndex,
    editor.errors,
  );
  const actionLiteralKind = TRANSFORM_ACTION_TO_LITERAL_KIND_MAP.get(action);
  const isColumnOnly = step.kind !== TransformStepKind.COMPUTE || action === TRANSFORM_COPY_ACTION;

  const setOutputExpr = (expr: TransformExpr) => {
    const compute = toTransformComputeStep(step);
    onChange({
      ...compute,
      outputs: compute.outputs.map((candidate, index) =>
        index === outputIndex ? { ...candidate, expr } : candidate,
      ),
    });
  };
  const setRoot = (next: TransformLeafExpr) => {
    const name = next.kind === TransformExprKind.COLUMN ? next.name : "";
    switch (step.kind) {
      case TransformStepKind.RENAME:
        return onChange({
          ...step,
          pairs: step.pairs.map((pair, index) => (index === 0 ? { ...pair, from: name } : pair)),
        });
      case TransformStepKind.DROP:
        return onChange({
          ...step,
          names: step.names.map((candidate, index) => (index === 0 ? name : candidate)),
        });
      case TransformStepKind.COMPUTE:
        return setOutputExpr(chainToExpr(next, chain.calls));
    }
  };

  return (
    <FlexWrapper gap={FlexGap.SMALL} fillWidth>
      <FlexItem grow={1} basis={0} minWidth={0}>
        <PipelineTransformFieldsActionPicker
          root={root}
          rootPath={rootPath}
          action={action}
          offersKinds={outputIndex === 0 && root.kind !== TransformExprKind.LITERAL}
          onKind={(kind) => onChange(convertTransformStep(step, kind))}
          onCopy={() => setOutputExpr(root.kind === TransformExprKind.COLUMN ? root : EMPTY_EXPR)}
          onLiteral={(literalKind) => setOutputExpr(createTransformLiteral(literalKind))}
          onFunction={(fn) =>
            setOutputExpr(
              chainToExpr(
                root,
                replaceTransformChainCall(
                  chain.calls,
                  0,
                  fn,
                  getTransformExprType(root, rootPath, editor),
                  functionsByName,
                  editor.columns,
                  root.kind === TransformExprKind.COLUMN ? root.name : undefined,
                ),
              ),
            )
          }
          isError={isActionError}
        />
      </FlexItem>
      <FlexItem grow={1} basis={0} minWidth={0}>
        <PipelineTransformFieldsLeaf
          expr={root}
          onChange={(next) => setRoot(next as TransformLeafExpr)}
          placeholder={
            actionLiteralKind
              ? TRANSFORM_LITERAL_KIND_TO_PLACEHOLDER_MAP[actionLiteralKind]
              : isColumnOnly
                ? "Choose a column"
                : "Choose a column or value"
          }
          logicalTypes={functionsByName.get(action)?.args[0]?.logicalTypes}
          isColumnOnly={isColumnOnly}
          isLiteralOnly={actionLiteralKind !== undefined}
          isError={isRootError}
        />
      </FlexItem>
    </FlexWrapper>
  );
};

export default PipelineTransformFieldsSubject;
