import { GithubLogoIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { IconWeight } from "@galaxy-io/dls/icons/Icon";

import { GITHUB_REPO_URL } from "@/constants";

const GithubButton = () => {
  const handleGithub = () => {
    window.open(GITHUB_REPO_URL, "_blank");
  };

  return (
    <Button
      label="Star on GitHub"
      icon={GithubLogoIcon}
      iconWeight={IconWeight.FILL}
      variant={ButtonVariant.TERTIARY}
      size={ButtonSize.SMALL}
      onClick={handleGithub}
    />
  );
};

export default GithubButton;
