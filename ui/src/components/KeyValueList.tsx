import { Children, type FC, type PropsWithChildren } from "react";

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

interface KeyValueListProps {
  variant?: BoxVariant;
  hasBorder?: boolean;
}

const KeyValueList: FC<PropsWithChildren<KeyValueListProps>> = ({
  variant = BoxVariant.PRIMARY,
  hasBorder = true,
  children,
}) => {
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

export default KeyValueList;
