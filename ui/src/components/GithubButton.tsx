import { useState } from "react";

import { GithubLogoIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import { GITHUB_REPO_URL } from "@/constants";

interface GithubButtonState {
  isHovered: boolean;
}

const DEFAULT_STATE: GithubButtonState = {
  isHovered: false,
};

const GithubButton = () => {
  const [state, setState] = useState<GithubButtonState>(DEFAULT_STATE);

  const handleGithub = () => {
    window.open(GITHUB_REPO_URL, "_blank");
  };

  const handleMouseEnter = () => {
    setState({ ...state, isHovered: true });
  };

  const handleMouseLeave = () => {
    setState({ ...state, isHovered: false });
  };

  return (
    <Button
      icon={GithubLogoIcon}
      variant={ButtonVariant.SECONDARY}
      size={ButtonSize.SMALL}
      onClick={handleGithub}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    />
  );
};

export default GithubButton;
