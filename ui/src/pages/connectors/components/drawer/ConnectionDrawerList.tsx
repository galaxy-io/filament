import { Children, type PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { Radius } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

const ListItem = styled.div`
  padding: 12px;

  &:not(:last-child) {
    border-bottom: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  }
`;

interface ConnectionDrawerListProps {
  variant?: BoxVariant;
  hasBorder?: boolean;
}

const ConnectionDrawerList = ({
  variant = BoxVariant.PRIMARY,
  hasBorder = true,
  children,
}: PropsWithChildren<ConnectionDrawerListProps>) => {
  return (
    <Box
      variant={variant}
      {...(hasBorder ? { radius: Radius.LG, hasBorder: true } : {})}
      overflow="hidden"
      fillWidth
    >
      {Children.map(children, (child) => (
        <ListItem>{child}</ListItem>
      ))}
    </Box>
  );
};

export default ConnectionDrawerList;
