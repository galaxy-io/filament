import Link, { LinkUnderline } from "@galaxy-io/dls/links/Link";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { DOCUMENTATION_URL } from "@/constants";

interface DocsLinkProps {
  label: string;
  path?: string;
  size?: TextSize;
  underline?: LinkUnderline;
}

const DocsLink = ({
  label,
  path = "/",
  size = TextSize.BODY_MD,
  underline = LinkUnderline.HOVER,
}: DocsLinkProps) => (
  <Text size={size}>
    <Link href={`${DOCUMENTATION_URL}${path}`} underline={underline} isExternal>
      {label}
    </Link>
  </Text>
);

export default DocsLink;
