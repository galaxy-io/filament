import { useCallback } from "react";

import { MagnifyingGlassIcon, XIcon } from "@phosphor-icons/react";
import { useSearch } from "@tanstack/react-router";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import TextInput from "@galaxy-io/dls/inputs/TextInput";

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
  onClose: () => void;
}

const CreateConnectionSelectorHeader = ({
  search,
  onSearchChange,
  onClose,
}: CreateConnectionSelectorHeaderProps) => {
  const { connectorKind } = useSearch({ from: "__root__" });
  const kind = connectorKind ?? ConnectorKind.UNSPECIFIED;

  const handleSearchClear = useCallback(() => {
    onSearchChange("");
  }, [onSearchChange]);

  return (
    <FlexWrapper direction={FlexDirection.COLUMN} fillWidth>
      <FlexWrapper padding="16px" fillWidth>
        <BaseHeader
          size={BaseHeaderSize.LARGE}
          title={CONNECTOR_KIND_TO_CREATE_TITLE_MAP[kind]}
          description={CONNECTOR_KIND_TO_CREATE_DESCRIPTION_MAP[kind]}
          actions={[<DocsButton key="docs" path={CONNECTOR_KIND_TO_DOCS_PATH_MAP[kind]} />]}
          onClose={onClose}
        />
      </FlexWrapper>
      <HorizontalDivider />
      <FlexWrapper padding="8px 16px" fillWidth>
        <TextInput
          key="search"
          placeholder="Search connectors..."
          leading={{ icon: MagnifyingGlassIcon }}
          trailing={search ? { icon: XIcon, onClick: handleSearchClear } : undefined}
          onChange={onSearchChange}
          value={search}
          size={InputSize.LARGE}
          fillWidth
          autoFocus
        />
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default CreateConnectionSelectorHeader;
