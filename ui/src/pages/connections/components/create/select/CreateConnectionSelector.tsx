import { type FC, useCallback, useState } from "react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { ModalSize } from "@galaxy-io/dls/modal/Modal";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import CreateConnectionSelectorBody from "@/pages/connections/components/create/select/CreateConnectionSelectorBody";
import {
  type CreateConnectionSelectorProps,
  CreateConnectionSelectorShelf,
} from "@/pages/connections/components/create/types";
import ConnectionFormWrapper from "@/pages/connections/components/form/ConnectionFormWrapper";
import { CONNECTOR_KIND_TO_CREATE_TITLE_MAP } from "@/pages/connections/constants";

import { useFilamentLayoutSearch, useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";

const CreateConnectionSelector: FC<CreateConnectionSelectorProps> = ({
  onClose,
  onConnectorSelect,
}) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();
  const { connectorKind } = useFilamentLayoutSearch();
  const kind = connectorKind ?? ConnectorKind.UNSPECIFIED;

  const [shelf, setShelf] = useState(CreateConnectionSelectorShelf.ALL);

  const handleSearch = useCallback(
    (search: string) => {
      void updateSearch((prev) => ({ ...prev, connectorSearch: search || undefined }), {
        replace: true,
      });
    },
    [updateSearch],
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
