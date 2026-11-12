import { type FC, useCallback, useState } from "react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Modal from "@galaxy-io/dls/modal/Modal";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import CreateConnectionSelectorBody from "@/pages/connections/components/create/select/CreateConnectionSelectorBody";
import CreateConnectionSelectorSidebar from "@/pages/connections/components/create/select/CreateConnectionSelectorSidebar";
import { useCreateConnectionSelectorCatalog } from "@/pages/connections/components/create/select/hooks/useCreateConnectionSelectorCatalog";
import { CreateConnectionSelectorShelf } from "@/pages/connections/components/create/types";
import { CONNECTOR_KIND_TO_CREATE_TITLE_MAP } from "@/pages/connections/constants";

import { useFilamentSearchUpdate } from "@/module/hooks";
import type { FilamentLayoutSearch } from "@/module/schemas";

interface CreateConnectionSelectorProps {
  isOpen: boolean;
  connectorKind: ConnectorKind;
  connectorSearch: string;
  onClose: () => void;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

const CreateConnectionSelector: FC<CreateConnectionSelectorProps> = ({
  isOpen,
  connectorKind,
  connectorSearch,
  onClose,
  onConnectorSelect,
}) => {
  const updateSearch = useFilamentSearchUpdate<FilamentLayoutSearch>();

  const [shelf, setShelf] = useState(CreateConnectionSelectorShelf.ALL);
  const { familyCount, shelfItems, shelfFamilies, error, isLoading } =
    useCreateConnectionSelectorCatalog(connectorKind, connectorSearch, shelf);

  const handleSearch = useCallback(
    (search: string) => {
      void updateSearch((prev) => ({ ...prev, connectorSearch: search || undefined }), {
        replace: true,
      });
    },
    [updateSearch],
  );

  return (
    <Modal
      isOpen={isOpen}
      isFullscreen
      header={CONNECTOR_KIND_TO_CREATE_TITLE_MAP[connectorKind]}
      sidebar={
        <CreateConnectionSelectorSidebar
          connectorKind={connectorKind}
          shelfItems={shelfItems}
          shelf={shelf}
          onShelfChange={setShelf}
        />
      }
      footer={<Button label="Cancel" variant={ButtonVariant.SECONDARY} onClick={onClose} />}
      hasDividers
      isFlush
      onOpenChange={(isOpen) => {
        if (!isOpen) onClose();
      }}
    >
      <CreateConnectionSelectorBody
        connectorKind={connectorKind}
        connectorSearch={connectorSearch}
        familyCount={familyCount}
        shelfFamilies={shelfFamilies}
        isLoading={isLoading}
        error={error}
        onSearch={handleSearch}
        onConnectorSelect={onConnectorSelect}
      />
    </Modal>
  );
};

export default CreateConnectionSelector;
