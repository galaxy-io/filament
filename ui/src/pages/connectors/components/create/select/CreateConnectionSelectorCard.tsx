import { useCallback } from "react";

import { GithubLogoIcon, SlackLogoIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorMaturityIcon from "@/pages/connectors/components/ConnectorMaturityIcon";
import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
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
    <Box minHeight={CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT}>
      <Widget
        isInteractive
        /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */ fillWidth
        variant={WidgetVariant.PRIMARY}
        onClick={handleClick}
      >
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} height="100%">
          <Flex
            justifyContent={JustifyContent.SPACE_BETWEEN}
            alignItems={AlignItems.CENTER}
            gap={8}
            fillWidth
          >
            <Flex gap={8} alignItems={AlignItems.CENTER} minWidth={0}>
              <ConnectorTile connector={connector.name} kind={connector.kind} />
              <FlexItem minWidth={0} overflow="hidden">
                <Text weight={TextWeight.MEDIUM} lineClamp={1}>
                  {connector.displayName || connector.name}
                </Text>
              </FlexItem>
            </Flex>
            <Flex alignItems={AlignItems.CENTER} gap={8}>
              <ConnectorMaturityIcon maturity={connector.maturity} />
              <ConnectionKindChip kind={connector.kind} size={ChipSize.SMALL} />
            </Flex>
          </Flex>

          <FlexItem grow={1}>
            <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_SM}>
              {connector.description || CONNECTOR_KIND_TO_DESCRIPTION_MAP[connector.kind]}
            </Text>
          </FlexItem>
          <Button label="Connect" onClick={handleClick} fillWidth />
        </Flex>
      </Widget>
    </Box>
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
    <Box minHeight={CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT}>
      <Widget
        /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */ fillWidth
        variant={WidgetVariant.BASE}
      >
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={12} height="100%">
          <FlexItem grow={1}>
            <Flex
              alignItems={AlignItems.START}
              direction={
                FlexDirection.COLUMN
              } /* @dls-migrate layout.off-scale: Pick a value on the space scale (or a CSS-order tuple of them). */
              gap={6}
            >
              <Text weight={TextWeight.MEDIUM}>Looking for something different?</Text>
              <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_SM}>
                Request a connector by opening an issue on GitHub or message our Slack community.
              </Text>
            </Flex>
          </FlexItem>
          <Flex alignItems={AlignItems.START} gap={8} fillWidth>
            <Button
              label="GitHub"
              icon={GithubLogoIcon}
              variant={ButtonVariant.SECONDARY}
              onClick={handleCreateIssue}
              /* @dls-migrate button.isIconFilled: Removed: pass the filled icon in `leading` at the rung's icon size. */ isIconFilled
              fillWidth
            />
            <Button
              label="Slack"
              icon={SlackLogoIcon}
              variant={ButtonVariant.SECONDARY}
              onClick={handleContact}
              /* @dls-migrate button.isIconFilled: Removed: pass the filled icon in `leading` at the rung's icon size. */ isIconFilled
              fillWidth
            />
          </Flex>
        </Flex>
      </Widget>
    </Box>
  );
};

export default CreateConnectionSelectorCard;
