import { useCallback } from "react";

import { GithubLogoIcon, SlackLogoIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
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
    <Widget
      fillWidth
      minHeight={CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT}
      variant={WidgetVariant.PRIMARY}
      onClick={handleClick}
    >
      <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillHeight>
        <FlexWrapper
          justifyContent={JustifyContent.SPACE_BETWEEN}
          alignItems={AlignItems.CENTER}
          gap={8}
          fillWidth
        >
          <FlexWrapper gap={8} alignItems={AlignItems.CENTER} minWidth={0}>
            <ConnectorTile connector={connector.name} kind={connector.kind} />
            <FlexItem minWidth={0} overflow="hidden">
              <Text weight={TextWeight.MEDIUM} isEllipsis>
                {connector.displayName || connector.name}
              </Text>
            </FlexItem>
          </FlexWrapper>
          <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
            <ConnectorMaturityIcon maturity={connector.maturity} />
            <ConnectionKindChip kind={connector.kind} size={ChipSize.SMALL} />
          </FlexWrapper>
        </FlexWrapper>

        <FlexItem grow={1}>
          <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_SM}>
            {connector.description || CONNECTOR_KIND_TO_DESCRIPTION_MAP[connector.kind]}
          </Text>
        </FlexItem>
        <Button label="Connect" onClick={handleClick} fillWidth />
      </FlexWrapper>
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
    <Widget
      fillWidth
      minHeight={CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT}
      variant={WidgetVariant.BASE}
    >
      <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillHeight>
        <FlexItem grow={1}>
          <FlexWrapper direction={FlexDirection.COLUMN} gap={6}>
            <Text weight={TextWeight.MEDIUM}>Looking for something different?</Text>
            <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_SM}>
              Request a connector by opening an issue, or reach out and we&apos;ll help you get set
              up.
            </Text>
          </FlexWrapper>
        </FlexItem>
        <FlexWrapper gap={8} fillWidth>
          <Button
            label="GitHub"
            icon={GithubLogoIcon}
            variant={ButtonVariant.SECONDARY}
            onClick={handleCreateIssue}
            isIconFilled
            fillWidth
          />
          <Button
            label="Slack"
            icon={SlackLogoIcon}
            variant={ButtonVariant.SECONDARY}
            onClick={handleContact}
            isIconFilled
            fillWidth
          />
        </FlexWrapper>
      </FlexWrapper>
    </Widget>
  );
};

export default CreateConnectionSelectorCard;
