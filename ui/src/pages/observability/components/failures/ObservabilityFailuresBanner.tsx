import type { FC } from "react";

import { CaretDownIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Alert, { AlertVariant } from "@galaxy-io/dls/feedback/Alert";
import Popover from "@galaxy-io/dls/overlays/Popover";
import { Placement } from "@galaxy-io/dls/theme/enums";

import ObservabilityFailuresDetails from "@/pages/observability/components/failures/ObservabilityFailuresDetails";
import { useObservabilityPipelines } from "@/pages/observability/hooks/useObservabilityPipelines";

const ObservabilityFailuresBanner: FC = () => {
  const { failing } = useObservabilityPipelines();

  if (failing.length === 0) {
    return null;
  }

  return (
    <Alert
      isBanner
      variant={AlertVariant.ERROR}
      header={`${pluralize("pipeline", failing.length, true)} failing`}
      actions={
        <Popover
          placement={Placement.BOTTOM_END}
          ariaLabel="Failing pipelines"
          body={<ObservabilityFailuresDetails entries={failing} />}
        >
          <Button
            label="Details"
            size={ButtonSize.SMALL}
            variant={ButtonVariant.SECONDARY}
            icon={CaretDownIcon}
            isIconTrailing
          />
        </Popover>
      }
    />
  );
};

export default ObservabilityFailuresBanner;
