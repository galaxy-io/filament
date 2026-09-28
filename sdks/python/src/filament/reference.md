# Reference
## Auth
<details><summary><code>client.auth.<a href="src/filament/auth/client.py">accept_invite</a>(...) -> AuthV1AcceptInviteResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.auth.accept_invite(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**user_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**code:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**password:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.auth.<a href="src/filament/auth/client.py">get_config</a>(...) -> AuthV1GetAuthConfigResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Session; pre-token, public. An empty issuer means auth is disabled.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.auth.get_config(
    connect_timeout_ms=1000,
    request={
        "key": "value"
    },
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**request:** `AuthV1GetAuthConfigRequest` 
    
</dd>
</dl>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.auth.<a href="src/filament/auth/client.py">get_session</a>(...) -> AuthV1GetSessionResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Session; authenticated. The UI's sign-in check.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.auth.get_session(
    connect_timeout_ms=1000,
    request={
        "key": "value"
    },
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**request:** `AuthV1GetSessionRequest` 
    
</dd>
</dl>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.auth.<a href="src/filament/auth/client.py">get_token</a>(...) -> AuthV1GetTokenResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.auth.get_token(
    connect_timeout_ms=1000,
    client_id="clientId",
    client_secret="clientSecret",
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**client_id:** `str` 
    
</dd>
</dl>

<dl>
<dd>

**client_secret:** `str` 
    
</dd>
</dl>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.auth.<a href="src/filament/auth/client.py">login</a>(...) -> AuthV1LoginResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.auth.login(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**login_name:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**password:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.auth.<a href="src/filament/auth/client.py">logout</a>(...) -> AuthV1LogoutResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.auth.logout(
    connect_timeout_ms=1000,
    request={
        "key": "value"
    },
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**request:** `AuthV1LogoutRequest` 
    
</dd>
</dl>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.auth.<a href="src/filament/auth/client.py">register</a>(...) -> AuthV1RegisterResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.auth.register(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**org_name:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**given_name:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**family_name:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**email:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**password:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## ServiceAccount
<details><summary><code>client.service_account.<a href="src/filament/service_account/client.py">create</a>(...) -> AuthV1CreateServiceAccountResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.service_account.create(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**name:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**description:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**role:** `typing.Optional[AuthV1Role]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.service_account.<a href="src/filament/service_account/client.py">list</a>(...) -> AuthV1ListServiceAccountsResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Service accounts; authenticated tenant administration. Secrets are only
 returned by create and rotate and cannot be retrieved later.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.service_account.list(
    connect_timeout_ms=1000,
    request={
        "key": "value"
    },
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**request:** `AuthV1ListServiceAccountsRequest` 
    
</dd>
</dl>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.service_account.<a href="src/filament/service_account/client.py">remove</a>(...) -> AuthV1RemoveServiceAccountResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.service_account.remove(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**user_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.service_account.<a href="src/filament/service_account/client.py">rotate_secret</a>(...) -> AuthV1RotateServiceAccountSecretResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.service_account.rotate_secret(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**user_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Member
<details><summary><code>client.member.<a href="src/filament/member/client.py">invite</a>(...) -> AuthV1InviteMemberResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.member.invite(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**email:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**given_name:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**family_name:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**role:** `typing.Optional[AuthV1Role]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.member.<a href="src/filament/member/client.py">list</a>(...) -> AuthV1ListMembersResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Members; authenticated tenant administration.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.member.list(
    connect_timeout_ms=1000,
    request={
        "key": "value"
    },
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**request:** `AuthV1ListMembersRequest` 
    
</dd>
</dl>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.member.<a href="src/filament/member/client.py">remove</a>(...) -> AuthV1RemoveMemberResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.member.remove(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**user_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.member.<a href="src/filament/member/client.py">set_role</a>(...) -> AuthV1SetMemberRoleResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.member.set_role(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**user_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**role:** `typing.Optional[AuthV1Role]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Connection
<details><summary><code>client.connection.<a href="src/filament/connection/client.py">create</a>(...) -> IngestionV1CreateConnectionResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Connections; reusable, tenant-scoped sources and sinks.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.connection.create(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**kind:** `typing.Optional[IngestionV1ConnectorKind]` 
    
</dd>
</dl>

<dl>
<dd>

**name:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**connector:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**config:** `typing.Optional[GoogleProtobufStruct]` 
    
</dd>
</dl>

<dl>
<dd>

**secret_refs:** `typing.Optional[typing.Dict[str, str]]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.connection.<a href="src/filament/connection/client.py">delete</a>(...) -> IngestionV1DeleteConnectionResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.connection.delete(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.connection.<a href="src/filament/connection/client.py">get</a>(...) -> IngestionV1GetConnectionResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.connection.get(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.connection.<a href="src/filament/connection/client.py">list</a>(...) -> IngestionV1ListConnectionsResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.connection.list(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**kind:** `typing.Optional[IngestionV1ConnectorKind]` 
    
</dd>
</dl>

<dl>
<dd>

**include_deleted:** `typing.Optional[bool]` 
    
</dd>
</dl>

<dl>
<dd>

**pagination:** `typing.Optional[IngestionV1PaginationRequest]` 
    
</dd>
</dl>

<dl>
<dd>

**search:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**sorting:** `typing.Optional[IngestionV1SortingRequest]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.connection.<a href="src/filament/connection/client.py">update</a>(...) -> IngestionV1UpdateConnectionResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.connection.update(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**connection:** `typing.Optional[IngestionV1Connection]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Pipeline
<details><summary><code>client.pipeline.<a href="src/filament/pipeline/client.py">create</a>(...) -> IngestionV1CreatePipelineResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Pipelines; the persisted node graph.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.create(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**name:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**description:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**schedule:** `typing.Optional[IngestionV1PipelineScheduleConfig]` 
    
</dd>
</dl>

<dl>
<dd>

**worker_configuration:** `typing.Optional[IngestionV1WorkerConfiguration]` 

worker_configuration shapes this pipeline's runs. Omit it to configure the
 pipeline's workers later through UpdatePipeline.
    
</dd>
</dl>

<dl>
<dd>

**execution_mode:** `typing.Optional[IngestionV1ExecutionMode]` — Unspecified defaults to bounded.
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.<a href="src/filament/pipeline/client.py">delete</a>(...) -> IngestionV1DeletePipelineResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.delete(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.<a href="src/filament/pipeline/client.py">get</a>(...) -> IngestionV1GetPipelineResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.get(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**include_versions:** `typing.Optional[bool]` 
    
</dd>
</dl>

<dl>
<dd>

**include_last_run:** `typing.Optional[bool]` 
    
</dd>
</dl>

<dl>
<dd>

**include_schedule:** `typing.Optional[bool]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.<a href="src/filament/pipeline/client.py">list</a>(...) -> IngestionV1ListPipelinesResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.list(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**include_deleted:** `typing.Optional[bool]` 
    
</dd>
</dl>

<dl>
<dd>

**pagination:** `typing.Optional[IngestionV1PaginationRequest]` 
    
</dd>
</dl>

<dl>
<dd>

**include_versions:** `typing.Optional[bool]` 
    
</dd>
</dl>

<dl>
<dd>

**include_last_run:** `typing.Optional[bool]` 
    
</dd>
</dl>

<dl>
<dd>

**include_schedule:** `typing.Optional[bool]` 
    
</dd>
</dl>

<dl>
<dd>

**search:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**sorting:** `typing.Optional[IngestionV1SortingRequest]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.<a href="src/filament/pipeline/client.py">run</a>(...) -> IngestionV1RunPipelineResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Runs; compile + submit a pipeline, then list / snapshot / signal.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.run(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**client_token:** `typing.Optional[str]` 

client_token salts the idempotency key so a deliberate re-run differs from a
 double-click; reusing a token dedupes to the same runs.
    
</dd>
</dl>

<dl>
<dd>

**options:** `typing.Optional[IngestionV1RunOptions]` 

options overrides engine throughput defaults for every run this call
 produces. Unset (or any zero field) defers to defaults.
    
</dd>
</dl>

<dl>
<dd>

**worker_configuration:** `typing.Optional[IngestionV1WorkerConfiguration]` 

worker_configuration overrides the pipeline's own for this call only, field
 by field: an empty field inherits the pipeline's value. The resolved result
 is stamped onto each run, so editing the pipeline afterwards cannot change
 a run already requested.
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.<a href="src/filament/pipeline/client.py">update</a>(...) -> IngestionV1UpdatePipelineResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.update(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**name:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**description:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**worker_configuration:** `typing.Optional[IngestionV1WorkerConfiguration]` 
    
</dd>
</dl>

<dl>
<dd>

**execution_mode:** `typing.Optional[IngestionV1ExecutionMode]` — Unspecified preserves the saved mode; affects future starts only.
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.<a href="src/filament/pipeline/client.py">validate</a>(...) -> IngestionV1ValidatePipelineResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Pipeline options and validation are resolved for the complete graph.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.validate(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**graph:** `typing.Optional[IngestionV1PipelineGraph]` 
    
</dd>
</dl>

<dl>
<dd>

**execution_mode:** `typing.Optional[IngestionV1ExecutionMode]` — Unspecified means bounded. Validation and submission must use the same mode.
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Connector
<details><summary><code>client.connector.<a href="src/filament/connector/client.py">discover_resources</a>(...) -> IngestionV1DiscoverResourcesResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.connector.discover_resources(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**connector:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**config:** `typing.Optional[GoogleProtobufStruct]` 
    
</dd>
</dl>

<dl>
<dd>

**refresh:** `typing.Optional[bool]` 
    
</dd>
</dl>

<dl>
<dd>

**connection_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.connector.<a href="src/filament/connector/client.py">get</a>(...) -> IngestionV1GetConnectorResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.connector.get(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**connector:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**kind:** `typing.Optional[IngestionV1ConnectorKind]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.connector.<a href="src/filament/connector/client.py">get_resource_columns</a>(...) -> IngestionV1GetResourceColumnsResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.connector.get_resource_columns(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**connector:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**config:** `typing.Optional[GoogleProtobufStruct]` 
    
</dd>
</dl>

<dl>
<dd>

**connection_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**resources:** `typing.Optional[typing.List[str]]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.connector.<a href="src/filament/connector/client.py">list</a>(...) -> IngestionV1ListConnectorsResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Catalog of registered source/sink connectors and their config schemas.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.connector.list(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**kind:** `typing.Optional[IngestionV1ConnectorKind]` 
    
</dd>
</dl>

<dl>
<dd>

**pagination:** `typing.Optional[IngestionV1PaginationRequest]` 
    
</dd>
</dl>

<dl>
<dd>

**search:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**sorting:** `typing.Optional[IngestionV1SortingRequest]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.connector.<a href="src/filament/connector/client.py">validate_config</a>(...) -> IngestionV1ValidateConfigResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Ephemeral source/sink operations; no persisted state.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.connector.validate_config(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**kind:** `typing.Optional[IngestionV1ConnectorKind]` 
    
</dd>
</dl>

<dl>
<dd>

**connector:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**config:** `typing.Optional[GoogleProtobufStruct]` 
    
</dd>
</dl>

<dl>
<dd>

**connection_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Run
<details><summary><code>client.run.<a href="src/filament/run/client.py">get</a>(...) -> IngestionV1GetRunResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.run.get(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**run_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.run.<a href="src/filament/run/client.py">list</a>(...) -> IngestionV1ListRunsResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.run.list(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**pipeline_version_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**status:** `typing.Optional[typing.List[IngestionV1RunStatus]]` 
    
</dd>
</dl>

<dl>
<dd>

**pagination:** `typing.Optional[IngestionV1PaginationRequest]` 
    
</dd>
</dl>

<dl>
<dd>

**since_ms:** `typing.Optional[str]` 

since_ms/until_ms window on started_at (inclusive/exclusive, epoch
 millis); 0 means unbounded. Runs that never started are excluded.
    
</dd>
</dl>

<dl>
<dd>

**until_ms:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**search:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**sorting:** `typing.Optional[IngestionV1SortingRequest]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.run.<a href="src/filament/run/client.py">signal</a>(...) -> IngestionV1SignalRunResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.run.signal(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**run_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**signal:** `typing.Optional[IngestionV1RunSignal]` 
    
</dd>
</dl>

<dl>
<dd>

**expected_revision:** `typing.Optional[str]` 

Optional CAS for continuous desired state. A stale revision is rejected.
 Absent permits a transition against the current revision.
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Metrics
<details><summary><code>client.metrics.<a href="src/filament/metrics/client.py">query_aggregate</a>(...) -> MetricsV1QueryAggregateResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.metrics.query_aggregate(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**metrics:** `typing.Optional[typing.List[MetricsV1Metric]]` 
    
</dd>
</dl>

<dl>
<dd>

**since_ms:** `typing.Optional[str]` — inclusive
    
</dd>
</dl>

<dl>
<dd>

**until_ms:** `typing.Optional[str]` — exclusive; unset = now
    
</dd>
</dl>

<dl>
<dd>

**group_by:** `typing.Optional[MetricsV1MetricDimension]` — UNSPECIFIED = single total row, key ""
    
</dd>
</dl>

<dl>
<dd>

**filters:** `typing.Optional[typing.List[MetricsV1MetricFilter]]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.metrics.<a href="src/filament/metrics/client.py">query_timeseries</a>(...) -> MetricsV1QueryTimeseriesResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.metrics.query_timeseries(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**metrics:** `typing.Optional[typing.List[MetricsV1Metric]]` 

metrics are computed together per bucket; point values align with this
 order positionally.
    
</dd>
</dl>

<dl>
<dd>

**since_ms:** `typing.Optional[str]` — inclusive
    
</dd>
</dl>

<dl>
<dd>

**until_ms:** `typing.Optional[str]` — exclusive; unset = now
    
</dd>
</dl>

<dl>
<dd>

**granularity:** `typing.Optional[MetricsV1MetricGranularity]` 
    
</dd>
</dl>

<dl>
<dd>

**tz_offset_minutes:** `typing.Optional[int]` 

tz_offset_minutes shifts bucket boundaries east of UTC so DAY buckets
 match the viewer's local calendar day.
    
</dd>
</dl>

<dl>
<dd>

**group_by:** `typing.Optional[MetricsV1MetricDimension]` — UNSPECIFIED = one total series, key ""
    
</dd>
</dl>

<dl>
<dd>

**filters:** `typing.Optional[typing.List[MetricsV1MetricFilter]]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Pipeline Notifier
<details><summary><code>client.pipeline.notifier.<a href="src/filament/pipeline/notifier/client.py">create</a>(...) -> IngestionV1CreatePipelineNotifierResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Pipeline notifications; multiple independent rules per pipeline.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.notifier.create(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**notifier:** `typing.Optional[IngestionV1NotifierInput]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.notifier.<a href="src/filament/pipeline/notifier/client.py">delete</a>(...) -> IngestionV1DeletePipelineNotifierResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.notifier.delete(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**notifier_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.notifier.<a href="src/filament/pipeline/notifier/client.py">list</a>(...) -> IngestionV1ListPipelineNotifiersResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.notifier.list(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.notifier.<a href="src/filament/pipeline/notifier/client.py">update</a>(...) -> IngestionV1UpdatePipelineNotifierResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.notifier.update(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**notifier_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**notifier:** `typing.Optional[IngestionV1NotifierInput]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Pipeline Schedule
<details><summary><code>client.pipeline.schedule.<a href="src/filament/pipeline/schedule/client.py">create</a>(...) -> IngestionV1CreatePipelineScheduleResponse</code></summary>
<dl>
<dd>

#### 📝 Description

<dl>
<dd>

<dl>
<dd>

Pipeline schedules; one primary schedule per pipeline. Enable/disable is
 expressed through PipelineScheduleConfig.is_enabled on update; a schedule
 is removed only by deleting its pipeline.
</dd>
</dl>
</dd>
</dl>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.schedule.create(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**schedule:** `typing.Optional[IngestionV1PipelineScheduleConfig]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.schedule.<a href="src/filament/pipeline/schedule/client.py">update</a>(...) -> IngestionV1UpdatePipelineScheduleResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.schedule.update(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**schedule:** `typing.Optional[IngestionV1PipelineScheduleConfig]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

## Pipeline Version
<details><summary><code>client.pipeline.version.<a href="src/filament/pipeline/version/client.py">create</a>(...) -> IngestionV1CreatePipelineVersionResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.version.create(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**graph:** `typing.Optional[IngestionV1PipelineGraph]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.version.<a href="src/filament/pipeline/version/client.py">get</a>(...) -> IngestionV1GetPipelineVersionResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.version.get(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**version:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

<details><summary><code>client.pipeline.version.<a href="src/filament/pipeline/version/client.py">list</a>(...) -> IngestionV1ListPipelineVersionsResponse</code></summary>
<dl>
<dd>

#### 🔌 Usage

<dl>
<dd>

<dl>
<dd>

```python
from filament import Filament
from filament.environment import FilamentEnvironment

client = Filament(
    client_id="<clientId>",
    client_secret="<clientSecret>",
    environment=FilamentEnvironment.DEFAULT,
)

client.pipeline.version.list(
    connect_timeout_ms=1000,
)

```
</dd>
</dl>
</dd>
</dl>

#### ⚙️ Parameters

<dl>
<dd>

<dl>
<dd>

**connect_timeout_ms:** `typing.Optional[ConnectTimeoutHeader]` — Define the timeout, in ms
    
</dd>
</dl>

<dl>
<dd>

**pipeline_id:** `typing.Optional[str]` 
    
</dd>
</dl>

<dl>
<dd>

**pagination:** `typing.Optional[IngestionV1PaginationRequest]` 
    
</dd>
</dl>

<dl>
<dd>

**sorting:** `typing.Optional[IngestionV1SortingRequest]` 
    
</dd>
</dl>

<dl>
<dd>

**request_options:** `typing.Optional[RequestOptions]` — Request-specific configuration.
    
</dd>
</dl>
</dd>
</dl>


</dd>
</dl>
</details>

