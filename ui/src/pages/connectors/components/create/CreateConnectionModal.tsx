import { useCallback } from "react";

import { create } from "@bufbuild/protobuf";
import { useNavigate, useSearch } from "@tanstack/react-router";

import { type ConnectorSpec, GetConnectorRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import { useGetConnectorQuery } from "@/api/queries/connectors";

import CreateConnectionConfigure from "@/pages/connectors/components/create/CreateConnectionConfigure";
import CreateConnectionSelector from "@/pages/connectors/components/create/select/CreateConnectionSelector";
import type { CreateConnectionModalProps } from "@/pages/connectors/components/create/types";

const CreateConnectionModal = ({ onClose }: CreateConnectionModalProps) => {
  const navigate = useNavigate();
  const { connector, connectorKind } = useSearch({
    from: "__root__",
  });

  const { data } = useGetConnectorQuery({
    input: create(GetConnectorRequestSchema, { connector, kind: connectorKind }),
    options: { enabled: !!connector && !!connectorKind },
  });
  const selectedConnector = data?.connector;

  const handleConnectorSelect = useCallback(
    (connector: ConnectorSpec) => {
      void navigate({
        to: ".",
        search: (prev) => ({
          ...prev,
          connector: connector.name,
          connectorKind: connector.kind,
        }),
      });
    },
    [navigate],
  );

  const handleBack = useCallback(() => {
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        connector: undefined,
      }),
    });
  }, [navigate]);

  if (selectedConnector) {
    return (
      <CreateConnectionConfigure
        connector={selectedConnector}
        onClose={onClose}
        onBack={handleBack}
      />
    );
  }

  return <CreateConnectionSelector onClose={onClose} onConnectorSelect={handleConnectorSelect} />;
};

export default CreateConnectionModal;
