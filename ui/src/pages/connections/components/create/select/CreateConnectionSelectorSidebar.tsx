import type { FC } from "react";

import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Tabs, { type TabItem, TabsSize, TabsVariant } from "@galaxy-io/dls/navigation/Tabs";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { Orientation } from "@galaxy-io/dls/theme/enums";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_PLURAL_NOUN_MAP } from "@/components/connections/constants";
import DocsLink from "@/components/DocsLink";

import type { CreateConnectionSelectorShelf } from "@/pages/connections/components/create/types";
import { CONNECTOR_KIND_TO_DOCS_PATH_MAP } from "@/pages/connections/constants";

interface CreateConnectionSelectorSidebarProps {
  connectorKind: ConnectorKind;
  shelfItems: TabItem<CreateConnectionSelectorShelf>[];
  shelf: CreateConnectionSelectorShelf;
  onShelfChange: (shelf: CreateConnectionSelectorShelf) => void;
}

const CreateConnectionSelectorSidebar: FC<CreateConnectionSelectorSidebarProps> = ({
  connectorKind,
  shelfItems,
  shelf,
  onShelfChange,
}) => (
  <Flex direction={FlexDirection.COLUMN} justifyContent={JustifyContent.SPACE_BETWEEN} grow={1}>
    <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} padding={12} fillWidth>
      <Widget variant={WidgetVariant.SECONDARY}>
        <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={12}>
          <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} variant={TextVariant.TERTIARY}>
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
        label={`Learn about ${CONNECTOR_KIND_TO_PLURAL_NOUN_MAP[connectorKind]}`}
        path={CONNECTOR_KIND_TO_DOCS_PATH_MAP[connectorKind]}
      />
    </Flex>
  </Flex>
);

export default CreateConnectionSelectorSidebar;
