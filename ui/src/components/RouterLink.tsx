import { type AnchorHTMLAttributes, forwardRef } from "react";

import { Link } from "@tanstack/react-router";

import { useFilamentBase } from "@/module/hooks";

const RouterLink = forwardRef<HTMLAnchorElement, AnchorHTMLAttributes<HTMLAnchorElement>>(
  ({ href, ...rest }, ref) => {
    const base = useFilamentBase();
    return <Link ref={ref} from={base} to={(href ?? ".") as "."} {...rest} />;
  },
);

export default RouterLink;
