import { useCallback, useEffect, useState } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import { useDebouncedValue } from "@galaxy-io/dls/hooks/useDebouncedValue";
import { ModalSize } from "@galaxy-io/dls/modal/Modal";

import CreateConnectionSelectorBody from "@/pages/connectors/components/create/select/CreateConnectionSelectorBody";
import CreateConnectionSelectorHeader from "@/pages/connectors/components/create/select/CreateConnectionSelectorHeader";
import type { CreateConnectionSelectorProps } from "@/pages/connectors/components/create/types";
import ConnectionFormWrapper from "@/pages/connectors/components/form/ConnectionFormWrapper";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/api/utils";

interface CreateConnectionSelectorState {
  search: string;
}

const DEFAULT_STATE: CreateConnectionSelectorState = {
  search: "",
};

const CreateConnectionSelector = ({
  onClose,
  onConnectorSelect,
}: CreateConnectionSelectorProps) => {
  const navigate = useNavigate();
  const { connectorSearch = "" } = useSearch({ from: "/_app" });

  const [state, setState] = useState<CreateConnectionSelectorState>(() => ({
    ...DEFAULT_STATE,
    search: connectorSearch,
  }));
  const debouncedSearch = useDebouncedValue(state.search, LIST_SEARCH_DEBOUNCE_MS);

  const handleSearchChange = useCallback((search: string) => {
    setState((prev) => ({ ...prev, search }));
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
      size={ModalSize.LARGE}
      header={
        <CreateConnectionSelectorHeader search={state.search} onSearchChange={handleSearchChange} />
      }
      onClose={onClose}
    >
      <CreateConnectionSelectorBody onConnectorSelect={onConnectorSelect} />
    </ConnectionFormWrapper>
  );
};

export default CreateConnectionSelector;
