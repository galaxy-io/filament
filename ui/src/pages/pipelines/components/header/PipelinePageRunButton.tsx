import type { FC } from "react";

import { PlayIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { IconWeight } from "@galaxy-io/dls/icons/Icon";
import BulletedList, { BulletedListSize } from "@galaxy-io/dls/lists/BulletedList";

interface PipelinePageRunButtonProps {
  runErrors: string[];
  isRunnable: boolean;
  onClick: () => void;
}

const PipelinePageRunButton: FC<PipelinePageRunButtonProps> = ({
  runErrors,
  isRunnable,
  onClick,
}) => (
  <Button
    label="Run"
    icon={PlayIcon}
    iconWeight={IconWeight.FILL}
    variant={ButtonVariant.PRIMARY}
    isDisabled={!isRunnable || runErrors.length > 0}
    tooltip={
      runErrors.length > 0 ? <BulletedList items={runErrors} size={BulletedListSize.SMALL} /> : ""
    }
    onClick={onClick}
  />
);

export default PipelinePageRunButton;
