import { useState } from "react";

import { useNavigate } from "@tanstack/react-router";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

/**
 * Hook to open the connection drawer from anywhere in the app.
 * Updates the URL search params which triggers the drawer at the root level.
 */
export const useOpenConnectionDrawer = () => {
  const navigate = useNavigate();

  const openConnectionDrawer = (connectionId: string) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectionId }),
    });
  };

  return openConnectionDrawer;
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
