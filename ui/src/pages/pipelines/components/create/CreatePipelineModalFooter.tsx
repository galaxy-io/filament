import { styled } from "@linaria/react";
import { ArrowLeftIcon, ArrowRightIcon, PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import { NOOP } from "@/constants";

const FooterWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

interface CreatePipelineModalFooterProps {
  isBackVisible: boolean;
  isLastStep: boolean;
  isNextDisabled: boolean;
  isSubmitting: boolean;
  hint?: string;
  onBack: () => void;
  onNext: () => void;
  onCreate: () => void;
}

const CreatePipelineModalFooter = ({
  isBackVisible,
  isLastStep,
  isNextDisabled,
  isSubmitting,
  hint,
  onBack,
  onNext,
  onCreate,
}: CreatePipelineModalFooterProps) => {
  const renderAction = () => {
    if (isSubmitting) {
      return (
        <Button size={ButtonSize.LARGE} label="Creating..." onClick={NOOP} isLoading isDisabled />
      );
    }

    if (isLastStep) {
      return (
        <Button
          size={ButtonSize.LARGE}
          label="Create pipeline"
          icon={PlusIcon}
          onClick={onCreate}
          isDisabled={isNextDisabled}
        />
      );
    }

    return (
      <Button
        size={ButtonSize.LARGE}
        label="Next"
        icon={ArrowRightIcon}
        onClick={onNext}
        isDisabled={isNextDisabled}
        isIconTrailing
      />
    );
  };

  return (
    <FooterWrapper>
      <Button
        size={ButtonSize.LARGE}
        label="Back"
        icon={ArrowLeftIcon}
        variant={ButtonVariant.SECONDARY}
        onClick={onBack}
        isDisabled={isSubmitting || !isBackVisible}
      />
      <Tooltip body={hint ?? ""} position={TooltipPosition.TOP} isDisabled={!hint}>
        {renderAction()}
      </Tooltip>
    </FooterWrapper>
  );
};

export default CreatePipelineModalFooter;
