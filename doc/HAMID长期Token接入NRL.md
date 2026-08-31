# HAM ID 长期 Token 接入 NRL 说明

> 适用版本：NRL 后端已包含 `longtoken.go`、`token_login` 配置项的版本。  
> 适用对象：NRL 服务器运维、App / 小程序 / 设备管理端开发、HAM ID 平台管理员。  
> 结论：设备、App、小程序仍然只访问 NRL；NRL 后端接受 HAM ID 长期 API Token，并向 HAM ID 做 introspection 校验。

---

## 1. 背景

HAM ID 是统一认证平台，负责账号、审核、OAuth/OIDC、设备证书、长期 API Token 的集中管理。

NRL 是业务应用服务器。设备、App、小程序、Web 等业务客户端最终访问的是 NRL 后端，而不是直接访问 HAM ID 业务 API。

为此，NRL 增加了一层兼容：

```text
客户端
  │
  │ 携带 HAM ID 长期 API Token
  ▼
NRL 后端
  │ 1. 识别 hamid_pat_ 前缀 token
  │ 2. 调 HAM ID /oauth/introspect 验证
  │ 3. 根据 sub / kind 构造 NRL 会话
  ▼
NRL 业务 API / WebSocket
```

这样客户端不需要在 NRL 里重复注册本地账号，认证状态、吊销、设备状态仍然集中在 HAM ID 管理。

---

## 2. Token 类型

HAM ID 长期 API Token 格式：

```text
hamid_pat_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

它是不透明随机 token，不是 NRL JWT，也不是 OAuth access token JWT。

HAM ID 平台里有两类长期 token：

### 2.1 用户手动创建 Token

由用户在 HAM ID 平台个人中心创建。

- `kind = user`
- `sub = 用户呼号`
- 用于脚本、App、个人设备管理端
- 可设置长期有效或指定过期时间
- 可随时吊销

### 2.2 设备自动签发 Token

由设备 activate 流程自动签发。

- `kind = device`
- `sub = 设备绑定用户的呼号`
- `mac = 设备 MAC`
- 与 HAM ID 中的设备记录关联
- 设备吊销后 token 同步吊销
- 设备禁用后 introspection 不再返回 active

注意：长期 token 用于 NRL 的 HTTP / WebSocket 管理接口。  
业余无线电设备的 UDP 语音数据面不走这个 token，仍使用原有设备证书 / 设备接入体系。

---

## 3. NRL 配置

NRL 的 OIDC 配置节点同时用于 OIDC 登录和长期 token introspection。

### 3.1 配置示例

```yaml
OIDC:
  enabled: true
  issuer: "https://hamid.example.com"
  client_id: "nrl-client-id"
  client_secret: "nrl-client-secret"
  redirect_url: "https://nrl.example.com/user/oidc/callback"
  auto_provision: false
  virtual_login: true
  token_login: true
  button_name: "HAM统一认证平台登录"
```

### 3.2 字段说明

| 字段 | 必填 | 说明 |
|---|---:|---|
| `enabled` | 是 | OIDC 总开关。长期 token 也依赖此开关。 |
| `issuer` | 是 | HAM ID 地址，例如 `https://hamid.example.com`。不要带末尾 `/`。 |
| `client_id` | 是 | NRL 在 HAM ID 注册的 OAuth Client ID。 |
| `client_secret` | 是 | NRL 在 HAM ID 注册的 OAuth Client Secret。introspection 必须使用机密客户端。 |
| `redirect_url` | OIDC 登录需要 | Web OIDC 回调地址。只使用长期 token 时，NRL introspection 本身不使用它。 |
| `auto_provision` | 否 | 是否自动创建 NRL 本地账号。 |
| `virtual_login` | 建议 true | 本地没有同呼号账号时，是否允许创建内存临时会话。 |
| `token_login` | 是 | 是否接受 `hamid_pat_` 长期 token。 |
| `button_name` | 否 | Web 登录页 OIDC 按钮文案。 |

### 3.3 推荐配置

如果目标是“HAM ID 认证通过即可使用 NRL，不要求 NRL 本地账号”：

```yaml
auto_provision: false
virtual_login: true
token_login: true
```

如果要求必须先有 NRL 本地账号：

```yaml
auto_provision: false
virtual_login: false
token_login: true
```

如果希望 HAM ID 用户首次登录时自动创建 NRL 本地账号：

```yaml
auto_provision: true
virtual_login: false
token_login: true
```

---

## 4. HAM ID 平台侧准备

### 4.1 创建 NRL 的 OAuth Client

在 HAM ID 管理后台创建 OAuth 应用，或调用管理接口：

```http
POST /api/admin/oauth/client/create
```

请求要点：

```json
{
  "name": "NRL",
  "redirectUris": ["https://nrl.example.com/user/oidc/callback"],
  "scopes": "openid profile email",
  "public": false
}
```

注意：

- `public` 必须为 `false`。
- introspection 接口要求机密客户端。
- 保存返回的 `clientId` 和 `clientSecret`。
- `clientSecret` 只返回一次，必须保存到 NRL 配置中。

### 4.2 用户创建长期 Token

用户登录 HAM ID 后，在个人中心创建 API Token。

HAM ID 用户侧接口：

```http
POST /api/account/apitokens
```

请求示例：

```json
{
  "name": "NRL App",
  "expiresInDays": 0
}
```

`expiresInDays = 0` 表示长期有效，可随时吊销。

响应示例：

```json
{
  "id": 1,
  "name": "NRL App",
  "kind": "user",
  "prefix": "hamid_pat_ab12cd",
  "scopes": "api",
  "expiresAt": 0,
  "token": "hamid_pat_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
}
```

`token` 只返回一次。

### 4.3 设备自动签发 Token

设备先在 HAM ID 中登记并绑定用户，然后调用：

```http
POST /api/device/activate
```

首次激活响应中会包含：

```json
{
  "result": "ok",
  "code": 0,
  "authTag": "...",
  "apiTokenIssued": true,
  "apiToken": "hamid_pat_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "certPackage": {
    "userCert": {},
    "intermediateCert": {}
  }
}
```

后续证书续期：

```json
{
  "result": "ok",
  "apiTokenIssued": false
}
```

不会重复返回 token 明文。

如果设备丢失 token：

1. 用户在 HAM ID 平台吊销该设备的旧 token。
2. 设备重新 activate。
3. 平台重新签发新 token。

---

## 5. HAM ID Introspection 协议

NRL 后端使用 RFC 7662 Token Introspection 验证长期 token。

### 5.1 请求

```http
POST /oauth/introspect
Content-Type: application/x-www-form-urlencoded

token=hamid_pat_xxx&client_id=xxx&client_secret=xxx
```

NRL 后端内部会自动完成这个调用，客户端不需要访问 HAM ID。

也支持 HTTP Basic：

```http
POST /oauth/introspect
Authorization: Basic base64(client_id:client_secret)
Content-Type: application/x-www-form-urlencoded

token=hamid_pat_xxx
```

### 5.2 用户 token 响应

```json
{
  "active": true,
  "token_type": "Bearer",
  "scope": "api",
  "sub": "BG1ABC",
  "username": "BG1ABC",
  "kind": "user",
  "iss": "https://hamid.example.com",
  "aud": "hamid-api",
  "iat": 1730000000,
  "exp": 1760000000
}
```

### 5.3 设备 token 响应

```json
{
  "active": true,
  "token_type": "Bearer",
  "scope": "api",
  "sub": "BG1ABC",
  "username": "BG1ABC",
  "kind": "device",
  "device_id": 123,
  "mac": "D0CF13510C4C",
  "iss": "https://hamid.example.com",
  "aud": "hamid-api",
  "iat": 1730000000
}
```

### 5.4 无效 token 响应

```json
{
  "active": false
}
```

以下情况都会返回 `active: false`：

- token 不存在；
- token 已吊销；
- token 已过期；
- token 所属用户已禁用；
- 设备 token 关联设备已禁用或吊销。

---

## 6. NRL 请求格式

NRL 支持三种携带方式。

### 6.1 标准 Bearer Header

推荐 App、小程序、脚本使用。

```http
GET /user/info
Authorization: Bearer hamid_pat_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### 6.2 NRL 原有 X-Token Header

适合已有 App / 小程序只替换 token 值的场景。

```http
GET /user/info
X-Token: hamid_pat_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### 6.3 X-Token + Bearer 前缀

兼容一些客户端统一封装 Authorization 逻辑的情况。

```http
GET /user/info
X-Token: Bearer hamid_pat_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

优先级：

1. `Authorization: Bearer ...`
2. `X-Token`
3. `X-Token: Bearer ...`

---

## 7. 常用接口示例

以下地址假设 NRL 服务地址为：

```text
https://nrl.example.com
```

### 7.1 获取当前用户

```http
GET /user/info
Authorization: Bearer hamid_pat_xxx
```

### 7.2 获取我的设备

```http
GET /device/mydevlist
Authorization: Bearer hamid_pat_xxx
```

### 7.3 获取群组列表

```http
POST /group/list
Authorization: Bearer hamid_pat_xxx
Content-Type: application/json

{}
```

### 7.4 查询群组

```http
POST /group/get
Authorization: Bearer hamid_pat_xxx
Content-Type: application/json

{
  "group_id": "0"
}
```

### 7.5 WebSocket

```text
wss://nrl.example.com/ws/calls?token=hamid_pat_xxx
```

WebSocket 无法方便地自定义 Header 时才使用 query token。  
应避免在普通 HTTP API 中把 token 放到 URL 上。

---

## 8. 客户端示例

### 8.1 App / 小程序 HTTP 请求

以 `uni.request` 为例：

```js
const HAMID_TOKEN = uni.getStorageSync('hamid_token')

uni.request({
  url: 'https://nrl.example.com/user/info',
  method: 'GET',
  header: {
    Authorization: `Bearer ${HAMID_TOKEN}`
  },
  success(res) {
    console.log(res.data)
  },
  fail(err) {
    console.error(err)
  }
})
```

也可以使用 NRL 原有 header：

```js
uni.request({
  url: 'https://nrl.example.com/user/info',
  method: 'GET',
  header: {
    'X-Token': HAMID_TOKEN
  }
})
```

### 8.2 curl 示例

```bash
curl -H "Authorization: Bearer hamid_pat_xxx" \
  https://nrl.example.com/user/info
```

或者：

```bash
curl -H "X-Token: hamid_pat_xxx" \
  https://nrl.example.com/user/info
```

### 8.3 WebSocket 示例

```js
const token = uni.getStorageSync('hamid_token')
const ws = uni.connectSocket({
  url: `wss://nrl.example.com/ws/calls?token=${encodeURIComponent(token)}`
})
```

---

## 9. NRL 侧身份映射规则

NRL 收到长期 token 后，会先调用 HAM ID introspection。

假设 introspection 返回：

```json
{
  "active": true,
  "sub": "BG1ABC",
  "kind": "user"
}
```

NRL 的处理顺序：

1. 用 `sub` 转成大写呼号，即 `BG1ABC`。
2. 查询 NRL 本地 `users` 表。
3. 如果 NRL 本地存在同呼号账号，则使用本地账号、本地角色。
4. 如果本地不存在：
   - `virtual_login: true`：创建内存中的临时用户，不写数据库；
   - `virtual_login: false`：返回账号不存在。

角色规则：

- HAM ID token 的 `scope` 目前只是 `api`，不表示 NRL 管理角色。
- NRL 本地账号存在时，使用 NRL 本地角色。
- 本地无账号且使用临时会话时，默认角色为 `ham`。

因此：

```text
HAM ID Token = 认证身份
NRL 本地账号 / 角色 = 业务授权
```

HAM ID 的角色不会直接映射成 NRL 管理员。

---

## 10. 本地账号、临时会话、自动建号关系

### 10.1 本地账号存在

```yaml
virtual_login: true
auto_provision: false
```

HAM ID `sub = BG1ABC`，NRL 本地存在 `BG1ABC`：

```text
使用 NRL 本地账号
角色由 NRL users.roles 决定
```

### 10.2 本地账号不存在，允许临时会话

```yaml
virtual_login: true
auto_provision: false
```

HAM ID `sub = BH9NEW`，NRL 本地不存在 `BH9NEW`：

```text
创建内存临时用户
不写 users 表
默认角色 ham
可访问普通业务
不能执行依赖本地账号 ID 的写操作
```

### 10.3 本地账号不存在，不允许临时会话

```yaml
virtual_login: false
auto_provision: false
```

返回认证失败或账号不存在。

### 10.4 自动创建本地账号

```yaml
virtual_login: true
auto_provision: true
```

NRL 会创建本地账号。

推荐如果只是统一认证，使用：

```yaml
virtual_login: true
auto_provision: false
```

---

## 11. 临时会话能力限制

临时会话没有 NRL 本地 `users.id`，因此不能执行依赖本地账号记录的操作。

以下操作会被拒绝或不可用：

- 修改 NRL 本地资料；
- 修改 NRL 本地密码；
- 修改 NRL 本地头像；
- 创建续费订单；
- 其他强依赖 `users.id` 的写操作。

普通查询、公开房间、设备查询等以呼号为核心的能力可以使用。

如果用户之后需要这些能力，应创建 NRL 本地账号，并保证呼号与 HAM ID 呼号一致。

---

## 12. 吊销与失效

### 12.1 用户 Token 吊销

在 HAM ID 平台吊销用户 token 后：

```text
HAM ID introspection -> active false
NRL 立即拒绝新验证
```

NRL 有 introspection 短缓存，默认 30 秒。  
已缓存请求最多可能继续有效 30 秒。

### 12.2 用户禁用

HAM ID 用户被禁用后：

```text
introspection -> active false
NRL 拒绝 token
```

### 12.3 设备吊销

HAM ID 设备吊销时，会同步吊销该设备关联的 API Token：

```text
设备吊销
  -> 设备证书吊销
  -> api_tokens revoked
  -> introspection active false
  -> NRL 拒绝
```

### 12.4 设备禁用

HAM ID 设备被 block 后：

```text
introspection active false
NRL 拒绝 token
```

取消禁用后 token 恢复有效，前提是 token 本身未吊销、未过期。

---

## 13. 缓存与性能

NRL 不会每次请求都强制访问 HAM ID。

当前实现：

- introspection 结果缓存 30 秒；
- 如果 token 有 `exp`，缓存时间不会超过 token 过期时间；
- token 不落库、不写日志；
- 缓存 key 使用 token 的 SHA-256，不保存原始 token；
- introspection 请求超时 5 秒；
- 缓存条目上限 4096，超过后清理。

吊销延迟：

```text
最大约 30 秒
```

如果对吊销实时性要求极高，可以将缓存时间改为更短，但会增加 HAM ID introspection 压力。

---

## 14. 安全要求

### 14.1 Token 保密

长期 token 等同于账号密码。

- 不要提交到 Git；
- 不要写进前端公开配置；
- 不要打印到日志；
- 不要放在 URL 中；
- 不要通过明文 HTTP 传输；
- 不要给多个设备复用同一个用户 token。

### 14.2 使用 HTTPS

NRL 和 HAM ID 都应使用 HTTPS。

```text
客户端 -> NRL：必须 HTTPS
NRL -> HAM ID：必须 HTTPS
```

### 14.3 最小授权

- 普通用户/设备只应使用 `kind = user` 或 `kind = device` 的普通 token。
- 不要把管理员呼号的 token 配置到公共设备。
- 不要在固件或小程序包中硬编码用户 token。
- 设备应使用 `kind = device` 的设备 token，便于单独吊销。

### 14.4 Web 与长期 token

NRL Web 前端可以继续使用现有 OIDC 登录流程。

Web 不强制使用长期 token。  
如果 Web 后续需要支持长期 token，可以直接把 token 保存到现有 token 存储，并通过 `X-Token` 发送；后端已经兼容。

---

## 15. 错误排查

### 15.1 NRL 返回 token 错误

NRL 原有错误响应通常类似：

```json
{
  "code": 50008,
  "message": "..."
}
```

可能原因：

| 原因 | 处理 |
|---|---|
| token 没有携带 | 检查 `Authorization` 或 `X-Token` |
| token 不是 `hamid_pat_` 前缀 | NRL 会按本地 JWT 处理 |
| NRL 未开启 `token_login` | 打开 NRL OIDC 配置中的长期 API Token 登录 |
| NRL 未配置 `client_secret` | introspection 需要机密客户端 |
| HAM ID 不可达 | 检查 NRL 到 HAM ID 的网络和 HTTPS |
| token 已吊销 | 重新创建或重新激活设备 |
| token 已过期 | 重新创建 |
| HAM ID 用户被禁用 | 到 HAM ID 平台恢复账号 |

### 15.2 提示“长期 token 登录未启用”

检查 NRL 配置：

```yaml
OIDC:
  enabled: true
  token_login: true
```

并确认：

```yaml
issuer:
client_id:
client_secret:
```

都不为空。

### 15.3 提示“账号不存在，请联系管理员”

这表示 HAM ID introspection 已通过，但 NRL 本地没有同呼号账号，且未开启临时会话。

处理方式：

```yaml
virtual_login: true
```

或者在 NRL 创建同呼号本地账号。

### 15.4 提示 HAM ID introspection 401 / invalid_client

通常是 NRL 配置的 `client_id` / `client_secret` 无效，或 HAM ID 中该 OAuth Client 是 public client。

处理：

- HAM ID 中创建机密 OAuth Client；
- `public = false`；
- NRL 配置正确 `client_secret`。

### 15.5 设备 token 一直无效

检查：

1. 设备是否已在 HAM ID 登记；
2. 设备是否已绑定用户；
3. 设备是否被禁用；
4. 设备是否被吊销；
5. token 是否已被吊销；
6. token 是否被重新激活后替换；
7. App 是否缓存了旧 token。

---

## 16. 实现位置

NRL 后端：

```text
longtoken.go     长期 token 验证、introspection、缓存
users.go         checktoken / checktokenSilent / userFromTokenClaims
tools.go         HTTP 鉴权工具函数
calls_ws.go      WebSocket 鉴权
config.go        token_login / virtual_login 配置
http.go          CORS 允许 Authorization header
```

关键函数：

```go
tokenFromRequest(req)
userFromRawToken(ctx, rawToken)
verifyLongToken(ctx, rawToken)
userFromTokenClaims(token)
```

NRL 会把长期 token 统一转成内部 `Claims`：

```go
type Claims struct {
    Username    string
    Roles       []string
    Name        string
    OIDCVirtual bool
}
```

其中 `Username` 是 HAM ID `sub` 对应的大写呼号。

---

## 17. 部署检查清单

HAM ID 平台：

- [ ] 已创建 NRL 机密 OAuth Client；
- [ ] 已保存 `client_id` / `client_secret`；
- [ ] 用户 Token 或设备 Token 已创建；
- [ ] Token 所属用户状态正常；
- [ ] 设备状态正常。

NRL 服务：

- [ ] 已升级到包含长期 token 支持的版本；
- [ ] `OIDC.enabled = true`；
- [ ] `OIDC.token_login = true`；
- [ ] `OIDC.issuer` 指向 HAM ID；
- [ ] `OIDC.client_id` / `client_secret` 配置正确；
- [ ] `OIDC.virtual_login` 按需求配置；
- [ ] NRL 可访问 HAM ID `https://.../oauth/introspect`；
- [ ] 客户端使用 HTTPS 访问 NRL；
- [ ] 客户端不再缓存旧 NRL JWT。

验证：

```bash
curl -H "Authorization: Bearer hamid_pat_xxx" \
  https://nrl.example.com/user/info
```

期望返回 NRL 用户信息。

---

## 18. 推荐使用方式

### 用户个人脚本 / App

使用 HAM ID 用户手动创建的 token：

```http
Authorization: Bearer hamid_pat_xxx
```

### 设备管理端 / 嵌入式设备

优先使用设备自动签发的 token：

```json
{
  "kind": "device",
  "mac": "D0CF13510C4C",
  "sub": "BG1ABC"
}
```

这样设备可以单独吊销，不影响用户其他设备。

### 多设备用户

不要多个设备共用同一个 token。

每台设备应通过 activate 自动获得独立 `kind = device` token。

### 团队 / 多站点部署

每个 NRL 实例在 HAM ID 注册自己的机密 OAuth Client：

```yaml
client_id: nrl-site-a
client_secret: xxx
```

不要多个 NRL 站点共用一个 client secret，便于审计和吊销。

---

## 19. 当前边界与后续扩展

当前版本已经支持：

- 用户 token 登录；
- 设备 token 登录；
- HTTP API；
- WebSocket；
- 本地账号优先；
- 无本地账号临时会话；
- HAM ID introspection；
- 短缓存；
- 用户禁用 / token 吊销 / 设备吊销联动。

当前版本未做：

- 细粒度 scope 控制；
- HAM ID 角色到 NRL 角色映射；
- 设备 token 与 NRL 设备资源的单独授权策略；
- NRL 侧主动刷新或长连接 token 续期。

后续如需更细权限，可以扩展 scope，例如：

```text
api:read
api:write
device:read
device:control
voice:join
admin:read
```

然后在 NRL 按 `scope` 和 `kind` 做二次授权。
