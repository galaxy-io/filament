import { type FC, useState } from "react";

import { TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Widget, { WidgetSize } from "@galaxy-io/dls/widget/Widget";

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

const PipelineNotifierForm: FC<PipelineNotifierFormProps> = ({
  initialState,
  isSaving = false,
  onSave,
  onCancel,
  onDelete,
}) => {
  const [state, setState] = useState<PipelineNotifierState>(initialState);

  const handleChange = (partial: Partial<PipelineNotifierState>) => {
    setState((prev) => ({ ...prev, ...partial }));
  };

  const handleSave = () => {
    onSave({ ...state, isEnabled: initialState.isEnabled });
  };

  return (
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} padding={12} fillWidth>
      <Widget size={WidgetSize.LARGE} gap={12}>
        <PipelineNotifierFields state={state} onChange={handleChange} isDisabled={isSaving} />
        <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.END} gap={8} fillWidth>
          {onDelete && (
            <Button
              label="Delete"
              icon={TrashIcon}
              variant={ButtonVariant.SECONDARY}
              onClick={onDelete}
              isDisabled={isSaving}
              ariaLabel="Delete notifier"
            />
          )}
          <Button
            label="Cancel"
            variant={ButtonVariant.SECONDARY}
            onClick={onCancel}
            isDisabled={isSaving}
          />
          <Button
            label="Save"
            onClick={handleSave}
            isDisabled={!isPipelineNotifierValid(state)}
            isLoading={isSaving}
          />
        </Flex>
      </Widget>
    </Flex>
  );
};

export default PipelineNotifierForm;
