import { styled } from "@linaria/react";

import Text, { TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

const PageWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: 100%;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme }) => theme.color.background.primary};
`);

const PipelineHistoryPage = () => {
  return (
    <PageWrapper>
      <Text variant={TextVariant.TERTIARY}>Run History - Coming Soon</Text>
    </PageWrapper>
  );
};

export default PipelineHistoryPage;
