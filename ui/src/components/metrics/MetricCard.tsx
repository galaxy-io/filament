import type { ReactNode } from "react";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import StatChart, { type StatChartVariant } from "@galaxy-io/dls/charts/StatChart";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";

interface MetricCardProps {
  label: string;
  value: ReactNode;
  icon?: PhosphorIcon;
  variant?: StatChartVariant;
  noBorder?: boolean;
}

const MetricCard = ({ label, value, icon, variant, noBorder }: MetricCardProps) => (
  <StatChart
    label={label}
    value={
      icon ? (
        <Flex alignItems={AlignItems.CENTER} gap={4}>
          {typeof value === "string" ? (
            <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
              {value}
            </Text>
          ) : (
            value
          )}
          <Icon component={icon} variant={IconVariant.TERTIARY} size={14} />
        </Flex>
      ) : (
        value
      )
    }
    variant={variant}
    hasBorder={!noBorder}
  />
);

export default MetricCard;
