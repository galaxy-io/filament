import { TextSize } from "@galaxy-io/dls/text/Text";

import { BaseHeaderSize } from "@/layouts/components/types";

export const BASE_HEADER_SIZE_TO_TITLE_SIZE_MAP: Record<BaseHeaderSize, TextSize> = {
  [BaseHeaderSize.SMALL]: TextSize.BODY_MD,
  [BaseHeaderSize.MEDIUM]: TextSize.BODY_LG,
  [BaseHeaderSize.LARGE]: TextSize.HEADING_SM,
};
