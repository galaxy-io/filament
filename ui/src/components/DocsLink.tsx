import type { FC } from "react";

import Link, { LinkUnderline } from "@galaxy-io/dls/links/Link";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { DOCUMENTATION_URL } from "@/constants";

interface DocsLinkProps {
  label: string;
  path?: string;
  size?: TextSize;
}

const DocsLink: FC<DocsLinkProps> = ({ label, path = "/", size = TextSize.BODY_MD }) => (
  <Text size={size}>
    <Link href={`${DOCUMENTATION_URL}${path}`} underline={LinkUnderline.HOVER} isExternal>
      {label}
    </Link>
  </Text>
);

export default DocsLink;
