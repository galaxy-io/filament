import { useCallback, useEffect, useState } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { useDebouncedValue } from "@galaxy-io/dls/hooks/useDebouncedValue";
import { ModalSize } from "@galaxy-io/dls/modal/Modal";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import CreateConnectionSelectorBody from "@/pages/connectors/components/create/select/CreateConnectionSelectorBody";
import {
  type CreateConnectionSelectorProps,
  CreateConnectionSelectorShelf,
} from "@/pages/connectors/components/create/types";
import ConnectionFormWrapper from "@/pages/connectors/components/form/ConnectionFormWrapper";
import { CONNECTOR_KIND_TO_CREATE_TITLE_MAP } from "@/pages/connectors/constants";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/api/utils";

interface CreateConnectionSelectorState {
  search: string;
  shelf: CreateConnectionSelectorShelf;
}

const DEFAULT_STATE: CreateConnectionSelectorState = {
  search: "",
  shelf: CreateConnectionSelectorShelf.ALL,
};

const CreateConnectionSelector = ({
  onClose,
  onConnectorSelect,
}: CreateConnectionSelectorProps) => {
  const navigate = useNavigate();
  const { connectorSearch = "", connectorKind } = useSearch({ from: "/_app" });
  const kind = connectorKind ?? ConnectorKind.UNSPECIFIED;

  const [state, setState] = useState<CreateConnectionSelectorState>(() => ({
    ...DEFAULT_STATE,
    search: connectorSearch,
  }));
  const debouncedSearch = useDebouncedValue(state.search, LIST_SEARCH_DEBOUNCE_MS);

  const handleSearchChange = useCallback((search: string) => {
    setState((prev) => ({ ...prev, search }));
  }, []);

  const handleShelfChange = useCallback((shelf: CreateConnectionSelectorShelf) => {
    setState((prev) => ({ ...prev, shelf }));
  }, []);

  useEffect(() => {
    if (debouncedSearch === connectorSearch) {
      return;
    }
    void navigate({
      to: ".",
      replace: true,
      search: (prev) => ({ ...prev, connectorSearch: debouncedSearch || undefined }),
    });
  }, [debouncedSearch, connectorSearch, navigate]);

  return (
    <ConnectionFormWrapper
      size={ModalSize.X_LARGE}
      header={CONNECTOR_KIND_TO_CREATE_TITLE_MAP[kind]}
      footer={<Button label="Cancel" variant={ButtonVariant.SECONDARY} onClick={onClose} />}
      onClose={onClose}
    >
      <CreateConnectionSelectorBody
        search={state.search}
        onSearchChange={handleSearchChange}
        shelf={state.shelf}
        onShelfChange={handleShelfChange}
        onConnectorSelect={onConnectorSelect}
      />
    </ConnectionFormWrapper>
  );
};

export default CreateConnectionSelector;
