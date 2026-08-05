import { useCallback, useState } from "react";

import { useSearch } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import CreateConnectionSelectorBody from "@/pages/connectors/components/create/select/CreateConnectionSelectorBody";
import CreateConnectionSelectorHeader from "@/pages/connectors/components/create/select/CreateConnectionSelectorHeader";
import type { CreateConnectionSelectorProps } from "@/pages/connectors/components/create/types";
import ConnectionFormWrapper from "@/pages/connectors/components/form/ConnectionFormWrapper";
import { CREATE_CONNECTION_MODAL_SELECTOR_WIDTH } from "@/pages/connectors/constants";

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
  const { connectorKind } = useSearch({ from: "__root__" });

  const [state, setState] = useState<CreateConnectionSelectorState>(DEFAULT_STATE);

  const handleSearchChange = useCallback((search: string) => {
    setState((prev) => ({ ...prev, search }));
  }, []);

  return (
    <ConnectionFormWrapper width={CREATE_CONNECTION_MODAL_SELECTOR_WIDTH}>
      <FlexItem grow={0} shrink={0}>
        <CreateConnectionSelectorHeader
          search={state.search}
          onSearchChange={handleSearchChange}
          onClose={onClose}
        />
      </FlexItem>
      <FlexItem grow={0} shrink={0}>
        <HorizontalDivider />
      </FlexItem>
      <CreateConnectionSelectorBody
        search={state.search}
        connectorKind={connectorKind ?? ConnectorKind.UNSPECIFIED}
        onConnectorSelect={onConnectorSelect}
      />
    </ConnectionFormWrapper>
  );
};

export default CreateConnectionSelector;
