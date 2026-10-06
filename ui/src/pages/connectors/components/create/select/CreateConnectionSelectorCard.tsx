import { useCallback } from "react";

import { GithubLogoIcon, SlackLogoIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import {
  CONNECTOR_KIND_TO_DESCRIPTION_MAP,
  CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT,
} from "@/pages/connectors/constants";

import { GITHUB_REPO_URL, SLACK_COMMUNITY_URL } from "@/constants";

interface CreateConnectionSelectorCardProps {
  connector: ConnectorSpec;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

const CreateConnectionSelectorCard = ({
  connector,
  onConnectorSelect,
}: CreateConnectionSelectorCardProps) => {
  const handleClick = useCallback(() => {
    onConnectorSelect(connector);
  }, [connector, onConnectorSelect]);

  return (
    <Widget isInteractive variant={WidgetVariant.PRIMARY} onClick={handleClick}>
      <Flex
        alignItems={AlignItems.START}
        direction={FlexDirection.COLUMN}
        gap={12}
        grow={1}
        minHeight={CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT}
      >
        <Flex
          justifyContent={JustifyContent.SPACE_BETWEEN}
          alignItems={AlignItems.CENTER}
          gap={8}
          fillWidth
        >
          <Flex gap={12} alignItems={AlignItems.CENTER} minWidth={0}>
            <ConnectorTile
              connector={connector.name}
              kind={connector.kind}
              size={ConnectorTileSize.LARGE}
            />
            <FlexItem minWidth={0} overflow="hidden">
              <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM} lineClamp={1}>
                {connector.displayName || connector.name}
              </Text>
            </FlexItem>
          </Flex>
          <ConnectionKindChip kind={connector.kind} size={ChipSize.SMALL} />
        </Flex>

        <FlexItem grow={1}>
          <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_MD}>
            {connector.description || CONNECTOR_KIND_TO_DESCRIPTION_MAP[connector.kind]}
          </Text>
        </FlexItem>
        <Button label="Connect" variant={ButtonVariant.BASE} onClick={handleClick} fillWidth />
      </Flex>
    </Widget>
  );
};

export const CreateConnectionSelectorEmptyCard = () => {
  const handleCreateIssue = () => {
    window.open(`${GITHUB_REPO_URL}/issues/new`, "_blank");
  };

  const handleContact = () => {
    window.open(SLACK_COMMUNITY_URL, "_blank");
  };

  return (
    <Widget variant={WidgetVariant.BASE}>
      <Flex
        alignItems={AlignItems.START}
        direction={FlexDirection.COLUMN}
        gap={12}
        grow={1}
        minHeight={CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT}
      >
        <FlexItem grow={1}>
          <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={4}>
            <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
              Looking for something different?
            </Text>
            <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_MD}>
              Request a connector by opening an issue on GitHub or message our Slack community.
            </Text>
          </Flex>
        </FlexItem>
        <Flex alignItems={AlignItems.START} gap={8} fillWidth>
          <Button
            label="GitHub"
            leading={
              <Icon
                component={GithubLogoIcon}
                size={14}
                weight={IconWeight.FILL}
                variant={IconVariant.INHERIT}
              />
            }
            variant={ButtonVariant.SECONDARY}
            onClick={handleCreateIssue}
            fillWidth
          />
          <Button
            label="Slack"
            leading={
              <Icon
                component={SlackLogoIcon}
                size={14}
                weight={IconWeight.FILL}
                variant={IconVariant.INHERIT}
              />
            }
            variant={ButtonVariant.SECONDARY}
            onClick={handleContact}
            fillWidth
          />
        </Flex>
      </Flex>
    </Widget>
  );
};

export default CreateConnectionSelectorCard;
