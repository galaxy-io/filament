import type { FC } from "react";

import { WebhooksLogoIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import BulletedList, { BulletedListSize } from "@galaxy-io/dls/lists/BulletedList";

interface PipelinePageStreamButtonProps {
  runErrors: string[];
  isRunnable: boolean;
  isStarting: boolean;
  isCompact?: boolean;
  onClick: () => void;
}

const PipelinePageStreamButton: FC<PipelinePageStreamButtonProps> = ({
  runErrors,
  isRunnable,
  isStarting,
  isCompact = false,
  onClick,
}) => (
  <Button
    label="Start stream"
    icon={WebhooksLogoIcon}
    variant={ButtonVariant.SECONDARY}
    size={isCompact ? ButtonSize.SMALL : ButtonSize.MEDIUM}
    isLoading={isStarting}
    isDisabled={!isRunnable || runErrors.length > 0}
    tooltip={
      runErrors.length > 0 ? (
        <BulletedList items={runErrors} size={BulletedListSize.SMALL} />
      ) : isRunnable ? (
        "Stream webhook resources continuously"
      ) : (
        "Add a webhook resource to start a stream"
      )
    }
    onClick={onClick}
  />
);

export default PipelinePageStreamButton;
