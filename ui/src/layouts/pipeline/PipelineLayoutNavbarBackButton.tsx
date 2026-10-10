import type { FC } from "react";

import { ArrowLeftIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";

import { PIPELINE_NAVBAR_HEIGHT } from "@/layouts/pipeline/constants";

import { useFilamentNavigate } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";

const PipelineLayoutNavbarBackButton: FC = () => {
  const navigate = useFilamentNavigate();

  const handleBack = () => {
    navigate({ to: FilamentPath.PIPELINES });
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
