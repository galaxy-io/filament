import { useState } from "react";

import { ArrowUpRightIcon } from "@phosphor-icons/react";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";

import { DOCUMENTATION_URL } from "@/constants";

interface DocsButtonProps {
  path: string;
}

interface DocsButtonState {
  isHovered: boolean;
}

const DEFAULT_STATE: DocsButtonState = {
  isHovered: false,
};

const DocsButton = ({ path }: DocsButtonProps) => {
  const [state, setState] = useState<DocsButtonState>(DEFAULT_STATE);

  const handleDocs = () => {
    window.open(`${DOCUMENTATION_URL}${path}`, "_blank");
  };

  const handleMouseEnter = () => {
    setState({ ...state, isHovered: true });
  };

  const handleMouseLeave = () => {
    setState({ ...state, isHovered: false });
  };

  return (
    <FlexWrapper
      alignItems={AlignItems.CENTER}
      gap={FlexGap.XSMALL}
      onClick={handleDocs}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      <Text variant={state.isHovered ? TextVariant.PRIMARY : TextVariant.SECONDARY}>Docs</Text>
      <Icon
        component={ArrowUpRightIcon}
        variant={state.isHovered ? IconVariant.PRIMARY : IconVariant.SECONDARY}
        size={12}
      />
    </FlexWrapper>
  );
};

export default DocsButton;
