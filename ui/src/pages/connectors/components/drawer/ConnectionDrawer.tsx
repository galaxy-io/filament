import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";

import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import ConnectionDrawerDetails from "@/pages/connectors/components/drawer/ConnectionDrawerDetails";
import ConnectionDrawerFooter from "@/pages/connectors/components/drawer/ConnectionDrawerFooter";
import ConnectionDrawerHeader from "@/pages/connectors/components/drawer/ConnectionDrawerHeader";
import ConnectionDrawerMetrics from "@/pages/connectors/components/drawer/ConnectionDrawerMetrics";

import { useDeleteConnectionMutation } from "@/api/queries/connectors";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

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

interface ConnectionDrawerProps {
  connection: Connection;
  onClose: () => void;
}

const ConnectionDrawer = ({ connection, onClose }: ConnectionDrawerProps) => {
  const navigate = useNavigate();
  const deleteConnection = useDeleteConnectionMutation();

  const handleDelete = () => {
    deleteConnection.mutate(
      { id: connection.id },
      {
        onSuccess: () => {
          onClose();
          navigate({ to: "/connectors" });
        },
      },
    );
  };

  return (
    <DrawerWrapper>
      <ConnectionDrawerHeader connection={connection} onClose={onClose} />

      <HorizontalDivider />

      <DrawerBody>
        <ConnectionDrawerMetrics connection={connection} />

        <ConnectionDrawerDetails connection={connection} />

        {/* Spacer */}
        <FlexWrapper fillWidth fillHeight />

        <ConnectionDrawerFooter onDelete={handleDelete} />
      </DrawerBody>
    </DrawerWrapper>
  );
};

export default ConnectionDrawer;
