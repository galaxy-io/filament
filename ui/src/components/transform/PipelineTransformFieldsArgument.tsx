import type { TransformArgument } from "@/gen/ingestion/v1/transformations_pb";

import PipelineTransformFieldsLeaf from "@/components/transform/PipelineTransformFieldsLeaf";
import PipelineTransformFieldsNested from "@/components/transform/PipelineTransformFieldsNested";
import { usePipelineTransformFieldsEditor } from "@/components/transform/PipelineTransformFieldsProvider";
import { type TransformExpr, TransformExprKind } from "@/components/transform/types";

interface PipelineTransformFieldsArgumentProps {
  expr: TransformExpr;
  spec: TransformArgument | undefined;
  expectedTypes: string[];
  path: string;
  rootColumn: string | undefined;
  depth: number;
  onChange: (expr: TransformExpr) => void;
}

const PipelineTransformFieldsArgument = ({
  expr,
  spec,
  expectedTypes,
  path,
  rootColumn,
  depth,
  onChange,
}: PipelineTransformFieldsArgumentProps) => {
  const { errors } = usePipelineTransformFieldsEditor();
  if (expr.kind === TransformExprKind.CALL) {
    return (
      <PipelineTransformFieldsNested
        expr={expr}
        basePath={path}
        rootColumn={rootColumn}
        depth={depth}
        onChange={onChange}
      />
    );
  }
  return (
    <PipelineTransformFieldsLeaf
      expr={expr}
      onChange={onChange}
      placeholder={spec?.isColumn ? "Choose a column" : undefined}
      logicalTypes={expectedTypes}
      isLiteralOnly={spec?.isLiteral}
      isColumnOnly={spec?.isColumn}
      isOptional={spec?.isOptional}
      onApplyFunction={
        spec?.isLiteral
          ? undefined
          : () => onChange({ kind: TransformExprKind.CALL, fn: "", args: [expr] })
      }
      isError={errors.has(path)}
    />
  );
};

export default PipelineTransformFieldsArgument;
