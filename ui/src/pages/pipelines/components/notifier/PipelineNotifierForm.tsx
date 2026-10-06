import { useState } from "react";

import { TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
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
    <Flex alignItems={AlignItems.START} padding={12} fillWidth>
      <Widget /* @dls-migrate widget.padding-other: The body inset is fixed at 12px: remove `padding` (use `isFlush` for 0). */
        padding="16px" /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */
        fillWidth
      >
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} fillWidth>
          <PipelineNotifierFields state={state} onChange={handleChange} isDisabled={isSaving} />
          <Flex
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
          </Flex>
        </Flex>
      </Widget>
    </Flex>
  );
};

export default PipelineNotifierForm;
