import { GithubLogoIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";

import { GITHUB_REPO_URL } from "@/constants";

const GithubButton = () => {
  const handleGithub = () => {
    window.open(GITHUB_REPO_URL, "_blank");
  };

  return (
    <Button
      label="Star on GitHub"
      leading={
        <Icon
          component={GithubLogoIcon}
          size={12}
          weight={IconWeight.FILL}
          variant={IconVariant.INHERIT}
        />
      }
      variant={ButtonVariant.TERTIARY}
      size={ButtonSize.SMALL}
      onClick={handleGithub}
    />
  );
};

export default GithubButton;
