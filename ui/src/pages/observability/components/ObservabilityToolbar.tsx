import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import ObservabilityTimeframeSwitcher from "@/pages/observability/components/ObservabilityTimeframeSwitcher";
import { useObservabilityTimeframe } from "@/pages/observability/providers/ObservabilityTimeframeProvider";

const ObservabilityToolbar = () => {
  const { timeframe, setTimeframe } = useObservabilityTimeframe();

  return (
    <BaseToolbar
      leadingActions={[
        <Text key="title" variant={TextVariant.PRIMARY} weight={TextWeight.MEDIUM}>
          Observability
        </Text>,
      ]}
      trailingActions={[
        <ObservabilityTimeframeSwitcher
          key="timeframe-switcher"
          value={timeframe}
          onChange={setTimeframe}
        />,
      ]}
    />
  );
};

export default ObservabilityToolbar;
