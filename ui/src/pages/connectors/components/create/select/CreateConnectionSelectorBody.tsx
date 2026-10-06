import { useMemo } from "react";

import { styled } from "@linaria/react";
import { useSearch } from "@tanstack/react-router";
import pluralize from "pluralize";

import Skeleton, { SkeletonSize, SkeletonVariant } from "@galaxy-io/dls/feedback/Skeleton";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Grid from "@galaxy-io/dls/layout/Grid";
import Tabs, { type TabItem, TabsSize, TabsVariant } from "@galaxy-io/dls/navigation/Tabs";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { Orientation } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
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
import { MAX_LIST_SEARCH_LENGTH } from "@/api/utils";

interface CreateConnectionSelectorBodyProps {
  search: string;
  onSearchChange: (search: string) => void;
  shelf: CreateConnectionSelectorShelf;
  onShelfChange: (shelf: CreateConnectionSelectorShelf) => void;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

const FrameWrapper = styled.div`
  display: flex;
  height: 100%;

  background-color: ${t.color.background.primary};
  border: 0.5px solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};
  overflow: hidden;
`;

const SidebarWrapper = styled.div`
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  flex-shrink: 0;

  width: ${CREATE_CONNECTION_SELECTOR_SIDEBAR_WIDTH}px;

  background-color: ${t.color.background.primary};
  border-right: 0.5px solid ${t.color.border.primary};
`;

const ContentWrapper = styled.div`
  display: flex;
  flex-direction: column;

  flex: 1;
  min-width: 0;
  background-color: ${t.color.background.base};
`;

const ScrollWrapper = styled.div`
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 16px;
`;

const GhostCard = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px;
  border: 0.5px solid ${t.color.border.tertiary};
  border-radius: ${t.radius.lg};
  background-color: ${t.color.background.primary};
`;

const CreateConnectionSelectorBody = ({
  search,
  onSearchChange,
  shelf,
  onShelfChange,
  onConnectorSelect,
}: CreateConnectionSelectorBodyProps) => {
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
    <FrameWrapper>
      <SidebarWrapper>
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
                variant={TabsVariant.PILL}
                size={TabsSize.SMALL}
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
      </SidebarWrapper>
      <ContentWrapper>
        <Box padding={8} fillWidth>
          <SearchInput
            placeholder={
              isLoading ? `Search ${kindLabel}...` : `Search ${families.length} ${kindLabel}...`
            }
            ariaLabel={`Search ${kindLabel}`}
            onChange={onSearchChange}
            value={search}
            fillWidth
            autoFocus
          />
        </Box>
        <Divider />
        <ScrollWrapper>
          <Grid columns={CREATE_CONNECTION_SELECTOR_GRID_COLUMNS} gap={12}>
            {isLoading
              ? Array.from({ length: CREATE_CONNECTION_SELECTOR_GHOST_COUNT }, (_, index) => (
                  // biome-ignore lint/suspicious/noArrayIndexKey: static skeleton list
                  <GhostCard key={index}>
                    <Box width={24}>
                      <Skeleton variant={SkeletonVariant.RECT} size={SkeletonSize.SMALL} />
                    </Box>
                    <Box width={100}>
                      <Skeleton />
                    </Box>
                  </GhostCard>
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
        </ScrollWrapper>
      </ContentWrapper>
    </FrameWrapper>
  );
};

export default CreateConnectionSelectorBody;
