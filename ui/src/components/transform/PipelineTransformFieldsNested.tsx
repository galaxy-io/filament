import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";

import { chainOf, chainToExpr } from "@/components/transform/grammar/chain";
import { getTransformChainPaths } from "@/components/transform/grammar/paths";
import PipelineTransformFieldsBox from "@/components/transform/PipelineTransformFieldsBox";
import PipelineTransformFieldsChain from "@/components/transform/PipelineTransformFieldsChain";
import PipelineTransformFieldsFunctionPicker from "@/components/transform/PipelineTransformFieldsFunctionPicker";
import PipelineTransformFieldsLeaf from "@/components/transform/PipelineTransformFieldsLeaf";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/components/transform/PipelineTransformFieldsRow";
import type { TransformExpr, TransformLeafExpr } from "@/components/transform/types";
import { getTransformExprType, replaceTransformChainCall } from "@/components/transform/utils";

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
  const chain = chainOf(expr);
  const paths = getTransformChainPaths(basePath, chain);
  const first = chain.calls[0];
  const firstFn = first ? functionsByName.get(first.fn) : undefined;
  const setCalls = (calls: typeof chain.calls) => onChange(chainToExpr(chain.root, calls));

  return (
    <PipelineTransformFieldsBox>
      <PipelineTransformFieldsRow
        variant={PipelineTransformFieldsRowVariant.STEP}
        action={
          <Button
            icon={XIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.MEDIUM}
            onClick={() => setCalls(chain.calls.slice(1))}
            isDisabled={editor.isDisabled}
            ariaLabel={`Remove ${firstFn?.displayName || first?.fn || "function"}`}
          />
        }
      >
        <FlexWrapper gap={FlexGap.SMALL} fillWidth>
          <FlexItem grow={1} basis={0} minWidth={0}>
            <PipelineTransformFieldsFunctionPicker
              input={chain.root}
              inputPath={paths.root}
              fn={first?.fn ?? ""}
              onChange={(name) =>
                setCalls(
                  replaceTransformChainCall(
                    chain.calls,
                    0,
                    name,
                    getTransformExprType(chain.root, paths.root, editor),
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
              onChange={(root) => onChange(chainToExpr(root as TransformLeafExpr, chain.calls))}
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
