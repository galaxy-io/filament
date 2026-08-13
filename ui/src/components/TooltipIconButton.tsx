import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

interface TooltipIconButtonProps {
  label: string;
  icon: PhosphorIcon;
  onClick: (event: React.MouseEvent<HTMLButtonElement>) => void;
  onMouseEnter?: (event: React.MouseEvent<HTMLButtonElement>) => void;
  variant?: ButtonVariant;
  size?: ButtonSize;
  isIconFilled?: boolean;
  isDisabled?: boolean;
  isTooltipDisabled?: boolean;
  ariaExpanded?: boolean;
  ariaControls?: string;
  ariaHasPopup?: boolean;
}

const TooltipIconButton = ({
  label,
  icon,
  onClick,
  onMouseEnter,
  variant = ButtonVariant.SECONDARY,
  size = ButtonSize.SMALL,
  isIconFilled = false,
  isDisabled = false,
  isTooltipDisabled = false,
  ariaExpanded,
  ariaControls,
  ariaHasPopup,
}: TooltipIconButtonProps) => {
  return (
    <Tooltip body={label} position={TooltipPosition.BOTTOM} isDisabled={isTooltipDisabled}>
      <Button
        icon={icon}
        variant={variant}
        size={size}
        onClick={onClick}
        onMouseEnter={onMouseEnter}
        isIconFilled={isIconFilled}
        isDisabled={isDisabled}
        ariaLabel={label}
        ariaExpanded={ariaExpanded}
        ariaControls={ariaControls}
        ariaHasPopup={ariaHasPopup}
      />
    </Tooltip>
  );
};

export default TooltipIconButton;
