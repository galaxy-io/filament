import { useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import GridWrapper from "@galaxy-io/dls/containers/GridWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import DropdownButton from "@galaxy-io/dls/dropdown/DropdownButton";
import DropdownItem from "@galaxy-io/dls/dropdown/DropdownItem";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";
import { ListProvidersRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import { useListProvidersQuery } from "@/api/queries/providers";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import ProviderCard from "@/pages/providers/components/ProviderCard";
import {
  PROVIDER_GRID_MIN_COLUMN_WIDTH,
  PROVIDER_KIND_TO_LABEL_MAP,
  PROVIDER_SEARCH_WIDTH,
} from "@/pages/providers/constants";

const ToolbarWrapper = styled.div`
  width: 100%;

  padding: 8px 12px;
`;

const ProviderListScrollArea = styled.div`
  flex: 1;
  width: 100%;
  min-height: 0;

  padding: 12px;

  overflow-y: auto;
`;

const ProvidersPage = () => {
  const [search, setSearch] = useState("");
  const [kindFilter, setKindFilter] = useState<ProviderKind>(ProviderKind.UNSPECIFIED);
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);

  // Fetch all providers (UNSPECIFIED returns both sources and sinks)
  const { data, isLoading } = useListProvidersQuery({
    input: create(ListProvidersRequestSchema, { kind: ProviderKind.UNSPECIFIED }),
  });

  const filteredProviders = useMemo(() => {
    if (!data?.providers) return [];

    return data.providers.filter((provider) => {
      const matchesSearch = search
        ? (provider.displayName || provider.name).toLowerCase().includes(search.toLowerCase())
        : true;
      const matchesKind =
        kindFilter === ProviderKind.UNSPECIFIED || provider.kind === kindFilter;
      return matchesSearch && matchesKind;
    });
  }, [data?.providers, search, kindFilter]);

  const handleSelectKindFilter = (kind: ProviderKind) => {
    setKindFilter(kind);
    setIsFiltersOpen(false);
  };

  return (
    <FlexWrapper fillWidth fillHeight direction={FlexDirection.COLUMN}>
      <ToolbarWrapper>
        <BaseToolbar
          leadingActions={[
            <TextInput
              key="search"
              value={search}
              onChange={setSearch}
              placeholder="Search"
              width={PROVIDER_SEARCH_WIDTH}
              leading={{ icon: MagnifyingGlassIcon }}
            />,
            <Dropdown
              key="filters"
              isOpen={isFiltersOpen}
              onClose={() => setIsFiltersOpen(false)}
              position={DropdownPosition.BOTTOM_START}
              body={
                <>
                  <DropdownItem
                    label="All providers"
                    onClick={() => handleSelectKindFilter(ProviderKind.UNSPECIFIED)}
                  />
                  <DropdownItem
                    label="Sources"
                    onClick={() => handleSelectKindFilter(ProviderKind.SOURCE)}
                  />
                  <DropdownItem
                    label="Sinks"
                    onClick={() => handleSelectKindFilter(ProviderKind.SINK)}
                  />
                </>
              }
            >
              <DropdownButton
                label={PROVIDER_KIND_TO_LABEL_MAP[kindFilter]}
                isOpen={isFiltersOpen}
                onClick={() => setIsFiltersOpen((prev) => !prev)}
                variant={ButtonVariant.SECONDARY}
              />
            </Dropdown>,
          ]}
          trailingActions={[
            <Button
              key="new-provider"
              label="New provider"
              icon={PlusIcon}
              variant={ButtonVariant.PRIMARY}
              onClick={() => {
                // TODO: open the provider creation flow once it exists.
              }}
            />,
          ]}
        />
      </ToolbarWrapper>
      <FlexItem grow={0} shrink={0} fillWidth>
        <HorizontalDivider />
      </FlexItem>
      <ProviderListScrollArea>
        {isLoading ? (
          <FlexWrapper
            fillWidth
            fillHeight
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.CENTER}
          >
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
              Loading...
            </Text>
          </FlexWrapper>
        ) : filteredProviders.length ? (
          <GridWrapper
            columns={`repeat(auto-fill, minmax(${PROVIDER_GRID_MIN_COLUMN_WIDTH}px, 1fr))`}
            gap={12}
          >
            {filteredProviders.map((provider) => (
              <ProviderCard
                key={`${provider.kind}:${provider.name}`}
                provider={provider}
              />
            ))}
          </GridWrapper>
        ) : (
          <FlexWrapper
            fillWidth
            fillHeight
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.CENTER}
          >
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
              No providers match your search.
            </Text>
          </FlexWrapper>
        )}
      </ProviderListScrollArea>
    </FlexWrapper>
  );
};

export default ProvidersPage;
