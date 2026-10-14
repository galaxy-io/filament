import type { FC } from "react";

import { match } from "ts-pattern";

import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import {
  TRANSFORM_BODY_INSET,
  TRANSFORM_HEADER_PADDING_X,
} from "@/pages/pipelines/components/transform/constants";
import { usePipelineTransformFieldsValidation } from "@/pages/pipelines/components/transform/hooks/usePipelineTransformFieldsValidation";
import {
  PipelineTransformFieldsEditorContext,
  usePipelineTransformFieldsActions,
  usePipelineTransformFieldsEnvironment,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsStepCompute from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepCompute";
import PipelineTransformFieldsStepDrop from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepDrop";
import PipelineTransformFieldsStepFooter from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepFooter";
import PipelineTransformFieldsStepHeader from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepHeader";
import PipelineTransformFieldsStepRename from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepRename";
import PipelineTransformFieldsStepSurface from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepSurface";
import PipelineTransformFieldsSubject from "@/pages/pipelines/components/transform/PipelineTransformFieldsSubject";
import {
  type PipelineTransformFieldsDraft,
  type PipelineTransformFieldsStepHandle,
  TransformStepKind,
} from "@/pages/pipelines/components/transform/types";

interface PipelineTransformFieldsStepCardProps {
  draft: PipelineTransformFieldsDraft;
  handle?: PipelineTransformFieldsStepHandle;
}

const PipelineTransformFieldsStepCard: FC<PipelineTransformFieldsStepCardProps> = ({
  draft,
  handle,
}) => {
  const { isReadOnly } = usePipelineTransformFieldsEnvironment();
  const { setDraft, cancel, save, removeStep } = usePipelineTransformFieldsActions();
  const { editor, issues, warnings, typeSummary, isSaveDisabled } =
    usePipelineTransformFieldsValidation(draft);
  const { id, step } = draft;

  return (
    <PipelineTransformFieldsEditorContext.Provider value={editor}>
      <PipelineTransformFieldsStepSurface $isHoverable>
        <PipelineTransformFieldsStepHeader isOpen onToggle={cancel} handle={handle}>
          <PipelineTransformFieldsSubject step={step} outputIndex={0} onChange={setDraft} />
        </PipelineTransformFieldsStepHeader>
        <Flex
          alignItems={AlignItems.START}
          padding={[
            0,
            TRANSFORM_HEADER_PADDING_X,
            TRANSFORM_HEADER_PADDING_X,
            TRANSFORM_BODY_INSET,
          ]}
          fillWidth
        >
          <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={8} fillWidth>
            {match(step)
              .with({ kind: TransformStepKind.RENAME }, (rename) => (
                <PipelineTransformFieldsStepRename step={rename} onChange={setDraft} />
              ))
              .with({ kind: TransformStepKind.DROP }, (drop) => (
                <PipelineTransformFieldsStepDrop step={drop} onChange={setDraft} />
              ))
              .with({ kind: TransformStepKind.COMPUTE }, (compute) => (
                <PipelineTransformFieldsStepCompute step={compute} onChange={setDraft} />
              ))
              .exhaustive()}
            <PipelineTransformFieldsStepFooter
              issues={issues}
              warnings={warnings}
              typeSummary={typeSummary}
              isSaveDisabled={isSaveDisabled}
              isDisabled={isReadOnly}
              onSave={save}
              onCancel={cancel}
              onDelete={id === null ? undefined : () => removeStep(id)}
            />
          </Flex>
        </Flex>
      </PipelineTransformFieldsStepSurface>
    </PipelineTransformFieldsEditorContext.Provider>
  );
};

export default PipelineTransformFieldsStepCard;
