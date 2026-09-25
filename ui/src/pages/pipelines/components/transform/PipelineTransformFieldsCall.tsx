import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import {
  getTransformArgumentSpec,
  getTransformArgumentTypes,
  getTransformCallSlotCount,
  isTransformExprComplete,
  isTransformVariadic,
} from "@/pages/pipelines/components/transform/grammar/catalog";
import { TRANSFORM_EMPTY_EXPR } from "@/pages/pipelines/components/transform/grammar/chain";
import { getTransformArgumentPath } from "@/pages/pipelines/components/transform/grammar/paths";
import PipelineTransformFieldsArgument from "@/pages/pipelines/components/transform/PipelineTransformFieldsArgument";
import PipelineTransformFieldsBox from "@/pages/pipelines/components/transform/PipelineTransformFieldsBox";
import PipelineTransformFieldsFunctionPicker from "@/pages/pipelines/components/transform/PipelineTransformFieldsFunctionPicker";
import {
  usePipelineTransformFieldsEditor,
  usePipelineTransformFieldsEnvironment,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRemoveButton from "@/pages/pipelines/components/transform/PipelineTransformFieldsRemoveButton";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsRow";
import {
  type TransformChainCall,
  type TransformExpr,
  TransformExprKind,
} from "@/pages/pipelines/components/transform/types";
import {
  createTransformChainCall,
  getTransformExprType,
} from "@/pages/pipelines/components/transform/utils";

interface PipelineTransformFieldsCallProps {
  call: TransformChainCall;
  path: string;
  input: TransformExpr;
  inputPath: string;
  rootColumn: string | undefined;
  showFunction: boolean;
  depth: number;
  onChange: (call: TransformChainCall) => void;
}

const PipelineTransformFieldsCall = ({
  call,
  path,
  input,
  inputPath,
  rootColumn,
  showFunction,
  depth,
  onChange,
}: PipelineTransformFieldsCallProps) => {
  const editor = usePipelineTransformFieldsEditor();
  const { functionsByName } = usePipelineTransformFieldsEnvironment();
  const fn = functionsByName.get(call.fn);
  const inputType = isTransformExprComplete(input, functionsByName)
    ? getTransformExprType(input, inputPath, editor)
    : undefined;
  const sameTypeInput = input.kind === TransformExprKind.LITERAL ? undefined : inputType;
  const slots = fn ? Math.max(getTransformCallSlotCount(fn), call.args.length) : call.args.length;
  const isVariadic = fn ? isTransformVariadic(fn) : false;
  const fixedCount = fn ? Math.max(0, fn.args.length - 1 - (isVariadic ? 1 : 0)) : 0;

  const setArgument = (position: number, expr: TransformExpr) => {
    const args = Array.from(
      { length: Math.max(slots, position + 1) },
      (_, slot) => call.args[slot] ?? TRANSFORM_EMPTY_EXPR,
    );
    args[position] = expr;
    onChange({ ...call, args });
  };

  return (
    <PipelineTransformFieldsBox>
      {showFunction && (
        <PipelineTransformFieldsRow variant={PipelineTransformFieldsRowVariant.STEP}>
          <PipelineTransformFieldsFunctionPicker
            input={input}
            inputType={inputType}
            fn={call.fn}
            onChange={(name) =>
              onChange(
                createTransformChainCall(
                  name,
                  call,
                  sameTypeInput,
                  functionsByName,
                  editor.columns,
                  rootColumn,
                ),
              )
            }
            isError={call.fn !== "" && editor.errors.has(path)}
          />
        </PipelineTransformFieldsRow>
      )}
      {fn &&
        Array.from({ length: slots }, (_, position) => {
          const spec = getTransformArgumentSpec(fn, position + 1);
          const repeat = position - fixedCount;
          const label = spec
            ? `${spec.displayName || spec.name}${spec.isVariadic && repeat > 0 ? ` ${repeat + 1}` : ""}`
            : `Extra ${position + 1}`;
          const canRemove = (isVariadic && position > fixedCount) || spec === undefined;
          const argumentPath = getTransformArgumentPath(path, call, position);
          return (
            <PipelineTransformFieldsRow
              key={argumentPath}
              variant={PipelineTransformFieldsRowVariant.ARG}
              gutter={label}
              action={
                canRemove ? (
                  <PipelineTransformFieldsRemoveButton
                    label={`Remove ${label.toLowerCase()}`}
                    onClick={() =>
                      onChange({ ...call, args: call.args.filter((_, slot) => slot !== position) })
                    }
                  />
                ) : undefined
              }
              isBoxed={call.args[position]?.kind === TransformExprKind.CALL}
            >
              <PipelineTransformFieldsArgument
                expr={call.args[position] ?? TRANSFORM_EMPTY_EXPR}
                spec={spec}
                expectedTypes={getTransformArgumentTypes(fn, position + 1, sameTypeInput)}
                path={argumentPath}
                rootColumn={rootColumn}
                depth={depth + 1}
                onChange={(next) => setArgument(position, next)}
              />
            </PipelineTransformFieldsRow>
          );
        })}
      {fn && isVariadic && (
        <PipelineTransformFieldsRow variant={PipelineTransformFieldsRowVariant.ARG} isAddRow>
          <Button
            label={fn.variadicAddLabel || "Add value"}
            icon={PlusIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={() => onChange({ ...call, args: [...call.args, TRANSFORM_EMPTY_EXPR] })}
            isDisabled={editor.isDisabled}
          />
        </PipelineTransformFieldsRow>
      )}
    </PipelineTransformFieldsBox>
  );
};

export default PipelineTransformFieldsCall;
