import { useState } from "react";

import { TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Widget from "@galaxy-io/dls/widget/Widget";

import PipelineNotifierFields from "@/pages/pipelines/components/notifier/PipelineNotifierFields";
import type { PipelineNotifierState } from "@/pages/pipelines/components/notifier/types";
import { isPipelineNotifierValid } from "@/pages/pipelines/components/notifier/utils";

interface PipelineNotifierFormProps {
  initialState: PipelineNotifierState;
  isSaving?: boolean;
  onSave: (state: PipelineNotifierState) => void;
  onCancel: () => void;
  onDelete?: () => void;
}

const PipelineNotifierForm = ({
  initialState,
  isSaving = false,
  onSave,
  onCancel,
  onDelete,
}: PipelineNotifierFormProps) => {
  const [state, setState] = useState<PipelineNotifierState>(initialState);

  const handleChange = (partial: Partial<PipelineNotifierState>) => {
    setState((prev) => ({ ...prev, ...partial }));
  };

  const handleSave = () => {
    onSave({ ...state, isEnabled: initialState.isEnabled });
  };

  return (
    <FlexWrapper padding="12px" fillWidth>
      <Widget noHover padding="16px" fillWidth>
        <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
          <PipelineNotifierFields state={state} onChange={handleChange} isDisabled={isSaving} />
          <FlexWrapper
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.END}
            gap={8}
            fillWidth
          >
            {onDelete && (
              <Button
                label="Delete"
                icon={TrashIcon}
                variant={ButtonVariant.SECONDARY}
                size={ButtonSize.MEDIUM}
                onClick={onDelete}
                isDisabled={isSaving}
                ariaLabel="Delete notifier"
              />
            )}
            <Button
              label="Cancel"
              variant={ButtonVariant.SECONDARY}
              size={ButtonSize.MEDIUM}
              onClick={onCancel}
              isDisabled={isSaving}
            />
            <Button
              label="Save"
              size={ButtonSize.MEDIUM}
              onClick={handleSave}
              isDisabled={!isPipelineNotifierValid(state)}
              isLoading={isSaving}
            />
          </FlexWrapper>
        </FlexWrapper>
      </Widget>
    </FlexWrapper>
  );
};

export default PipelineNotifierForm;
