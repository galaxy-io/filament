import type { ReactNode } from "react";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import StatChart, { type StatChartVariant } from "@galaxy-io/dls/charts/StatChart";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
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
        <FlexWrapper alignItems={AlignItems.CENTER} gap={6}>
          {typeof value === "string" ? (
            <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
              {value}
            </Text>
          ) : (
            value
          )}
          <Icon component={icon} variant={IconVariant.TERTIARY} size={14} />
        </FlexWrapper>
      ) : (
        value
      )
    }
    variant={variant}
    noBorder={noBorder}
  />
);

export default MetricCard;
