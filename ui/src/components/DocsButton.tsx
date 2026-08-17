import { ArrowUpRightIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import { DOCUMENTATION_URL } from "@/constants";

interface DocsButtonProps {
  label?: string;
  path?: string;
  variant?: ButtonVariant;
  size?: ButtonSize;
}

const DocsButton = ({
  label = "Docs",
  path = "/",
  variant = ButtonVariant.SECONDARY,
  size = ButtonSize.SMALL,
}: DocsButtonProps) => {
  const handleDocs = () => {
    window.open(`${DOCUMENTATION_URL}${path}`, "_blank");
  };

  return (
    <Button
      label={label}
      icon={ArrowUpRightIcon}
      variant={variant}
      size={size}
      onClick={handleDocs}
      isIconTrailing
    />
  );
};

export default DocsButton;
