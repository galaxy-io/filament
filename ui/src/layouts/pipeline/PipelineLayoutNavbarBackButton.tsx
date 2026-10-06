import { styled } from "@linaria/react";
import { ArrowLeftIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { PIPELINE_NAVBAR_HEIGHT } from "@/layouts/pipeline/constants";

const PipelineLayoutNavbarBackButtonWrapper = styled.div`
  width: 100%;
  height: ${PIPELINE_NAVBAR_HEIGHT}px;

  display: flex;
  align-items: center;
  justify-content: center;

  flex-shrink: 0;

  background-color: ${t.color.background.base};
`;

const PipelineLayoutNavbarBackButton = () => {
  const navigate = useNavigate();

  const handleBack = () => {
    navigate({ to: "/pipelines" });
  };

  return (
    <PipelineLayoutNavbarBackButtonWrapper>
      <Button
        variant={ButtonVariant.SECONDARY}
        size={ButtonSize.SMALL}
        icon={ArrowLeftIcon}
        ariaLabel="Back to pipelines"
        tooltip="Back to pipelines"
        onClick={handleBack}
      />
    </PipelineLayoutNavbarBackButtonWrapper>
  );
};

export default PipelineLayoutNavbarBackButton;
