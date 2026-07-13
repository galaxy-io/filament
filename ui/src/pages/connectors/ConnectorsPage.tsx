import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { MagnifyingGlassIcon, PlusIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import GridWrapper from "@galaxy-io/dls/containers/GridWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Dropdown, { DropdownPosition } from "@galaxy-io/dls/dropdown/Dropdown";
import DropdownButton from "@galaxy-io/dls/dropdown/DropdownButton";
import DropdownItem from "@galaxy-io/dls/dropdown/DropdownItem";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectorsRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import { useListConnectorsQuery } from "@/api/queries/connectors";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import ConnectorCard from "@/pages/connectors/components/ConnectorCard";
import ConnectorsPageEmpty from "@/pages/connectors/ConnectorsPageEmpty";
import ConnectorsPageError from "@/pages/connectors/ConnectorsPageError";
import ConnectorsPageLoading from "@/pages/connectors/ConnectorsPageLoading";
import {
  CONNECTOR_GRID_MIN_COLUMN_WIDTH,
  CONNECTOR_KIND_TO_LABEL_MAP,
  CONNECTOR_SEARCH_WIDTH,
} from "@/pages/connectors/constants";
import { useConnectorsPageState } from "@/pages/connectors/hooks";

const ToolbarWrapper = styled.div`
  width: 100%;

  padding: 8px 12px;
`;

const ConnectorListScrollArea = styled.div`
  flex: 1;
  width: 100%;
  min-height: 0;

  padding: 12px;

  overflow-y: auto;
`;

const ConnectorsPage = () => {
  const navigate = useNavigate();

  const {
    search,
    setSearch,
    kindFilter,
    isFiltersOpen,
    setIsFiltersOpen,
    handleSelectKindFilter,
  } = useConnectorsPageState();

  const { data, isLoading, isError } = useListConnectorsQuery({
    input: create(ListConnectorsRequestSchema, { kind: ConnectorKind.UNSPECIFIED }),
  });

  const isToolbarDisabled = isLoading || isError;

  const filteredConnectors = useMemo(() => {
    if (!data?.connectors) return [];

    return data.connectors.filter((connector) => {
      const matchesSearch = search
        ? (connector.displayName || connector.name).toLowerCase().includes(search.toLowerCase())
        : true;
      const matchesKind =
        kindFilter === ConnectorKind.UNSPECIFIED || connector.kind === kindFilter;
      return matchesSearch && matchesKind;
    });
  }, [data?.connectors, search, kindFilter]);

  const handleConnectorClick = (connectorName: string) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, connectorId: connectorName }),
    });
  };

  const renderContent = () => {
    if (isLoading) {
      return <ConnectorsPageLoading />;
    }

    if (isError) {
      return <ConnectorsPageError />;
    }

    if (!filteredConnectors.length) {
      return <ConnectorsPageEmpty hasSearchQuery={!!search} />;
    }

    return (
      <GridWrapper
        columns={`repeat(auto-fill, minmax(${CONNECTOR_GRID_MIN_COLUMN_WIDTH}px, 1fr))`}
        gap={12}
      >
        {filteredConnectors.map((connector) => (
          <ConnectorCard
            key={`${connector.kind}:${connector.name}`}
            connector={connector}
            onClick={() => handleConnectorClick(connector.name)}
          />
        ))}
      </GridWrapper>
    );
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
              width={CONNECTOR_SEARCH_WIDTH}
              leading={{ icon: MagnifyingGlassIcon }}
              isDisabled={isToolbarDisabled}
            />,
            <Dropdown
              key="filters"
              isOpen={isFiltersOpen}
              onClose={() => setIsFiltersOpen(false)}
              position={DropdownPosition.BOTTOM_START}
              body={
                <>
                  <DropdownItem
                    label="All connectors"
                    onClick={() => handleSelectKindFilter(ConnectorKind.UNSPECIFIED)}
                  />
                  <DropdownItem
                    label="Sources"
                    onClick={() => handleSelectKindFilter(ConnectorKind.SOURCE)}
                  />
                  <DropdownItem
                    label="Sinks"
                    onClick={() => handleSelectKindFilter(ConnectorKind.SINK)}
                  />
                </>
              }
            >
              <DropdownButton
                label={CONNECTOR_KIND_TO_LABEL_MAP[kindFilter]}
                isOpen={isFiltersOpen}
                onClick={() => setIsFiltersOpen((prev) => !prev)}
                variant={ButtonVariant.SECONDARY}
                isDisabled={isToolbarDisabled}
              />
            </Dropdown>,
          ]}
          trailingActions={[
            <Button
              key="new-connector"
              label="New connector"
              icon={PlusIcon}
              variant={ButtonVariant.PRIMARY}
              isDisabled={isToolbarDisabled}
              onClick={() => {
                // TODO: open the connector creation flow once it exists.
              }}
            />,
          ]}
        />
      </ToolbarWrapper>
      <FlexItem grow={0} shrink={0} fillWidth>
        <HorizontalDivider />
      </FlexItem>
      <ConnectorListScrollArea>{renderContent()}</ConnectorListScrollArea>
    </FlexWrapper>
  );
};

export default ConnectorsPage;
