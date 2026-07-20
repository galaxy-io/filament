import { useCallback, useMemo, useState } from "react";

import { useSearch } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

import CreateConnectionConfigureWrapper from "@/pages/connectors/components/create/configure/CreateConnectionConfigureWrapper";
import CreateConnectionSelectorBody from "@/pages/connectors/components/create/select/CreateConnectionSelectorBody";
import CreateConnectionSelectorHeader from "@/pages/connectors/components/create/select/CreateConnectionSelectorHeader";
import {
  CreateConnectionModalStep,
  type CreateConnectionSelectorProps,
} from "@/pages/connectors/components/create/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

const CreateConnectionSelector = ({
  onClose,
  onConnectorSelect,
}: CreateConnectionSelectorProps) => {
  const { connectorKind: connectorKindParam } = useSearch({ from: "__root__" });

  const [search, setSearch] = useState("");

  const connectorKindFilter = useMemo(
    () => connectorKindParam ?? ConnectorKind.UNSPECIFIED,
    [connectorKindParam],
  );

  const handleSearchChange = useCallback((value: string) => {
    setSearch(value);
  }, []);

  return (
    <CreateConnectionConfigureWrapper step={CreateConnectionModalStep.SELECT}>
      <FlexItem grow={0} shrink={0}>
        <CreateConnectionSelectorHeader
          search={search}
          onSearchChange={handleSearchChange}
          onClose={onClose}
        />
      </FlexItem>
      <FlexItem grow={0} shrink={0}>
        <HorizontalDivider />
      </FlexItem>
      <CreateConnectionSelectorBody
        search={search}
        connectorKindFilter={connectorKindFilter}
        onConnectorSelect={onConnectorSelect}
      />
    </CreateConnectionConfigureWrapper>
  );
};

export default CreateConnectionSelector;
