import { useCallback, useMemo, useState } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

import CreateConnectionModalWrapper from "@/pages/connectors/components/create/components/CreateConnectionModalWrapper";
import CreateConnectionSelectorBody from "@/pages/connectors/components/create/select/CreateConnectionSelectorBody";
import CreateConnectionSelectorHeader from "@/pages/connectors/components/create/select/CreateConnectionSelectorHeader";
import {
  type CreateConnectionSelectorProps,
  CreateConnectionStep,
} from "@/pages/connectors/components/create/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

const CreateConnectionSelector = ({
  onClose,
  onConnectorSelect,
}: CreateConnectionSelectorProps) => {
  const navigate = useNavigate();
  const { connectorKind: connectorKindParam } = useSearch({ from: "__root__" });

  const [search, setSearch] = useState("");

  const connectorKindFilter = useMemo(
    () => connectorKindParam ?? ConnectorKind.UNSPECIFIED,
    [connectorKindParam],
  );

  const handleSearchChange = useCallback((value: string) => {
    setSearch(value);
  }, []);

  const handleConnectorKindFilterChange = useCallback(
    (kind: ConnectorKind) => {
      void navigate({
        to: ".",
        search: (prev) => ({
          ...prev,
          connectorKind: kind,
        }),
      });
    },
    [navigate],
  );

  return (
    <CreateConnectionModalWrapper step={CreateConnectionStep.SELECT}>
      <FlexItem grow={0} shrink={0}>
        <CreateConnectionSelectorHeader
          search={search}
          onSearchChange={handleSearchChange}
          connectorKindFilter={connectorKindFilter}
          onConnectorKindFilterChange={handleConnectorKindFilterChange}
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
    </CreateConnectionModalWrapper>
  );
};

export default CreateConnectionSelector;
