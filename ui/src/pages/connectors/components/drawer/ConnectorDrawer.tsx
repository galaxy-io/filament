import { styled } from "@linaria/react";

import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

import ConnectorDrawerDetails from "@/pages/connectors/components/drawer/ConnectorDrawerDetails";
import ConnectorDrawerFooter from "@/pages/connectors/components/drawer/ConnectorDrawerFooter";
import ConnectorDrawerHeader from "@/pages/connectors/components/drawer/ConnectorDrawerHeader";
import ConnectorDrawerMetrics from "@/pages/connectors/components/drawer/ConnectorDrawerMetrics";

const DrawerWrapper = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;

  width: 100%;
  height: 100%;

  overflow: hidden;

  background-color: ${({ theme }) => theme.color.background.primary};
`);

const DrawerBody = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;
  gap: 12px;

  flex: 1;

  padding: 16px;

  min-height: 0;

  overflow-y: auto;

  background-color: ${({ theme }) => theme.color.background.base};
`);

interface ConnectorDrawerProps {
  connector: ConnectorSpec;
  onClose: () => void;
}

const ConnectorDrawer = ({ connector, onClose }: ConnectorDrawerProps) => {
  const handleDelete = () => {
    // TODO: Implement delete confirmation dialog
  };

  return (
    <DrawerWrapper>
      <ConnectorDrawerHeader connector={connector} onClose={onClose} />

      <HorizontalDivider />

      <DrawerBody>
        <ConnectorDrawerMetrics connector={connector} />

        <ConnectorDrawerDetails connector={connector} />

        {/* Spacer */}
        <FlexWrapper fillWidth fillHeight />

        <ConnectorDrawerFooter onDelete={handleDelete} />
      </DrawerBody>
    </DrawerWrapper>
  );
};

export default ConnectorDrawer;
