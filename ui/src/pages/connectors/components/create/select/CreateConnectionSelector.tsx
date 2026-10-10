import { type FC, useCallback, useState } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { ModalSize } from "@galaxy-io/dls/modal/Modal";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import CreateConnectionSelectorBody from "@/pages/connectors/components/create/select/CreateConnectionSelectorBody";
import {
  type CreateConnectionSelectorProps,
  CreateConnectionSelectorShelf,
} from "@/pages/connectors/components/create/types";
import ConnectionFormWrapper from "@/pages/connectors/components/form/ConnectionFormWrapper";
import { CONNECTOR_KIND_TO_CREATE_TITLE_MAP } from "@/pages/connectors/constants";

const CreateConnectionSelector: FC<CreateConnectionSelectorProps> = ({
  onClose,
  onConnectorSelect,
}) => {
  const navigate = useNavigate();
  const { connectorKind } = useSearch({ from: "/_app" });
  const kind = connectorKind ?? ConnectorKind.UNSPECIFIED;

  const [shelf, setShelf] = useState(CreateConnectionSelectorShelf.ALL);

  const handleSearch = useCallback(
    (search: string) => {
      void navigate({
        to: ".",
        replace: true,
        search: (prev) => ({ ...prev, connectorSearch: search || undefined }),
      });
    },
    [navigate],
  );

  return (
    <ConnectionFormWrapper
      size={ModalSize.X_LARGE}
      header={CONNECTOR_KIND_TO_CREATE_TITLE_MAP[kind]}
      footer={<Button label="Cancel" variant={ButtonVariant.SECONDARY} onClick={onClose} />}
      onClose={onClose}
    >
      <CreateConnectionSelectorBody
        onSearch={handleSearch}
        shelf={shelf}
        onShelfChange={setShelf}
        onConnectorSelect={onConnectorSelect}
      />
    </ConnectionFormWrapper>
  );
};

export default CreateConnectionSelector;
