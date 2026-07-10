import { styled } from "@linaria/react";

import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import type { ProviderSpec } from "@/gen/ingestion/v1/providers_pb";

import ProviderDrawerDetails from "@/pages/providers/components/drawer/ProviderDrawerDetails";
import ProviderDrawerFooter from "@/pages/providers/components/drawer/ProviderDrawerFooter";
import ProviderDrawerHeader from "@/pages/providers/components/drawer/ProviderDrawerHeader";
import ProviderDrawerMetrics from "@/pages/providers/components/drawer/ProviderDrawerMetrics";

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

interface ProviderDrawerProps {
  provider: ProviderSpec;
  onClose: () => void;
}

const ProviderDrawer = ({ provider, onClose }: ProviderDrawerProps) => {
  const handleDelete = () => {
    // TODO: Implement delete confirmation dialog
  };

  return (
    <DrawerWrapper>
      <ProviderDrawerHeader provider={provider} onClose={onClose} />

      <HorizontalDivider />

      <DrawerBody>
        <ProviderDrawerMetrics provider={provider} />

        <ProviderDrawerDetails provider={provider} />

        {/* Spacer */}
        <FlexWrapper fillWidth fillHeight />

        <ProviderDrawerFooter onDelete={handleDelete} />
      </DrawerBody>
    </DrawerWrapper>
  );
};

export default ProviderDrawer;
