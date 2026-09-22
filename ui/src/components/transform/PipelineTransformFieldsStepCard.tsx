import { match } from "ts-pattern";

import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import {
  TRANSFORM_GAP,
  TRANSFORM_HANDLE,
  TRANSFORM_HEADER_PADDING_X,
} from "@/components/transform/constants";
import { usePipelineTransformFieldsValidation } from "@/components/transform/hooks/usePipelineTransformFieldsValidation";
import {
  PipelineTransformFieldsEditorProvider,
  usePipelineTransformFieldsActions,
  usePipelineTransformFieldsEnvironment,
} from "@/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsRow, {
  PipelineTransformFieldsRowVariant,
} from "@/components/transform/PipelineTransformFieldsRow";
import PipelineTransformFieldsStepCompute from "@/components/transform/PipelineTransformFieldsStepCompute";
import PipelineTransformFieldsStepDrop from "@/components/transform/PipelineTransformFieldsStepDrop";
import PipelineTransformFieldsStepFooter from "@/components/transform/PipelineTransformFieldsStepFooter";
import PipelineTransformFieldsStepHeader, {
  PipelineTransformFieldsStepSurface,
} from "@/components/transform/PipelineTransformFieldsStepHeader";
import PipelineTransformFieldsStepRename from "@/components/transform/PipelineTransformFieldsStepRename";
import PipelineTransformFieldsSubject from "@/components/transform/PipelineTransformFieldsSubject";
import {
  type PipelineTransformFieldsDraft,
  type PipelineTransformFieldsStepHandle,
  TransformStepKind,
} from "@/components/transform/types";

const BODY_INSET = TRANSFORM_HEADER_PADDING_X + TRANSFORM_HANDLE + TRANSFORM_GAP;

interface PipelineTransformFieldsStepCardProps {
  draft: PipelineTransformFieldsDraft;
  handle?: PipelineTransformFieldsStepHandle;
}

const PipelineTransformFieldsStepCard = ({
  draft,
  handle,
}: PipelineTransformFieldsStepCardProps) => {
  const { resources, isReadOnly } = usePipelineTransformFieldsEnvironment();
  const { setDraft, setResource, cancel, save, removeStep } = usePipelineTransformFieldsActions();
  const { editor, issues, warnings, typeSummary, isSaveDisabled } =
    usePipelineTransformFieldsValidation(draft);
  const resourceOptions: SelectInputOption[] = resources.map((resource) => ({
    id: resource,
    label: resource,
    value: resource,
  }));
  const { step } = draft;

  return (
    <PipelineTransformFieldsEditorProvider value={editor}>
      <PipelineTransformFieldsStepSurface $isHoverable>
        <PipelineTransformFieldsStepHeader isOpen onToggle={cancel} handle={handle}>
          <PipelineTransformFieldsSubject step={step} outputIndex={0} onChange={setDraft} />
        </PipelineTransformFieldsStepHeader>
        <FlexWrapper
          padding={`0 ${TRANSFORM_HEADER_PADDING_X}px ${TRANSFORM_HEADER_PADDING_X}px ${BODY_INSET}px`}
          fillWidth
        >
          <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.SMALL} fillWidth>
            {draft.id === null && resources.length > 1 && (
              <PipelineTransformFieldsRow
                variant={PipelineTransformFieldsRowVariant.STEP}
                gutter={
                  <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                    for
                  </Text>
                }
              >
                <SelectInput
                  options={resourceOptions}
                  value={resourceOptions.find((option) => option.id === draft.resource) ?? null}
                  onChange={(option) => setResource(option.id)}
                  placeholder="Choose a resource"
                  variant={InputVariant.TERTIARY}
                  size={InputSize.MEDIUM}
                  isDisabled={isReadOnly}
                  fillWidth
                />
              </PipelineTransformFieldsRow>
            )}
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
              onDelete={draft.id === null ? undefined : () => removeStep(draft.id ?? "")}
            />
          </FlexWrapper>
        </FlexWrapper>
      </PipelineTransformFieldsStepSurface>
    </PipelineTransformFieldsEditorProvider>
  );
};

export default PipelineTransformFieldsStepCard;
