import { styled } from "@linaria/react";

import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const CardWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;

  display: flex;
  flex-direction: column;

  background-color: ${({ theme }) => theme.color.background.primary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 5px;
`);

const CardSection = styled.div`
  display: flex;
  flex-direction: column;
  gap: 12px;

  padding: 16px;
`;

const ConnectionCardLoading = () => {
  return (
    <CardWrapper>
      <CardSection>
        <FlexWrapper
          fillWidth
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
        >
          <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
            <TextShimmer width={32} height={32} />
            <TextShimmer width={120} height={20} />
          </FlexWrapper>
          <TextShimmer width={56} height={24} />
        </FlexWrapper>
        <TextShimmer width={90} height={24} />
      </CardSection>
    </CardWrapper>
  );
};

export default ConnectionCardLoading;
