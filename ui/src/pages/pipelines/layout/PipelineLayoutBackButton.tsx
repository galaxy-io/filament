import { styled } from "@linaria/react";
import { ArrowLeftIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { PIPELINE_NAVBAR_HEIGHT } from "@/pages/pipelines/layout/constants";

const PipelineLayoutBackButtonWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${PIPELINE_NAVBAR_HEIGHT}px;

  display: flex;
  align-items: center;
  justify-content: center;

  flex-shrink: 0;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const PipelineLayoutBackButton = () => {
  const navigate = useNavigate();

  const handleBack = () => {
    navigate({ to: "/" });
  };

  return (
    <PipelineLayoutBackButtonWrapper>
      <Button
        variant={ButtonVariant.SECONDARY}
        size={ButtonSize.SMALL}
        icon={ArrowLeftIcon}
        onClick={handleBack}
      />
    </PipelineLayoutBackButtonWrapper>
  );
};

export default PipelineLayoutBackButton;
