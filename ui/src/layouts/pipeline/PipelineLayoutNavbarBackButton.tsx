import type { FC } from "react";

import { ArrowLeftIcon } from "@phosphor-icons/react";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";

import { PIPELINE_NAVBAR_HEIGHT } from "@/layouts/pipeline/constants";

const PipelineLayoutNavbarBackButton: FC = () => {
  const navigate = useNavigate();

  const handleBack = () => {
    navigate({ to: "/pipelines" });
  };

  return (
    <Flex
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
      fillWidth
      height={PIPELINE_NAVBAR_HEIGHT}
      shrink={0}
    >
      <Button
        variant={ButtonVariant.SECONDARY}
        size={ButtonSize.SMALL}
        icon={ArrowLeftIcon}
        ariaLabel="Back to pipelines"
        onClick={handleBack}
      />
    </Flex>
  );
};

export default PipelineLayoutNavbarBackButton;
