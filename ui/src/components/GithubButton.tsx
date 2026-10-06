import { GithubLogoIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import { GITHUB_REPO_URL } from "@/constants";

const GithubButton = () => {
  const handleGithub = () => {
    window.open(GITHUB_REPO_URL, "_blank");
  };

  return (
    <Button
      label="Star on GitHub"
      icon={GithubLogoIcon}
      variant={ButtonVariant.SECONDARY}
      size={ButtonSize.SMALL}
      onClick={handleGithub}
      /* @dls-migrate button.isIconFilled: Removed: pass the filled icon in `leading` at the rung's icon size. */ isIconFilled
    />
  );
};

export default GithubButton;
