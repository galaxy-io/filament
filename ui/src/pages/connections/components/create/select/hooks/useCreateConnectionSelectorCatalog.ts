import { useMemo } from "react";

import type { TabItem } from "@galaxy-io/dls/navigation/Tabs";
import { isSearchMatch } from "@galaxy-io/dls/utils/search";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { formatConnectorName, getConnectorFamily } from "@/components/connections/utils";

import type { CreateConnectionSelectorShelf } from "@/pages/connections/components/create/types";
import { isConnectorOnShelf } from "@/pages/connections/components/create/utils";
import {
  CREATE_CONNECTION_SELECTOR_SHELF_ORDER,
  CREATE_CONNECTION_SELECTOR_SHELF_TO_LABEL_MAP,
} from "@/pages/connections/constants";

import { useListConnectorsQuery } from "@/api/queries/connectors";
import { MAX_LIST_SEARCH_LENGTH } from "@/api/utils";

export const useCreateConnectionSelectorCatalog = (
  kind: ConnectorKind,
  connectorSearch: string,
  shelf: CreateConnectionSelectorShelf,
) => {
  const { data, error, isLoading } = useListConnectorsQuery();

  const families = useMemo(() => {
    const catalog = data?.connectors ?? [];
    const byKey = new Map<string, ConnectorSpec>();
    for (const connector of catalog) {
      if (kind !== ConnectorKind.UNSPECIFIED && connector.kind !== kind) continue;
      const family = getConnectorFamily(connector, catalog);
      byKey.set(`${family.kind}:${family.name}`, family);
    }
    return [...byKey.values()].sort((a, b) =>
      formatConnectorName(a).localeCompare(formatConnectorName(b)),
    );
  }, [data?.connectors, kind]);

  const searchedFamilies = useMemo(() => {
    const term = connectorSearch.slice(0, MAX_LIST_SEARCH_LENGTH);
    return families.filter((connector) =>
      isSearchMatch(term, connector.name, connector.displayName, connector.description),
    );
  }, [families, connectorSearch]);

  const shelfItems = useMemo<TabItem<CreateConnectionSelectorShelf>[]>(
    () =>
      CREATE_CONNECTION_SELECTOR_SHELF_ORDER.filter(
        (item) => item === shelf || families.some((c) => isConnectorOnShelf(c, item)),
      ).map((item) => ({
        id: item,
        label: CREATE_CONNECTION_SELECTOR_SHELF_TO_LABEL_MAP[item],
        count: isLoading
          ? undefined
          : searchedFamilies.filter((c) => isConnectorOnShelf(c, item)).length,
      })),
    [families, searchedFamilies, shelf, isLoading],
  );

  const shelfFamilies = searchedFamilies.filter((connector) =>
    isConnectorOnShelf(connector, shelf),
  );

  return { familyCount: families.length, shelfItems, shelfFamilies, error, isLoading };
};
