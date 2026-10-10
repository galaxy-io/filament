import { type FC, useCallback, useState } from "react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { ModalSize } from "@galaxy-io/dls/modal/Modal";

import CreateConnectionSelectorBody from "@/pages/connections/components/create/select/CreateConnectionSelectorBody";
import {
  type CreateConnectionSelectorProps,
  CreateConnectionSelectorShelf,
} from "@/pages/connections/components/create/types";
import ConnectionFormWrapper from "@/pages/connections/components/form/ConnectionFormWrapper";
import { CONNECTOR_KIND_TO_CREATE_TITLE_MAP } from "@/pages/connections/constants";

import { useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";

const CreateConnectionSelector: FC<CreateConnectionSelectorProps> = ({
  isOpen,
  connectorKind,
  connectorSearch,
  onClose,
  onConnectorSelect,
}) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();

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
      isOpen={isOpen}
      size={ModalSize.X_LARGE}
      header={CONNECTOR_KIND_TO_CREATE_TITLE_MAP[connectorKind]}
      footer={<Button label="Cancel" variant={ButtonVariant.SECONDARY} onClick={onClose} />}
      onClose={onClose}
    >
      <CreateConnectionSelectorBody
        connectorKind={connectorKind}
        connectorSearch={connectorSearch}
        onSearch={handleSearch}
        shelf={shelf}
        onShelfChange={setShelf}
        onConnectorSelect={onConnectorSelect}
      />
    </ConnectionFormWrapper>
  );
};

export default CreateConnectionSelector;
