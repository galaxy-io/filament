import { useState } from "react";

import { useNavigate } from "@tanstack/react-router";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

/**
 * Hook to open the connector drawer from anywhere in the app.
 * Updates the URL search params which triggers the drawer at the root level.
 */
export const useOpenConnectorDrawer = () => {
  const navigate = useNavigate();

  const openConnectorDrawer = (connectorId: string) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectorId }),
    });
  };

  return openConnectorDrawer;
};

export const useConnectorsPageState = () => {
  const [search, setSearch] = useState("");
  const [kindFilter, setKindFilter] = useState<ConnectorKind>(ConnectorKind.UNSPECIFIED);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

  const handleSelectKindFilter = (kind: ConnectorKind) => {
    setKindFilter(kind);
    setIsFiltersOpen(false);
  };

  return {
    search,
    setSearch,
    kindFilter,
    setKindFilter,
    isFiltersOpen,
    setIsFiltersOpen,
    handleSelectKindFilter,
  };
};
