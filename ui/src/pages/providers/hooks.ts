import { useState } from "react";

import { useNavigate } from "@tanstack/react-router";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";

/**
 * Hook to open the provider drawer from anywhere in the app.
 * Updates the URL search params which triggers the drawer at the root level.
 */
export const useOpenProviderDrawer = () => {
  const navigate = useNavigate();

  const openProviderDrawer = (providerId: string) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, providerId }),
    });
  };

  return openProviderDrawer;
};

export const useProvidersPageState = () => {
  const [search, setSearch] = useState("");
  const [kindFilter, setKindFilter] = useState<ProviderKind>(ProviderKind.UNSPECIFIED);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

  const handleSelectKindFilter = (kind: ProviderKind) => {
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
