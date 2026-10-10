import { type FC, useMemo } from "react";

import { useSearch } from "@tanstack/react-router";
import pluralize from "pluralize";

import Skeleton, { SkeletonSize, SkeletonVariant } from "@galaxy-io/dls/feedback/Skeleton";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, {
  AlignItems,
  FlexDirection,
  FlexVariant,
  JustifyContent,
} from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Grid from "@galaxy-io/dls/layout/Grid";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Tabs, { type TabItem, TabsSize, TabsVariant } from "@galaxy-io/dls/navigation/Tabs";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { Orientation, Radius } from "@galaxy-io/dls/theme/enums";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import DocsLink from "@/components/DocsLink";

import CreateConnectionSelectorCard, {
  CreateConnectionSelectorEmptyCard,
} from "@/pages/connectors/components/create/select/CreateConnectionSelectorCard";
import type { CreateConnectionSelectorShelf } from "@/pages/connectors/components/create/types";
import { getConnectorFamily, isConnectorOnShelf } from "@/pages/connectors/components/create/utils";
import {
  CONNECTOR_KIND_TO_DOCS_PATH_MAP,
  CONNECTOR_KIND_TO_LABEL_MAP,
  CREATE_CONNECTION_SELECTOR_GHOST_COUNT,
  CREATE_CONNECTION_SELECTOR_GRID_COLUMNS,
  CREATE_CONNECTION_SELECTOR_SHELF_ORDER,
  CREATE_CONNECTION_SELECTOR_SHELF_TO_LABEL_MAP,
  CREATE_CONNECTION_SELECTOR_SIDEBAR_WIDTH,
} from "@/pages/connectors/constants";

import { useListConnectorsQuery } from "@/api/queries/connectors";
import { LIST_SEARCH_DEBOUNCE_MS, MAX_LIST_SEARCH_LENGTH } from "@/api/utils";

interface CreateConnectionSelectorBodyProps {
  onSearch: (search: string) => void;
  shelf: CreateConnectionSelectorShelf;
  onShelfChange: (shelf: CreateConnectionSelectorShelf) => void;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

const CreateConnectionSelectorBody: FC<CreateConnectionSelectorBodyProps> = ({
  onSearch,
  shelf,
  onShelfChange,
  onConnectorSelect,
}) => {
  const { connectorKind, connectorSearch = "" } = useSearch({ from: "/_app" });
  const kind = connectorKind ?? ConnectorKind.UNSPECIFIED;

  const { data, isLoading } = useListConnectorsQuery();

  const families = useMemo(() => {
    const catalog = data?.connectors ?? [];
    const byKey = new Map<string, ConnectorSpec>();
    for (const connector of catalog) {
      if (kind !== ConnectorKind.UNSPECIFIED && connector.kind !== kind) continue;
      const family = getConnectorFamily(connector, catalog);
      byKey.set(`${family.kind}:${family.name}`, family);
    }
    return [...byKey.values()].sort((a, b) =>
      (a.displayName || a.name).localeCompare(b.displayName || b.name),
    );
  }, [data?.connectors, kind]);

  const searchedFamilies = useMemo(() => {
    const query = connectorSearch.trim().slice(0, MAX_LIST_SEARCH_LENGTH).toLowerCase();
    if (!query) return families;
    return families.filter((connector) =>
      [connector.name, connector.displayName, connector.description].some((value) =>
        value.toLowerCase().includes(query),
      ),
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

  const kindLabel = pluralize(CONNECTOR_KIND_TO_LABEL_MAP[kind].toLowerCase());

  return (
    <Flex
      height="100%"
      variant={FlexVariant.PRIMARY}
      hasBorder
      radius={Radius.LG}
      overflow="hidden"
    >
      <Flex
        direction={FlexDirection.COLUMN}
        justifyContent={JustifyContent.SPACE_BETWEEN}
        shrink={0}
        width={CREATE_CONNECTION_SELECTOR_SIDEBAR_WIDTH}
      >
        <Flex
          alignItems={AlignItems.STRETCH}
          direction={FlexDirection.COLUMN}
          padding={12}
          fillWidth
        >
          <Widget variant={WidgetVariant.SECONDARY}>
            <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12}>
              <Text
                size={TextSize.BODY_SM}
                weight={TextWeight.MEDIUM}
                variant={TextVariant.TERTIARY}
              >
                Maturity
              </Text>
              <Tabs
                ariaLabel="Maturity"
                items={shelfItems}
                value={shelf}
                onChange={onShelfChange}
                orientation={Orientation.VERTICAL}
                variant={TabsVariant.UNDERLINE}
                size={TabsSize.SMALL}
                fillWidth
              />
            </Flex>
          </Widget>
        </Flex>
        <Flex alignItems={AlignItems.START} padding={16} fillWidth>
          <DocsLink
            label={`Learn about ${kindLabel}`}
            path={CONNECTOR_KIND_TO_DOCS_PATH_MAP[kind]}
          />
        </Flex>
      </Flex>
      <Divider orientation={Orientation.VERTICAL} />
      <Flex
        direction={FlexDirection.COLUMN}
        grow={1}
        basis={0}
        minWidth={0}
        variant={FlexVariant.BASE}
      >
        <Box padding={8} fillWidth>
          <SearchInput
            placeholder={
              isLoading ? `Search ${kindLabel}...` : `Search ${families.length} ${kindLabel}...`
            }
            ariaLabel={`Search ${kindLabel}`}
            defaultValue={connectorSearch}
            debounceMs={LIST_SEARCH_DEBOUNCE_MS}
            onSearch={onSearch}
            fillWidth
            autoFocus
          />
        </Box>
        <Divider />
        <FlexItem grow={1} minHeight={0}>
          <ScrollArea>
            <Box padding={16}>
              <Grid columns={CREATE_CONNECTION_SELECTOR_GRID_COLUMNS} gap={12}>
                {isLoading
                  ? Array.from({ length: CREATE_CONNECTION_SELECTOR_GHOST_COUNT }, (_, index) => (
                      // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton list
                      <Widget key={index}>
                        <Flex alignItems={AlignItems.CENTER} gap={8}>
                          <Box width={24}>
                            <Skeleton variant={SkeletonVariant.RECT} size={SkeletonSize.SMALL} />
                          </Box>
                          <Box width={100}>
                            <Skeleton />
                          </Box>
                        </Flex>
                      </Widget>
                    ))
                  : shelfFamilies.map((connector) => (
                      <CreateConnectionSelectorCard
                        key={`${connector.name}-${connector.kind}`}
                        connector={connector}
                        onConnectorSelect={onConnectorSelect}
                      />
                    ))}
                {!isLoading && <CreateConnectionSelectorEmptyCard />}
              </Grid>
            </Box>
          </ScrollArea>
        </FlexItem>
      </Flex>
    </Flex>
  );
};

export default CreateConnectionSelectorBody;
