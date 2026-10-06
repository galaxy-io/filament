import { useSearch } from "@tanstack/react-router";

import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SearchInput from "@galaxy-io/dls/inputs/SearchInput";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import DocsButton from "@/components/DocsButton";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import {
  CONNECTOR_KIND_TO_CREATE_DESCRIPTION_MAP,
  CONNECTOR_KIND_TO_CREATE_TITLE_MAP,
  CONNECTOR_KIND_TO_DOCS_PATH_MAP,
} from "@/pages/connectors/constants";

interface CreateConnectionSelectorHeaderProps {
  search: string;
  onSearchChange: (search: string) => void;
}

const CreateConnectionSelectorHeader = ({
  search,
  onSearchChange,
}: CreateConnectionSelectorHeaderProps) => {
  const { connectorKind } = useSearch({ from: "/_app" });
  const kind = connectorKind ?? ConnectorKind.UNSPECIFIED;

  return (
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12} fillWidth>
      <BaseHeader
        size={BaseHeaderSize.LARGE}
        title={CONNECTOR_KIND_TO_CREATE_TITLE_MAP[kind]}
        description={CONNECTOR_KIND_TO_CREATE_DESCRIPTION_MAP[kind]}
        actions={[<DocsButton key="docs" path={CONNECTOR_KIND_TO_DOCS_PATH_MAP[kind]} />]}
      />
      <SearchInput
        placeholder="Search connectors..."
        onChange={onSearchChange}
        value={search}
        size={InputSize.LARGE}
        fillWidth
        autoFocus
      />
    </Flex>
  );
};

export default CreateConnectionSelectorHeader;
