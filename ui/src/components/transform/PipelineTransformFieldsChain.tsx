import { PlusIcon, XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { chainInputOf, chainToExpr } from "@/components/transform/grammar/chain";
import { getTransformChainPaths } from "@/components/transform/grammar/paths";
import PipelineTransformFieldsCall from "@/components/transform/PipelineTransformFieldsCall";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/components/transform/PipelineTransformFieldsRow";
import type { TransformChain, TransformChainCall } from "@/components/transform/types";
import { getTransformCallSlotCount, isTransformExprComplete } from "@/components/transform/utils";

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
  const isComplete = isTransformExprComplete(chainToExpr(chain.root, chain.calls), functionsByName);
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
            gutter={
              inSubject ? undefined : (
                <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                  then
                </Text>
              )
            }
            action={
              inSubject ? undefined : (
                <Button
                  icon={XIcon}
                  variant={ButtonVariant.TERTIARY}
                  size={ButtonSize.MEDIUM}
                  onClick={() => onChange(chain.calls.filter((_, slot) => slot !== index))}
                  isDisabled={isDisabled}
                  ariaLabel={`Remove ${fn?.displayName || call.fn || "function"}`}
                />
              )
            }
            isBoxed={!inSubject}
          >
            <PipelineTransformFieldsCall
              call={call}
              path={paths.calls[index]}
              input={chainInputOf(chain, index)}
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
