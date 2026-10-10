import type { FC } from "react";

import { GithubLogoIcon, SlackLogoIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { IconWeight } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import { CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT } from "@/pages/connections/constants";

import { GITHUB_REPO_URL, SLACK_COMMUNITY_URL } from "@/constants";

const CreateConnectionSelectorEmptyCard: FC = () => {
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
            icon={GithubLogoIcon}
            iconWeight={IconWeight.FILL}
            variant={ButtonVariant.SECONDARY}
            href={`${GITHUB_REPO_URL}/issues/new`}
            isExternal
            fillWidth
          />
          <Button
            label="Slack"
            icon={SlackLogoIcon}
            iconWeight={IconWeight.FILL}
            variant={ButtonVariant.SECONDARY}
            href={SLACK_COMMUNITY_URL}
            isExternal
            fillWidth
          />
        </Flex>
      </Flex>
    </Widget>
  );
};

export default CreateConnectionSelectorEmptyCard;
