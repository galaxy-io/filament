import { useState } from "react";

import { styled } from "@linaria/react";
import { GithubLogoIcon } from "@phosphor-icons/react";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import { type PropsWithTheme, withTheme } from "@galaxy-io/dls/theme";

import { GITHUB_REPO_URL } from "@/constants";

const IconWrapper = withTheme(styled.div<PropsWithTheme<{ $isHovered: boolean }>>`
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
`);

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
    <IconWrapper $isHovered={state.isHovered}>
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        gap={FlexGap.XSMALL}
        onClick={handleGithub}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={handleMouseLeave}
      >
        <Icon
          component={GithubLogoIcon}
          weight={IconWeight.FILL}
          variant={state.isHovered ? IconVariant.PRIMARY : IconVariant.TERTIARY}
        />
      </FlexWrapper>
    </IconWrapper>
  );
};

export default GithubButton;
