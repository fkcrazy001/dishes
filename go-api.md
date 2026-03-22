# dishes-go API（从 Node 版本整理）

本文档从当前 Node 后端实现与前端对接增量（api-inc.md）整理，作为 Go 版本后端（dishes-go）实现清单与进度表。

## 统一约定

- Base URL：`/api`
- 成功响应：

```json
{ "success": true, "data": {} }
```

- 失败响应：

```json
{
  "success": false,
  "error": { "code": "STRING_CODE", "message": "给用户看的简短信息", "details": {} }
}
```

- 认证：Bearer Token（JWT）
  - 登录成功返回 `accessToken`
  - 需要登录的接口头：`Authorization: Bearer <accessToken>`

## 数据规则（增量）

- `loveMilli`：单位为“千分之一爱心”
  - 显示：`loveMilli / 1000`
  - 规则：`1 分 = 0.001 爱心`，因此 `loveMilli = totalCent`
  - 初始值：`loveMilli = 100000`
- 下单扣爱心：`POST /api/orders` 成功后下单人 `loveMilli -= totalCent`
- 取消退爱心：`POST /api/orders/:orderId/cancel` 成功后下单人 `loveMilli += totalCent`
- 完成返爱心：`POST /api/orders/:orderId/finish` 成功后接单人 `loveMilli += totalCent`

## 实时（增量）

- WebSocket：`ws(s)://<host>/api/ws/orders?scope=mine|all&token=<JWT>`
  - `scope=mine`：只推送“我下的单”
  - `scope=all`：推送全部订单（做菜看板）
  - 消息：

```json
{ "type": "order.updated", "data": { "orderId": "ODXXXXXX", "status": "accepted", "updatedAt": 1730000005000 } }
```

可选快照（Node 版本实际会推送）：

```json
{ "type": "order.snapshot", "data": { "order": { "id": "ODXXXXXX", "status": "accepted", "updatedAt": 1730000005000 } } }
```

- SSE（可选）：`GET /api/orders/stream?scope=mine|all`（需要登录）

## 上传与静态资源

- 上传目录：`/uploads/...`（用于订单完成照片、评价图片）
- 前端静态站点：由后端托管（SPA history fallback）

## 进度表（Go 版本）

说明：✅ 已完成 / ⏳ 进行中 / ⬜ 未开始

### Auth / User

- ✅ `POST /api/auth/register`（支持 `password` 或 `passwordHash` 注册；返回 user 含 loveMilli）
- ✅ `POST /api/auth/login`（支持 `password` 或 `passwordHash` 登录；返回 accessToken + user 含 loveMilli）
- ✅ `GET /api/me`（登录态；返回 user 含 loveMilli）
- ✅ `GET /api/users?sort=loveMilli_desc&limit=50`（爱心排行榜；返回 items[{id,name,loveMilli}]）

### Dishes

- ✅ `GET /api/dishes?category=&q=&page=&pageSize=`（点菜页；不强制登录）
- ✅ `GET /api/dishes?scope=mine&page=&pageSize=`（我的菜谱；需要登录）
- ✅ `GET /api/dishes/{dishId}`（详情；不强制登录）
- ✅ `POST /api/dishes`（创建菜谱；需要登录；写入 createdBy）
- ✅ `DELETE /api/dishes/{dishId}`（删除自己的菜谱；需要登录）

### Orders

- ✅ `POST /api/orders`（下单；需要登录；支持 note；扣 love；返回 order + me；推送 ws/sse）
- ✅ `GET /api/orders?scope=mine|all&status=&page=&pageSize=`（列表；需要登录）
- ✅ `GET /api/orders/{orderId}`（详情；需要登录；下单人/接单人可看）
- ✅ `POST /api/orders/{orderId}/accept`（接单；需要登录；写入 acceptedBy；推送 ws/sse）
- ✅ `POST /api/orders/{orderId}/cancel`（取消；需要登录；仅下单人且 placed；退 love；返回 order + me；推送 ws/sse）
- ✅ `POST /api/orders/{orderId}/finish`（完成；需要登录；仅接单人且 accepted；multipart images[] + note；返 love；返回 order + me；推送 ws/sse）
- ✅ `POST /api/orders/{orderId}/review`（评价；需要登录；仅下单人且 done；multipart rating/content/images[]；推送 ws/sse）

### Realtime

- ✅ `GET /api/orders/stream?scope=mine|all`（SSE；需要登录；推送 order.updated + ping）
- ✅ `GET /api/ws/orders?scope=mine|all&token=<JWT>`（WebSocket；连接时推送 order.snapshot；更新时推送 order.updated + order.snapshot）

### Static

- ✅ `/uploads/**`（静态文件）
- ✅ `/`（前端 SPA 静态站点 + history fallback；deploy.sh 会将前端 dist 拷贝进 dishes-go 并编译进 Go 二进制）
