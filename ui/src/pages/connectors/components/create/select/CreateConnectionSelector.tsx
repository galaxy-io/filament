import { useCallback, useState } from "react";

import { useSearch } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import CreateConnectionConfigureWrapper from "@/pages/connectors/components/create/configure/CreateConnectionConfigureWrapper";
import CreateConnectionSelectorBody from "@/pages/connectors/components/create/select/CreateConnectionSelectorBody";
import CreateConnectionSelectorHeader from "@/pages/connectors/components/create/select/CreateConnectionSelectorHeader";
import {
  CreateConnectionModalStep,
  type CreateConnectionSelectorProps,
} from "@/pages/connectors/components/create/types";

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
    <CreateConnectionConfigureWrapper step={CreateConnectionModalStep.SELECT}>
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
    </CreateConnectionConfigureWrapper>
  );
};

export default CreateConnectionSelector;
