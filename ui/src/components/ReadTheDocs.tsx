import { styled } from "@linaria/react";
import { ArrowUpRightIcon } from "@phosphor-icons/react";

import FlexWrapper, {
  AlignItems,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { type PropsWithTheme, withTheme } from "@galaxy-io/dls/theme";

import { DOCUMENTATION_URL } from "@/constants";
import { useState } from "react";

const TextWraper = withTheme(styled.div<
  PropsWithTheme<{ $isHovered: boolean }>
>`
  display: flex;
  align-items: center;
  justify-content: center;
  border-bottom: 0.5px dashed
    ${({ theme, $isHovered }) =>
      $isHovered ? theme.color.text.primary : theme.color.text.tertiary};
`);

interface ReadTheDocsProps {
  path: string;
}

interface ReadTheDocsState {
  isHovered: boolean;
}

const DEFAULT_STATE: ReadTheDocsState = {
  isHovered: false,
};

const ReadTheDocs = ({ path }: ReadTheDocsProps) => {
  const [state, setState] = useState<ReadTheDocsState>(DEFAULT_STATE);

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
      <TextWraper $isHovered={state.isHovered}>
        <Text
          variant={
            state.isHovered ? TextVariant.PRIMARY : TextVariant.SECONDARY
          }
        >
          Read the docs
        </Text>
      </TextWraper>
      <Icon
        component={ArrowUpRightIcon}
        variant={state.isHovered ? IconVariant.PRIMARY : IconVariant.SECONDARY}
        size={12}
      />
    </FlexWrapper>
  );
};

export default ReadTheDocs;
