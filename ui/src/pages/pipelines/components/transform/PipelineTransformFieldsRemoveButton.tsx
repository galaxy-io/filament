import type { FC } from "react";

import { XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import { usePipelineTransformFieldsEditor } from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";

interface PipelineTransformFieldsRemoveButtonProps {
  label: string;
  onClick: () => void;
}

const PipelineTransformFieldsRemoveButton: FC<PipelineTransformFieldsRemoveButtonProps> = ({
  label,
  onClick,
}) => {
  const { isDisabled } = usePipelineTransformFieldsEditor();
  return (
    <Button
      icon={XIcon}
      variant={ButtonVariant.TERTIARY}
      size={ButtonSize.MEDIUM}
      onClick={onClick}
      isDisabled={isDisabled}
      ariaLabel={label}
      tooltip={label}
    />
  );
};

export default PipelineTransformFieldsRemoveButton;
