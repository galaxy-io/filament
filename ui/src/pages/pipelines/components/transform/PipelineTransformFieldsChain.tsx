import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import {
  getTransformCallSlotCount,
  isTransformExprComplete,
} from "@/pages/pipelines/components/transform/grammar/catalog";
import {
  createTransformChainExpr,
  getTransformChainInput,
} from "@/pages/pipelines/components/transform/grammar/chain";
import { getTransformChainPaths } from "@/pages/pipelines/components/transform/grammar/paths";
import PipelineTransformFieldsCall from "@/pages/pipelines/components/transform/PipelineTransformFieldsCall";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRemoveButton from "@/pages/pipelines/components/transform/PipelineTransformFieldsRemoveButton";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsRow";
import type {
  TransformChain,
  TransformChainCall,
} from "@/pages/pipelines/components/transform/types";

interface PipelineTransformFieldsChainProps {
  chain: TransformChain;
  basePath: string;
  rootColumn: string | undefined;
  hasSubject: boolean;
  depth: number;
  onChange: (calls: TransformChainCall[]) => void;
}

const PipelineTransformFieldsChain = ({
  chain,
  basePath,
  rootColumn,
  hasSubject,
  depth,
  onChange,
}: PipelineTransformFieldsChainProps) => {
  const { isDisabled } = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const paths = getTransformChainPaths(basePath, chain);
  const isComplete = isTransformExprComplete(
    createTransformChainExpr(chain.root, chain.calls),
    functionsByName,
  );
  const setCall = (index: number, call: TransformChainCall) =>
    onChange(chain.calls.map((candidate, slot) => (slot === index ? call : candidate)));

  return (
    <>
      {chain.calls.map((call, index) => {
        const fn = functionsByName.get(call.fn);
        const inSubject = hasSubject && index === 0;
        if (inSubject && (!fn || getTransformCallSlotCount(fn) === 0) && call.args.length === 0) {
          return null;
        }
        return (
          <PipelineTransformFieldsRow
            // biome-ignore lint/suspicious/noArrayIndexKey: call order is its grammar identity
            key={index}
            variant={PipelineTransformFieldsRowVariant.STEP}
            gutter={inSubject ? undefined : "then"}
            action={
              inSubject ? undefined : (
                <PipelineTransformFieldsRemoveButton
                  label={`Remove ${fn?.displayName || call.fn || "function"}`}
                  onClick={() => onChange(chain.calls.filter((_, slot) => slot !== index))}
                />
              )
            }
            isBoxed={!inSubject}
          >
            <PipelineTransformFieldsCall
              call={call}
              path={paths.calls[index]}
              input={getTransformChainInput(chain, index)}
              inputPath={index === 0 ? paths.root : paths.calls[index - 1]}
              rootColumn={rootColumn}
              showFunction={!inSubject}
              depth={depth}
              onChange={(next) => setCall(index, next)}
            />
          </PipelineTransformFieldsRow>
        );
      })}
      {(!hasSubject || chain.calls.length > 0) && (
        <PipelineTransformFieldsRow variant={PipelineTransformFieldsRowVariant.STEP} isAddRow>
          <Button
            label="Add function"
            icon={PlusIcon}
            variant={depth === 0 ? ButtonVariant.SECONDARY : ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={() => onChange([...chain.calls, { fn: "", args: [] }])}
            isDisabled={isDisabled || !isComplete}
          />
        </PipelineTransformFieldsRow>
      )}
    </>
  );
};

export default PipelineTransformFieldsChain;
