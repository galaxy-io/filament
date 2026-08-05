import { styled } from "@linaria/react";
import { ArrowLeftIcon, ArrowRightIcon, PlusIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { CreatePipelineModalStep } from "@/pages/pipelines/components/create/types";

import { NOOP } from "@/constants";

const FooterWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background-color: ${({ theme }) => theme.color.background.primary};
`);

interface CreatePipelineModalFooterProps {
  step: CreatePipelineModalStep;
  hasSelection: boolean;
  isNextDisabled: boolean;
  isSubmitting: boolean;
  onBack: () => void;
  onNext: () => void;
  onCreate: () => void;
}

const CreatePipelineModalFooter = ({
  step,
  hasSelection,
  isNextDisabled,
  isSubmitting,
  onBack,
  onNext,
  onCreate,
}: CreatePipelineModalFooterProps) => {
  const renderAction = () => {
    return match(step)
      .with(CreatePipelineModalStep.CONNECTIONS, () => (
        <Button
          size={ButtonSize.LARGE}
          label={hasSelection ? "Next" : "Skip"}
          icon={ArrowRightIcon}
          onClick={onNext}
          isIconTrailing
        />
      ))
      .with(CreatePipelineModalStep.DETAILS, () => (
        <Button
          size={ButtonSize.LARGE}
          label="Next"
          icon={ArrowRightIcon}
          onClick={onNext}
          isDisabled={isNextDisabled}
          isIconTrailing
        />
      ))
      .with(CreatePipelineModalStep.SCHEDULE, () =>
        isSubmitting ? (
          <Button size={ButtonSize.LARGE} label="Creating..." onClick={NOOP} isLoading isDisabled />
        ) : (
          <Button
            size={ButtonSize.LARGE}
            label="Create pipeline"
            icon={PlusIcon}
            onClick={onCreate}
            isDisabled={isNextDisabled}
          />
        ),
      )
      .exhaustive();
  };

  return (
    <FooterWrapper>
      {step === CreatePipelineModalStep.CONNECTIONS ? (
        <div />
      ) : (
        <Button
          size={ButtonSize.LARGE}
          label="Back"
          icon={ArrowLeftIcon}
          variant={ButtonVariant.SECONDARY}
          onClick={onBack}
          isDisabled={isSubmitting}
        />
      )}
      {renderAction()}
    </FooterWrapper>
  );
};

export default CreatePipelineModalFooter;
