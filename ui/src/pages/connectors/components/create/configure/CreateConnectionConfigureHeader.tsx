import { styled } from "@linaria/react";
import { ArrowLeftIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";

import BaseHeader from "@/layouts/components/BaseHeader";
import { BaseHeaderSize } from "@/layouts/components/types";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

const HeaderWrapper = styled.div`
  flex: 1;
  min-width: 0;
`;

interface CreateConnectionConfigureHeaderProps {
  connector: ConnectorSpec;
  onClose: () => void;
  onBack: () => void;
}

const CreateConnectionConfigureHeader = ({
  connector,
  onClose,
  onBack,
}: CreateConnectionConfigureHeaderProps) => {
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} padding="16px" gap={12} fillWidth>
      <Button
        icon={ArrowLeftIcon}
        variant={ButtonVariant.TERTIARY}
        size={ButtonSize.SMALL}
        onClick={onBack}
      />
      <FlexItem shrink={0} grow={0}>
        <ConnectorTile connector={connector.name} size={ConnectorTileSize.MEDIUM} />
      </FlexItem>
      <HeaderWrapper>
        <BaseHeader
          size={BaseHeaderSize.LARGE}
          title={`New ${connector.displayName || connector.name} connection`}
          description="Configure your connection settings"
          onClose={onClose}
        />
      </HeaderWrapper>
    </FlexWrapper>
  );
};

export default CreateConnectionConfigureHeader;
