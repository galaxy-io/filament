import type { FC } from "react";

import { LockSimpleIcon } from "@phosphor-icons/react";

import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import PageLayout from "@galaxy-io/dls/layout/PageLayout";

import SettingsServiceAccountsContent from "@/pages/settings/components/service-accounts/SettingsServiceAccountsContent";

import { useSuspenseListMembersQuery } from "@/api/queries/auth";

const ServiceAccountsPage: FC = () => {
  const { data } = useSuspenseListMembersQuery();

  if (!data.canManage) {
    return (
      <PageLayout header="Service accounts">
        <ErrorLayout
          icon={LockSimpleIcon}
          header="Admins only"
          description="Ask an admin of this organization to manage service accounts."
        />
      </PageLayout>
    );
  }

  return <SettingsServiceAccountsContent />;
};

export default ServiceAccountsPage;
