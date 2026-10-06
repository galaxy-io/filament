import { type AnchorHTMLAttributes, forwardRef } from "react";

import { Link, type LinkProps } from "@tanstack/react-router";

const RouterLink = forwardRef<HTMLAnchorElement, AnchorHTMLAttributes<HTMLAnchorElement>>(
  ({ href, ...rest }, ref) => <Link ref={ref} to={href as LinkProps["to"]} {...rest} />,
);

export default RouterLink;
