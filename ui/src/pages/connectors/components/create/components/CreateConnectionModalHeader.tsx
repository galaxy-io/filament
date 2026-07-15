import { ArrowLeftIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";

import BaseHeader from "@/layouts/components/BaseHeader";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";

interface CreateConnectionModalHeaderProps {
  connector: ConnectorSpec;
  onClose: () => void;
  onBack: () => void;
}

const CreateConnectionModalHeader = ({
  connector,
  onClose,
  onBack,
}: CreateConnectionModalHeaderProps) => {
  return (
    <FlexWrapper alignItems={AlignItems.CENTER} padding="16px" gap={12} fillWidth>
      <Button
        icon={ArrowLeftIcon}
        variant={ButtonVariant.TERTIARY}
        size={ButtonSize.SMALL}
        onClick={onBack}
      />
      <FlexWrapper alignItems={AlignItems.CENTER} gap={12} fillWidth>
        <FlexItem shrink={0}>
          <ConnectorTile connector={connector.name} size={ConnectorTileSize.MEDIUM} />
        </FlexItem>
        <BaseHeader
          title={`New ${connector.displayName || connector.name} connection`}
          description="Configure your connection settings"
          onClose={onClose}
        />
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default CreateConnectionModalHeader;
