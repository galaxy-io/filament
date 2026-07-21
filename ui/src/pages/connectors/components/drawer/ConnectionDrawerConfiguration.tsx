import type { JsonObject } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { GearIcon } from "@phosphor-icons/react";

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

const ConfigValue = withTheme(styled.code<PropsWithTheme>`
  font-family: "SF Mono", "Monaco", "Inconsolata", "Roboto Mono", monospace;
  font-size: 12px;
  padding: 2px 6px;
  border-radius: 4px;
  background-color: ${({ theme }) => theme.color.background.tertiary};
  color: ${({ theme }) => theme.color.text.secondary};
  word-break: break-all;
`);

interface ConnectionDrawerConfigurationProps {
  config?: JsonObject;
}

const ConnectionDrawerConfiguration = ({ config }: ConnectionDrawerConfigurationProps) => {
  const configEntries = config ? Object.entries(config) : [];

  return (
    <Accordion
      header="Configuration"
      icon={GearIcon}
      metric={
        <Badge
          count={configEntries.length}
          size={BadgeSize.SMALL}
          variant={BadgeVariant.SECONDARY}
        />
      }
      isOpenInitial={false}
    >
      {configEntries.length === 0 ? (
        <EmptyLayout
          size={EmptyLayoutSize.SMALL}
          icon={<Icon component={GearIcon} size={16} variant={IconVariant.TERTIARY} />}
          header="No configuration"
          message="This connection has no configuration values."
        />
      ) : (
        <FlexWrapper fillWidth direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
          {configEntries.map(([key, value]) => (
            <FlexWrapper
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              gap={FlexGap.SMALL}
              key={key}
            >
              <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                {key}
              </Text>
              <ConfigValue>
                {typeof value === "object" ? JSON.stringify(value) : String(value)}
              </ConfigValue>
            </FlexWrapper>
          ))}
        </FlexWrapper>
      )}
    </Accordion>
  );
};

export default ConnectionDrawerConfiguration;
