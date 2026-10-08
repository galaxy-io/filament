# Filament Helm Chart

![Chart](https://img.shields.io/github/v/tag/galaxy-io/filament?filter=helm-chart-*&label=chart)
![App](https://img.shields.io/github/v/tag/galaxy-io/filament?filter=v*&sort=semver&label=app)

A Helm chart for Filament

## Introduction

This chart deploys Filament server, control plane, Kubernetes worker dispatch support, and the configuration needed to connect Filament to PostgreSQL and NATS.

> [!CAUTION]
> We strongly recommend running exactly one control-plane replica. Multiple replicas may race while coordinating schedules and runs. Keep `controlPlane.replicas` at `1` and leave `controlPlane.autoscaling.enabled` disabled.

## Optional Requirements

| Repository | Name | Version |
|------------|------|---------|
| https://charts.bitnami.com/bitnami | postgresql | 18.7.11 |
| https://charts.zitadel.com | zitadel | 10.0.4 |
| https://codecentric.github.io/helm-charts | keycloak(keycloakx) | 7.3.2 |
| https://nats-io.github.io/k8s/helm/charts | nats | 2.14.2 |

The vendored PostgreSQL, NATS, Zitadel, and Keycloak charts are disabled by default. See the [Bitnami PostgreSQL chart](https://artifacthub.io/packages/helm/bitnami/postgresql), [NATS chart](https://artifacthub.io/packages/helm/nats/nats), [Zitadel chart](https://artifacthub.io/packages/helm/zitadel/zitadel), and [keycloakx chart](https://artifacthub.io/packages/helm/codecentric/keycloakx) documentation for their full configuration surfaces.

## Runtime configuration

Filament requires a PostgreSQL DSN and a NATS URL. The default PostgreSQL-backed secret provider also requires a base64-encoded encryption key; AWS and GCP Secret Manager use their own credentials instead. Provide the values through `existingSecret` or through chart values so the chart can create the Secret.

To use an existing Secret:

```sh
kubectl create secret generic filament-runtime \
  --from-literal=PERSISTENCE_DSN='postgresql://USER:PASSWORD@HOST:5432/filament?sslmode=require' \
  --from-literal=ENCRYPTION_KEY="$(openssl rand -base64 32)" \
  --from-literal=NATS_URL='nats://nats.example.com:4222'

helm upgrade --install filament \
  oci://ghcr.io/galaxy-io/charts/filament \
  --set existingSecret=filament-runtime
```

For a local or test cluster with the vendored PostgreSQL and NATS charts:

```sh
PG_PASSWORD="$(openssl rand -hex 24)"
ENC_KEY="$(openssl rand -base64 32)"

helm upgrade --install filament \
  oci://ghcr.io/galaxy-io/charts/filament \
  --set postgresql.enabled=true \
  --set nats.enabled=true \
  --set-string postgresql.auth.password="$PG_PASSWORD" \
  --set-string persistence.postgresql.dsn="postgresql://filament:${PG_PASSWORD}@filament-postgresql:5432/filament?sslmode=disable" \
  --set-string secrets.datastore.encryptionKey="$ENC_KEY" \
  --set-string eventBus.nats.url='nats://filament-nats:4222'
```

## Overrides

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| fullnameOverride | string | `""` | String to fully override the generated release name. |
| nameOverride | string | `""` | Provide a name in place of `filament`. |
| namespaceOverride | string | `""` | Override the namespace used in rendered manifests. |

## Common parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| commonLabels | object | `{}` | Labels added to all Filament resources. |
| existingSecret | string | `""` | Existing runtime Secret with `PERSISTENCE_DSN`, `NATS_URL`, and `ENCRYPTION_KEY`. Shared by the server, control plane, and workers. Provider credentials live in a separate Secret so workers never receive them. |

## Server parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| server.autoscaling.enabled | bool | `false` | Enable a HorizontalPodAutoscaler for the server. |
| server.autoscaling.maxReplicas | int | `10` | Maximum server replicas when autoscaling is enabled. |
| server.autoscaling.minReplicas | int | `1` | Minimum server replicas when autoscaling is enabled. |
| server.autoscaling.targetCpu | int | `80` | Target average CPU utilization percentage for server autoscaling. |
| server.image.pullPolicy | string | `"IfNotPresent"` | Server image pull policy. |
| server.image.pullSecrets | list | `[]` | Image pull secrets for the server Deployment. |
| server.image.repository | string | `"ghcr.io/galaxy-io/filament/server"` | Server image repository. |
| server.image.tag | string | `""` (defaults to chart appVersion) | Server image tag. |
| server.ingress.annotations | object | `{}` | Annotations for the Ingress, e.g. a cert-manager issuer or ALB settings. |
| server.ingress.className | string | `""` | IngressClass name, e.g. `nginx` or `alb`. Empty uses the cluster default. |
| server.ingress.enabled | bool | `false` | Enable an Ingress for the server API and UI. |
| server.ingress.hosts | list | `["filament.example.com"]` | Hostnames served by the Ingress. |
| server.ingress.path | string | `"/"` | Path served by the Ingress. |
| server.ingress.pathType | string | `"Prefix"` | PathType for the path. |
| server.ingress.tls | list | `[]` | Ingress TLS configuration, passed through verbatim. |
| server.logLevel | string | `"INFO"` | Minimum server log level. Valid values: INFO, DEBUG, TRACE. |
| server.replicas | int | `1` | Number of server replicas. Ignored when `server.autoscaling.enabled` is true. |
| server.resources | object | `{}` (See [values.yaml]) | Server resource requests and limits. |
| server.service.annotations | object | `{}` | Annotations for the Service, e.g. cloud load balancer settings. |
| server.service.port | int | `8080` | Server service and container port. |
| server.service.type | string | `"ClusterIP"` | Service type. |
| server.serviceAccount.annotations | object | `{}` | Annotations for the chart-created server ServiceAccount, e.g. an IRSA role ARN. |
| server.serviceAccount.name | string | `""` | Existing ServiceAccount name for the server. When set, the chart does not create one. |

## Control plane parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| controlPlane.autoscaling.enabled | bool | `false` | Enable a HorizontalPodAutoscaler for the control plane. |
| controlPlane.autoscaling.maxReplicas | int | `10` | Maximum control plane replicas when autoscaling is enabled. |
| controlPlane.autoscaling.minReplicas | int | `1` | Minimum control plane replicas when autoscaling is enabled. |
| controlPlane.autoscaling.targetCpu | int | `80` | Target average CPU utilization percentage for control plane autoscaling. |
| controlPlane.dispatch.job.backoffLimit | int | `1` | Kubernetes Job backoff limit for dispatched workers. |
| controlPlane.dispatch.job.namePrefix | string | `"filament"` | Prefix used when naming dispatched worker Jobs. |
| controlPlane.dispatch.job.ttlSecondsAfterFinished | int | `3600` | Seconds to retain completed dispatched worker Jobs. |
| controlPlane.dispatch.mode | string | `"kubernetes"` | Worker dispatch backend. |
| controlPlane.dispatch.worker.activeDeadlineSeconds | string | `""` | Worker Job active deadline in seconds. Leave empty for no deadline. |
| controlPlane.dispatch.worker.heartbeatSeconds | int | `30` | Interval in seconds between worker heartbeats, stored in the worker ConfigMap as `HEARTBEAT_SECONDS`. Empty uses the worker's own default. |
| controlPlane.dispatch.worker.image.pullPolicy | string | `"IfNotPresent"` | Worker image pull policy. |
| controlPlane.dispatch.worker.image.pullSecrets | list | `[]` | Image pull secrets for dispatched worker Jobs. |
| controlPlane.dispatch.worker.image.repository | string | `"ghcr.io/galaxy-io/filament/worker"` | Worker image repository used for dispatched Jobs. |
| controlPlane.dispatch.worker.image.tag | string | `""` (defaults to chart appVersion) | Worker image tag. |
| controlPlane.dispatch.worker.logLevel | string | `"INFO"` | Minimum worker log level. Valid values: INFO, DEBUG, TRACE. |
| controlPlane.dispatch.worker.restartPolicy | string | `"Never"` | Restart policy for dispatched worker Jobs. |
| controlPlane.dispatch.worker.serviceAccount.annotations | object | `{}` | Annotations for the chart-created worker ServiceAccount, e.g. an IRSA role ARN. |
| controlPlane.dispatch.worker.serviceAccount.name | string | `""` | Existing ServiceAccount name for dispatched worker Jobs. When set, the chart does not create one. |
| controlPlane.dispatch.worker.terminationGraceSeconds | int | `30` | Worker Job termination grace period in seconds. |
| controlPlane.enabled | bool | `true` | Deploy the Filament control plane. |
| controlPlane.health.port | int | `8081` | Port the control plane serves `/livez`, `/startupz`, and `/readyz` on, stored in the ConfigMap as `HEALTH_ADDR` and used for the container port and probes. |
| controlPlane.image.pullPolicy | string | `"IfNotPresent"` | Control plane image pull policy. |
| controlPlane.image.pullSecrets | list | `[]` | Image pull secrets for the control plane Deployment. |
| controlPlane.image.repository | string | `"ghcr.io/galaxy-io/filament/control-plane"` | Control plane image repository. |
| controlPlane.image.tag | string | `""` (defaults to chart appVersion) | Control plane image tag. |
| controlPlane.logLevel | string | `"INFO"` | Minimum control-plane log level. Valid values: INFO, DEBUG, TRACE. |
| controlPlane.reaper.intervalSeconds | string | `""` | How often the reaper sweeps for zombie runs, in seconds. Empty uses the binary default (60). |
| controlPlane.reaper.staleAfterSeconds | string | `""` | How long a Running run may go without a heartbeat write before the reaper fails it, in seconds. Empty uses the binary default (300). |
| controlPlane.replicas | int | `1` | Number of control plane replicas. Ignored when `controlPlane.autoscaling.enabled` is true. |
| controlPlane.resources | object | `{}` (See [values.yaml]) | Control plane resource requests and limits. |
| controlPlane.serviceAccount.annotations | object | `{}` | Annotations for the chart-created control plane ServiceAccount, e.g. an IRSA role ARN. |
| controlPlane.serviceAccount.name | string | `""` | Existing ServiceAccount name for the control plane. When set, the chart does not create one. |

## Metrics parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| metrics.type | string | `"datastore"` | Metrics backend for the dashboard query API. `datastore` answers from the persistence store and emits no `METRICSSTORE_PROVIDER`; any other value is stored in the server ConfigMap as `METRICSSTORE_PROVIDER`. |

## Persistence parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| persistence.migrate | bool | `true` | Run database migrations as a post-install/post-upgrade hook Job. |
| persistence.postgresql.dsn | string | required | PostgreSQL connection string stored in the chart-created Secret as `PERSISTENCE_DSN`. Required unless `existingSecret` is set. |
| persistence.type | string | `"postgres"` | Persistence provider. |

## Secret provider parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| secrets.aws.region | string | `""` | AWS region for Secrets Manager, stored in the ConfigMap as `AWS_REGION`. Leave empty to use the SDK default chain (env, IMDS). |
| secrets.datastore.encryptionKey | string | required | Base64-encoded AES key stored in the chart-created Secret as `ENCRYPTION_KEY`. Required unless `existingSecret` is set. |
| secrets.datastore.encryptionKeyId | string | `""` | Encryption key identifier stored with each secret row. Change this when rotating keys. |
| secrets.gcp.projectId | string | `""` | GCP project containing Secret Manager secrets, stored as `GCP_PROJECT_ID`. Required when `secrets.type` is `gcp-secret-manager`. |
| secrets.gcp.region | string | `""` | GCP region containing Secret Manager secrets, stored as `GCP_REGION`. Required when `secrets.type` is `gcp-secret-manager`. |
| secrets.prefix | string | `""` | Root name external secret stores give every secret, stored in the ConfigMap as `SECRETS_PREFIX`. Defaults to `filament`: AWS names secrets `filament/<tenant>/...` and GCP `filament-<hash>`, and a prefix replaces that word. The separator is added for you. Unused by datastore. |
| secrets.type | string | `"datastore"` | Secret storage provider. `datastore` keeps secrets in the persistence store and emits no `SECRET_PROVIDER`; external provider names are stored in the ConfigMap as `SECRET_PROVIDER`. |

## Event bus parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| eventBus.nats.stream | string | `"EVENTBUS"` | NATS stream used by Filament. |
| eventBus.nats.subjects | string | `"ingestion.v1.>"` | NATS subject filter consumed by Filament. |
| eventBus.nats.ttlSeconds | int | `604800` | Maximum age of messages retained in the NATS stream, in seconds. Set to 0 to disable age-based expiration. |
| eventBus.nats.url | string | required | NATS connection URL stored in the chart-created Secret as `NATS_URL`. Required unless `existingSecret` is set. |
| eventBus.type | string | `"nats"` | Event bus provider. |

## Authentication parameters

Provider credentials live in a separate Secret so workers never receive them. External Zitadel uses `auth.zitadel.pat` or `auth.zitadel.existingSecret` containing `AUTH_PAT`; the chart creates `<release>-zitadel-auth` when no existing provider Secret is supplied. Vendored Zitadel keeps its setup-generated PAT Secret.

Earlier chart versions stored `AUTH_PAT` in the runtime Secret; move it to the provider Secret on upgrade. Changes to chart-managed auth Secrets roll the server. After changing an existing auth Secret, restart the server yourself.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| auth.bootstrap.adminEmail | string | `""` | Person who administers the tenant, stored as `AUTH_BOOTSTRAP_ADMIN_EMAIL`. Created on boot with `adminPassword` and left alone once it exists. Closes sign-up: people join by invitation. Required with `tenant` unless a Keycloak service account is set. |
| auth.bootstrap.adminPassword | string | `""` | That person's initial password, stored as `AUTH_BOOTSTRAP_ADMIN_PASSWORD` in the provider Secret. With an existing provider Secret, supply that key there instead. |
| auth.bootstrap.tenant | string | `""` | Tenant converged on every boot, stored as `AUTH_BOOTSTRAP_TENANT`. |
| auth.enabled | bool | `false` | Enable authentication. Disabled leaves the API unauthenticated and every request scoped to the default tenant. |
| auth.keycloak.adminPassword | string | `""` | Master-realm admin password, stored as `AUTH_ADMIN_PASSWORD` in the dedicated Keycloak Secret. Required when vendored unless `keycloak.existingSecret` is set. Changing it does not rotate the password stored in Keycloak; rotate there first. |
| auth.keycloak.adminUser | string | `""` | Master-realm admin, stored as `AUTH_ADMIN_USERNAME`. With `adminPassword` the server creates the realm and client on boot. For vendored Keycloak use `keycloak.adminUser`; this value is ignored. |
| auth.keycloak.bootstrap.clientId | string | `""` | Admin service account on the `auth.bootstrap.tenant`, so an SDK works before anyone signs in. Its client id, stored as `AUTH_BOOTSTRAP_CLIENT_ID`. Set with `clientSecret` or not at all. |
| auth.keycloak.bootstrap.clientSecret | string | `""` | Its secret, stored as `AUTH_BOOTSTRAP_CLIENT_SECRET`. With `keycloak.existingSecret`, supply that key in the Secret instead. |
| auth.keycloak.clientId | string | `"filament"` | Filament's confidential client, stored as `AUTH_CLIENT_ID`. |
| auth.keycloak.clientSecret | string | required unless `keycloak.existingSecret` is set | The client's secret, stored as `AUTH_CLIENT_SECRET` in the dedicated Keycloak Secret. Supply a stable value; the chart never generates it. |
| auth.keycloak.issuer | string | required when `auth.type=keycloak` and `keycloak.enabled=false` | Realm URL, `https://<host>/realms/<realm>`, stored as `AUTH_ISSUER`. Must match what the realm advertises. Derived from `keycloak.hostname` when vendored. Keycloak 26 or newer. |
| auth.proxy.roleHeader | string | `""` | Header carrying one of `admin`, `creator` or `viewer`, stored as `AUTH_PROXY_ROLE_HEADER`. Empty leaves callers without roles. |
| auth.proxy.serviceHeader | string | `""` | Header naming a trusted service calling on its own behalf, stored as `AUTH_PROXY_SERVICE_HEADER`. Empty disables service callers. |
| auth.proxy.tenant | string | required when `auth.type=proxy` | The one gateway tenant this release serves, stored as `AUTH_PROXY_TENANT`. Requests for any other tenant are refused. |
| auth.proxy.tenantHeader | string | required when `auth.type=proxy` | Header carrying the gateway's tenant id, stored as `AUTH_PROXY_TENANT_HEADER`. |
| auth.proxy.userHeader | string | required when `auth.type=proxy` | Header carrying the gateway's user id, stored as `AUTH_PROXY_USER_HEADER`. The gateway in front of Filament has already authenticated the caller; the network policy is what makes its headers trustworthy. |
| auth.type | string | `"zitadel"` | Identity provider. Valid values are `zitadel`, `keycloak` and `proxy`. |
| auth.uiOrigin | string | `""` | Origin the UI is served from, stored in the ConfigMap as `AUTH_UI_ORIGIN`; normally the ingress host. An https origin marks the session cookie Secure. |
| auth.zitadel.existingSecret | string | `""` | Server-only Secret containing `AUTH_PAT` for external Zitadel, plus `AUTH_BOOTSTRAP_ADMIN_PASSWORD` when bootstrapping an admin. Kept apart from `existingSecret`, which every worker receives. Unset, the chart creates a dedicated Secret from `auth.zitadel.pat` and `auth.bootstrap.adminPassword`. Vendored Zitadel mints its own PAT. |
| auth.zitadel.issuer | string | required when `auth.enabled=true` | Issuer URL Filament reaches the provider at, stored in the ConfigMap as `AUTH_ISSUER`. Only the server talks to Zitadel, so an in-cluster name is fine, but it must equal the issuer Zitadel advertises or discovery fails. |
| auth.zitadel.pat | string | required for external Zitadel unless `auth.zitadel.existingSecret` is set | Machine-user personal access token, stored in the chart-created Secret as `AUTH_PAT`. Zitadel generates the token itself and will not accept one you choose, so create the machine user out of band and paste the result here. Ignored when `zitadel.enabled=true`: the vendored setup job mints a token into its own Secret and the server reads it from there. |

## Observability parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| observability.otel.endpoint | string | `""` | OTLP endpoint metrics and traces are exported to, stored in the ConfigMaps as `OTEL_EXPORTER_OTLP_ENDPOINT`. Use an `http://` scheme for plaintext in-cluster collectors. Empty disables export. |
| observability.otel.protocol | string | `""` | OTLP transport, stored in the ConfigMaps as `OTEL_EXPORTER_OTLP_PROTOCOL`. Valid values are `grpc` (collector port 4317) and `http/protobuf` (port 4318). Empty defaults to `grpc`. |

## Vendored PostgreSQL parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| postgresql.auth.database | string | `"filament"` | Database created by the vendored PostgreSQL chart. |
| postgresql.auth.enablePostgresUser | bool | `false` | Disable the default `postgres` superuser in the vendored PostgreSQL chart. |
| postgresql.auth.password | string | required when `postgresql.enabled=true` | Password for the vendored PostgreSQL user. Must match `persistence.postgresql.dsn` when vendored PostgreSQL is enabled. |
| postgresql.auth.username | string | `"filament"` | Username created by the vendored PostgreSQL chart. |
| postgresql.backup.enabled | bool | `false` | Enable daily logical dumps (`pg_dumpall`) of the vendored PostgreSQL to a dedicated PVC. See `backup.cronjob.*` in the upstream chart for schedule and storage options. |
| postgresql.enabled | bool | `false` | Enable the vendored Bitnami PostgreSQL chart for local or test clusters. See the [Bitnami PostgreSQL chart](https://artifacthub.io/packages/helm/bitnami/postgresql) for additional configuration. |
| postgresql.fullnameOverride | string | `"filament-postgresql"` | Full name override for the vendored PostgreSQL release. |
| postgresql.primary.persistence.size | string | `"8Gi"` | PVC size for the vendored PostgreSQL primary. |
| postgresql.primary.readinessProbe.failureThreshold | int | `2` |  |
| postgresql.primary.resources | object | `{"limits":{"memory":"1Gi"},"requests":{"cpu":"250m","memory":"512Mi"}}` | Resources for the vendored PostgreSQL primary. Overrides the upstream `nano` preset (192Mi memory limit), which risks OOM kills and unclean shutdowns under real load. |
| postgresql.primary.startupProbe.enabled | bool | `true` |  |
| postgresql.primary.startupProbe.failureThreshold | int | `30` |  |
| postgresql.primary.startupProbe.initialDelaySeconds | int | `0` |  |
| postgresql.primary.startupProbe.periodSeconds | int | `10` |  |
| postgresql.primary.startupProbe.successThreshold | int | `1` |  |
| postgresql.primary.startupProbe.timeoutSeconds | int | `5` |  |

## Vendored NATS parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| nats.config.jetstream.enabled | bool | `true` | Enable JetStream in the vendored NATS chart. |
| nats.config.jetstream.fileStore.pvc.size | string | `"10Gi"` | PVC size for the vendored NATS JetStream file store. |
| nats.container.merge | object | `{"resources":{"limits":{"memory":"512Mi"},"requests":{"cpu":"100m","memory":"256Mi"}}}` | Merged into the vendored NATS container spec. Sets resources so the pod is not BestEffort QoS (first evicted under node pressure); the upstream chart sets none. |
| nats.enabled | bool | `false` | Enable the vendored NATS chart for local or test clusters. See the [NATS chart](https://artifacthub.io/packages/helm/nats/nats) for additional configuration. |
| nats.fullnameOverride | string | `"filament-nats"` | Full name override for the vendored NATS release. |

## Vendored Zitadel parameters

The vendored Zitadel runs its init and setup jobs as post-install hooks so the vendored PostgreSQL exists before they poll it, and the server starts once setup has minted its access token. Do not install with `--wait`, which holds those hooks until pods that depend on them are ready.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| zitadel.enabled | bool | `false` | Enable the vendored Zitadel chart for local or test clusters. See the [Zitadel chart](https://artifacthub.io/packages/helm/zitadel/zitadel) for additional configuration. |
| zitadel.fullnameOverride | string | `"filament-zitadel"` | Full name override for the vendored Zitadel release. |
| zitadel.initJob.annotations | object | `{"helm.sh/hook":"post-install,post-upgrade","helm.sh/hook-delete-policy":"before-hook-creation","helm.sh/hook-weight":"1"}` | Hook timing for the Zitadel init job, moved to post-install so the vendored PostgreSQL exists before the job polls it. |
| zitadel.login.enabled | bool | `false` | Deploy Zitadel's own login UI. Filament serves its own sign-in pages and points the OIDC client at them, so the upstream login Deployment, Service, ConfigMap, ServiceAccount, and generated keypair are unused. |
| zitadel.resources.limits.memory | string | `"1Gi"` |  |
| zitadel.resources.requests | object | `{"cpu":"100m","memory":"512Mi"}` | Resources for the vendored Zitadel pod. The upstream chart sets none, which leaves it BestEffort QoS and first to be evicted. |
| zitadel.setupJob.annotations | object | `{"helm.sh/hook":"post-install,post-upgrade","helm.sh/hook-delete-policy":"before-hook-creation","helm.sh/hook-weight":"2"}` | Hook timing for the Zitadel setup job, weighted to run after init. |
| zitadel.zitadel.configmapConfig.Database.Postgres.Admin.SSL.Mode | string | `"disable"` |  |
| zitadel.zitadel.configmapConfig.Database.Postgres.Database | string | `"zitadel"` | Database Zitadel creates and owns. Keep it separate from Filament's own database even when they share a server. |
| zitadel.zitadel.configmapConfig.Database.Postgres.Host | string | required when `zitadel.enabled=true` | PostgreSQL server Zitadel connects to, e.g. the vendored `filament-postgresql` Service. The init job also needs `Admin.Username` and `Admin.Password` for a role that can create databases, and the runtime needs `User.Password`. |
| zitadel.zitadel.configmapConfig.Database.Postgres.User.SSL.Mode | string | `"disable"` |  |
| zitadel.zitadel.configmapConfig.Database.Postgres.User.Username | string | `"zitadel"` |  |
| zitadel.zitadel.configmapConfig.ExternalDomain | string | required when `zitadel.enabled=true` | Hostname Zitadel advertises itself at. |
| zitadel.zitadel.configmapConfig.ExternalSecure | bool | `true` | Serve external traffic over HTTPS. Zitadel builds its OIDC issuer from `ExternalSecure`, `ExternalDomain`, and `ExternalPort`, and that issuer must match `auth.zitadel.issuer` exactly. |
| zitadel.zitadel.masterkey | string | required when `zitadel.enabled=true` | 32-character key Zitadel encrypts its stored credentials with. |

## Vendored Keycloak parameters

Enable `auth.enabled`, set `auth.type: keycloak`, and enable `keycloak.enabled`.
Set `keycloak.hostname` to the HTTP(S) origin reachable from both the cluster and token clients. Configure `keycloak.ingress.tls` when terminating HTTPS at the ingress.

Keycloak uses the vendored PostgreSQL by default; enable `postgresql.enabled` and set its password, or configure `keycloak.database` for an external database.
Supply stable `auth.keycloak.clientSecret` and `auth.keycloak.adminPassword` values, or set `keycloak.existingSecret` to a dedicated Secret with `AUTH_CLIENT_SECRET` and `AUTH_ADMIN_PASSWORD`. The chart creates `<release>-keycloak-auth` when no existing Secret is supplied. The server creates the realm and client on boot. `keycloak.adminUser` configures both the vendored admin and the server's admin login.

Changing the bootstrap admin password value does not rotate the password in Keycloak; rotate it there first, then update the Secret.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| keycloak.adminUser | string | `"admin"` | Master-realm admin the server creates Filament's realm with. |
| keycloak.command[0] | string | `"/opt/keycloak/bin/kc.sh"` |  |
| keycloak.command[1] | string | `"start"` |  |
| keycloak.database.database | string | `"keycloak"` | Database the init container below creates for Keycloak. Keep it separate from Filament's own database even when they share a server. |
| keycloak.database.existingSecret | string | `"filament-postgresql"` | Secret the vendored PostgreSQL chart stores its user password in. |
| keycloak.database.existingSecretKey | string | `"password"` |  |
| keycloak.database.hostname | string | `"filament-postgresql"` | The vendored PostgreSQL Service. |
| keycloak.database.port | int | `5432` |  |
| keycloak.database.username | string | `"filament"` |  |
| keycloak.database.vendor | string | `"postgres"` |  |
| keycloak.dbchecker.enabled | bool | `true` | Wait for PostgreSQL before starting. |
| keycloak.enabled | bool | `false` | Enable the vendored Keycloak chart. See the [keycloakx chart](https://artifacthub.io/packages/helm/codecentric/keycloakx) for additional configuration. |
| keycloak.existingSecret | string | `""` | Dedicated Secret containing `AUTH_CLIENT_SECRET`, plus `AUTH_ADMIN_PASSWORD` when bootstrapping a realm, `AUTH_BOOTSTRAP_CLIENT_SECRET` when bootstrapping a service account, and `AUTH_BOOTSTRAP_ADMIN_PASSWORD` when bootstrapping an admin. Used for external and vendored Keycloak. Kept apart from `existingSecret`, which every worker receives. Unset, the chart creates `<release>-keycloak-auth` from `auth.keycloak`. |
| keycloak.extraEnv | string | `"- name: KC_BOOTSTRAP_ADMIN_USERNAME\n  value: {{ .Values.adminUser | quote }}\n- name: KC_BOOTSTRAP_ADMIN_PASSWORD\n  valueFrom:\n    secretKeyRef:\n      name: {{ include \"filament.keycloak.secretName\" . | quote }}\n      key: AUTH_ADMIN_PASSWORD\n- name: KC_HOSTNAME\n  value: {{ required \"keycloak.hostname is required when keycloak.enabled=true\" .Values.hostname | trimSuffix \"/\" | quote }}\n"` |  |
| keycloak.extraInitContainers | string | `"- name: create-database\n  image: postgres:17-alpine\n  env:\n    - name: PGHOST\n      value: {{ .Values.database.hostname | quote }}\n    - name: PGPORT\n      value: {{ .Values.database.port | quote }}\n    - name: PGUSER\n      value: {{ .Values.database.username | quote }}\n    - name: PGPASSWORD\n      valueFrom:\n        secretKeyRef:\n          name: {{ .Values.database.existingSecret | quote }}\n          key: {{ .Values.database.existingSecretKey | quote }}\n  command:\n    - sh\n    - -ec\n    - |\n      db={{ .Values.database.database | quote }}\n      psql -d postgres -tAc \"SELECT 1 FROM pg_database WHERE datname = '$db'\" | grep -q 1 || psql -d postgres -c \"CREATE DATABASE \\\"$db\\\"\"\n"` | Creates `database.database` if missing, as `database.username`, which needs CREATEDB. The vendored PostgreSQL user has it. |
| keycloak.fullnameOverride | string | `"filament-keycloak"` | Full name override for the vendored Keycloak release. |
| keycloak.hostname | string | required when `keycloak.enabled=true` | Origin Keycloak advertises and the ingress serves, e.g. `https://id.example.com`. Becomes the issuer, so it must resolve inside the cluster too. An optional port and trailing slash are supported; paths, queries, and fragments are not. |
| keycloak.http.relativePath | string | `"/"` |  |
| keycloak.ingress.enabled | bool | `true` | Ingress at `hostname`, which token clients reach the issuer through. |
| keycloak.ingress.rules[0].host | string | `"{{ first (splitList \":\" (urlParse .Values.hostname).host) }}"` |  |
| keycloak.ingress.rules[0].paths[0].path | string | `"/"` |  |
| keycloak.ingress.rules[0].paths[0].pathType | string | `"Prefix"` |  |
| keycloak.ingress.tls | list | `[]` | Ingress TLS, e.g. a cert-manager Secret for `hostname`. |
| keycloak.proxy.enabled | bool | `true` |  |
| keycloak.proxy.http.enabled | bool | `true` |  |
| keycloak.proxy.mode | string | `"xforwarded"` | Proxy header scheme the ingress forwards. |
| keycloak.resources.limits.memory | string | `"1Gi"` |  |
| keycloak.resources.requests | object | `{"cpu":"100m","memory":"512Mi"}` | Resources for the vendored Keycloak pod. The upstream chart sets none. |

----------------------------------------------
Autogenerated from chart metadata using [helm-docs](https://github.com/norwoodj/helm-docs)
