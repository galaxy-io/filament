import Link, { LinkSize, LinkUnderline } from "@galaxy-io/dls/links/Link";

import { DOCUMENTATION_URL } from "@/constants";

interface DocsLinkProps {
  label: string;
  path?: string;
  size?: LinkSize;
  underline?: LinkUnderline;
}

const DocsLink = ({
  label,
  path = "/",
  size = LinkSize.MEDIUM,
  underline = LinkUnderline.HOVER,
}: DocsLinkProps) => (
  <Link href={`${DOCUMENTATION_URL}${path}`} size={size} underline={underline} isExternal>
    {label}
  </Link>
);

export default DocsLink;
