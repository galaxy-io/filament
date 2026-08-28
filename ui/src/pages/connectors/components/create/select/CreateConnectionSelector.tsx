import { useEffect, useState } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { useDebouncedValue } from "@galaxy-io/dls/inputs/hooks";

import CreateConnectionSelectorBody from "@/pages/connectors/components/create/select/CreateConnectionSelectorBody";
import CreateConnectionSelectorHeader from "@/pages/connectors/components/create/select/CreateConnectionSelectorHeader";
import type { CreateConnectionSelectorProps } from "@/pages/connectors/components/create/types";
import ConnectionFormWrapper from "@/pages/connectors/components/form/ConnectionFormWrapper";
import { CREATE_CONNECTION_MODAL_SELECTOR_WIDTH } from "@/pages/connectors/constants";

import { LIST_SEARCH_DEBOUNCE_MS } from "@/api/utils";

const CreateConnectionSelector = ({
  onClose,
  onConnectorSelect,
}: CreateConnectionSelectorProps) => {
  const navigate = useNavigate();
  const { connectorSearch = "" } = useSearch({ from: "__root__" });

  const [search, setSearch] = useState(connectorSearch);
  const debouncedSearch = useDebouncedValue(search, LIST_SEARCH_DEBOUNCE_MS);

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
    <ConnectionFormWrapper width={CREATE_CONNECTION_MODAL_SELECTOR_WIDTH}>
      <FlexItem grow={0} shrink={0}>
        <CreateConnectionSelectorHeader
          search={search}
          onSearchChange={setSearch}
          onClose={onClose}
        />
      </FlexItem>
      <FlexItem grow={0} shrink={0}>
        <HorizontalDivider />
      </FlexItem>
      <CreateConnectionSelectorBody onConnectorSelect={onConnectorSelect} />
    </ConnectionFormWrapper>
  );
};

export default CreateConnectionSelector;
