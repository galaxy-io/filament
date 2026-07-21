import { styled } from "@linaria/react";
import { KeyIcon } from "@phosphor-icons/react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Badge, { BadgeSize, BadgeVariant } from "@galaxy-io/dls/badge/Badge";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

const SecretValue = withTheme(styled.span<PropsWithTheme>`
  font-family: "SF Mono", "Monaco", "Inconsolata", "Roboto Mono", monospace;
  font-size: 12px;
  padding: 2px 6px;
  border-radius: 4px;
  background-color: ${({ theme }) => theme.color.background.tertiary};
  color: ${({ theme }) => theme.color.text.tertiary};
`);

interface ConnectionDrawerSecretsProps {
  secretRefs: { [key: string]: string };
}

const ConnectionDrawerSecrets = ({
  secretRefs,
}: ConnectionDrawerSecretsProps) => {
  const secretRefEntries = Object.entries(secretRefs || {});

  return (
    <Accordion
      header="Secrets"
      icon={KeyIcon}
      metric={
        <Badge
          count={secretRefEntries.length}
          size={BadgeSize.SMALL}
          variant={BadgeVariant.SECONDARY}
        />
      }
      isOpenInitial={false}
    >
      {secretRefEntries.length === 0 ? (
        <EmptyLayout
          size={EmptyLayoutSize.SMALL}
          icon={
            <Icon
              component={KeyIcon}
              size={16}
              variant={IconVariant.TERTIARY}
            />
          }
          header="No secrets"
          message="This connection has no secret references."
        />
      ) : (
        <FlexWrapper fillWidth direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
          {secretRefEntries.map(([key, value]) => (
            <FlexWrapper
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              gap={FlexGap.SMALL}
              key={key}
            >
              <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                {key}
              </Text>
              <SecretValue>{value}</SecretValue>
            </FlexWrapper>
          ))}
        </FlexWrapper>
      )}
    </Accordion>
  );
};

export default ConnectionDrawerSecrets;
