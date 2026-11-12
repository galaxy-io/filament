import type { FC } from "react";

import type { ConnectError } from "@connectrpc/connect";

import Skeleton, { SkeletonSize, SkeletonVariant } from "@galaxy-io/dls/feedback/Skeleton";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import ErrorLayout, { ErrorLayoutSize } from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, FlexDirection, FlexVariant } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Grid from "@galaxy-io/dls/layout/Grid";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import { CONNECTOR_KIND_TO_PLURAL_NOUN_MAP } from "@/components/connections/constants";

import CreateConnectionSelectorCard from "@/pages/connections/components/create/select/CreateConnectionSelectorCard";
import CreateConnectionSelectorEmptyCard from "@/pages/connections/components/create/select/CreateConnectionSelectorEmptyCard";
import {
  CREATE_CONNECTION_SELECTOR_GHOST_COUNT,
  CREATE_CONNECTION_SELECTOR_GRID_COLUMNS,
} from "@/pages/connections/constants";

import { IS_DEBUG, LIST_SEARCH_DEBOUNCE_MS } from "@/constants";

interface CreateConnectionSelectorBodyProps {
  connectorKind: ConnectorKind;
  connectorSearch: string;
  familyCount: number;
  shelfFamilies: ConnectorSpec[];
  isLoading: boolean;
  error: ConnectError | null;
  onSearch: (search: string) => void;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

const CreateConnectionSelectorBody: FC<CreateConnectionSelectorBodyProps> = ({
  connectorKind,
  connectorSearch,
  familyCount,
  shelfFamilies,
  isLoading,
  error,
  onSearch,
  onConnectorSelect,
}) => {
  const kindLabel = CONNECTOR_KIND_TO_PLURAL_NOUN_MAP[connectorKind];
  const isError = error !== null;

  return (
    <Flex direction={FlexDirection.COLUMN} height="100%" variant={FlexVariant.BASE}>
      <Box padding={8} fillWidth>
        <SearchInput
          placeholder={
            isLoading ? `Search ${kindLabel}...` : `Search ${familyCount} ${kindLabel}...`
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
        {isError ? (
          <ErrorLayout
            size={ErrorLayoutSize.SMALL}
            header="Failed to load connectors"
            description="The connector catalog could not be loaded. Try again later."
            detail={IS_DEBUG ? error?.message : undefined}
          />
        ) : (
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
        )}
      </FlexItem>
    </Flex>
  );
};

export default CreateConnectionSelectorBody;
