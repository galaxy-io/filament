import { styled } from "@linaria/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";

import type { ProviderSpec } from "@/gen/ingestion/v1/providers_pb";

import BaseHeader from "@/layouts/components/BaseHeader";

import ProviderTile, {
  ProviderTileSize,
} from "@/pages/providers/components/ProviderTile";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";

const HeaderWrapper = styled.div`
  padding: 12px 16px;
`;

interface ProviderDrawerHeaderProps {
  provider: ProviderSpec;
  onClose: () => void;
}

const ProviderDrawerHeader = ({
  provider,
  onClose,
}: ProviderDrawerHeaderProps) => {
  return (
    <HeaderWrapper>
      <FlexWrapper
        fillWidth
        alignItems={AlignItems.CENTER}
        gap={FlexGap.MEDIUM}
      >
        <FlexItem shrink={0}>
          <ProviderTile
            provider={provider.name}
            size={ProviderTileSize.LARGE}
          />
        </FlexItem>
        <FlexWrapper
          fillWidth
          direction={FlexDirection.COLUMN}
          gap={FlexGap.XSMALL}
        >
          <BaseHeader
            title={provider.displayName}
            description={provider.name}
            onClose={onClose}
          />
        </FlexWrapper>
      </FlexWrapper>
    </HeaderWrapper>
  );
};

export default ProviderDrawerHeader;
