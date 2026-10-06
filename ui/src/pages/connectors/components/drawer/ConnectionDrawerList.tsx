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
}

const ConnectionDrawerList = ({
  variant = BoxVariant.PRIMARY,
  children,
}: PropsWithChildren<ConnectionDrawerListProps>) => {
  return (
    <Box variant={variant} radius={Radius.LG} overflow="hidden" hasBorder fillWidth>
      {Children.map(children, (child) => (
        <ListItem>{child}</ListItem>
      ))}
    </Box>
  );
};

export default ConnectionDrawerList;
