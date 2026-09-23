import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";

import {
  createTransformChainExpr,
  getTransformChain,
} from "@/pages/pipelines/components/transform/grammar/chain";
import { getTransformChainPaths } from "@/pages/pipelines/components/transform/grammar/paths";
import PipelineTransformFieldsBox from "@/pages/pipelines/components/transform/PipelineTransformFieldsBox";
import PipelineTransformFieldsChain from "@/pages/pipelines/components/transform/PipelineTransformFieldsChain";
import PipelineTransformFieldsFunctionPicker from "@/pages/pipelines/components/transform/PipelineTransformFieldsFunctionPicker";
import PipelineTransformFieldsLeaf from "@/pages/pipelines/components/transform/PipelineTransformFieldsLeaf";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRemoveButton from "@/pages/pipelines/components/transform/PipelineTransformFieldsRemoveButton";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsRow";
import type { TransformExpr } from "@/pages/pipelines/components/transform/types";
import {
  getTransformExprType,
  replaceTransformChainCall,
} from "@/pages/pipelines/components/transform/utils";

interface PipelineTransformFieldsNestedProps {
  expr: TransformExpr;
  basePath: string;
  rootColumn: string | undefined;
  depth: number;
  onChange: (expr: TransformExpr) => void;
}

const PipelineTransformFieldsNested = ({
  expr,
  basePath,
  rootColumn,
  depth,
  onChange,
}: PipelineTransformFieldsNestedProps) => {
  const editor = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const chain = getTransformChain(expr);
  const paths = getTransformChainPaths(basePath, chain);
  const rootType = getTransformExprType(chain.root, paths.root, editor);
  const first = chain.calls[0];
  const firstFn = first ? functionsByName.get(first.fn) : undefined;
  const setCalls = (calls: typeof chain.calls) =>
    onChange(createTransformChainExpr(chain.root, calls));

  return (
    <PipelineTransformFieldsBox>
      <PipelineTransformFieldsRow
        variant={PipelineTransformFieldsRowVariant.STEP}
        action={
          <PipelineTransformFieldsRemoveButton
            label={`Remove ${firstFn?.displayName || first?.fn || "function"}`}
            onClick={() => setCalls(chain.calls.slice(1))}
          />
        }
      >
        <FlexWrapper gap={FlexGap.SMALL} fillWidth>
          <FlexItem grow={1} basis={0} minWidth={0}>
            <PipelineTransformFieldsFunctionPicker
              input={chain.root}
              inputType={rootType}
              fn={first?.fn ?? ""}
              onChange={(name) =>
                setCalls(
                  replaceTransformChainCall(
                    chain.calls,
                    0,
                    name,
                    rootType,
                    functionsByName,
                    editor.columns,
                    rootColumn,
                  ),
                )
              }
              isError={first !== undefined && first.fn !== "" && editor.errors.has(paths.calls[0])}
            />
          </FlexItem>
          <FlexItem grow={1} basis={0} minWidth={0}>
            <PipelineTransformFieldsLeaf
              expr={chain.root}
              onChange={(root) => onChange(createTransformChainExpr(root, chain.calls))}
              logicalTypes={firstFn?.args[0]?.logicalTypes}
              isError={editor.errors.has(paths.root)}
            />
          </FlexItem>
        </FlexWrapper>
      </PipelineTransformFieldsRow>
      <PipelineTransformFieldsChain
        chain={chain}
        basePath={basePath}
        rootColumn={rootColumn}
        hasSubject
        depth={depth + 1}
        onChange={setCalls}
      />
    </PipelineTransformFieldsBox>
  );
};

export default PipelineTransformFieldsNested;
