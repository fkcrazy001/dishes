# API 增量变更（前端对接记录）

本文档用于记录相对基础 API 的增量变更点，以及前端为此做了哪些改动。

## 1) 登录：密码字段改为 passwordHash

影响接口：`POST /api/auth/login`

变更：

- 旧：`{ "account": string, "password": string }`
- 新：`{ "account": string, "passwordHash": string }`

说明：
- `passwordHash` 为 `password` 的 SHA-256 hex 小写字符串（64 字符）
- 前端已在登录请求中使用 `passwordHash`

## 2) 订单状态推送：使用 WebSocket（替代前端轮询）

新增接口：

`ws(s)://<host>/api/ws/orders?scope=mine|all&token=<JWT>`

消息：

```json
{
  "type": "order.updated",
  "data": { "orderId": "ODK2F3A9", "status": "accepted", "updatedAt": 1730000005000 }
}
```

可选（服务端推送完整订单）：

```json
{
  "type": "order.snapshot",
  "data": { "order": { "id": "ODK2F3A9", "status": "accepted", "updatedAt": 1730000005000 } }
}
```

前端改动：
- 订单状态不再定时 `GET /api/orders`，改为 WebSocket 推送驱动；必要时才 `GET /api/orders/:orderId` 补齐详情

## 3) 用户：新增爱心值字段 loveMilli

影响接口：
- `GET /api/me`
- `POST /api/auth/login`
- `POST /api/auth/register`（若返回 user）
 - `GET /api/users?sort=loveMilli_desc&limit=50`

变更：

`User` 增加字段：

```json
{ "loveMilli": 100000 }
```

说明：
- `loveMilli` 单位为“千分之一爱心”，展示时 `loveMilli / 1000`
- 规则：`1 元 = 0.1 爱心`，等价于 `1 分 = 0.001 爱心`，因此 `loveMilli = totalCent`

前端改动：
- 页面右上角显示当前账号爱心值
- 点击“爱心”可打开排行榜（按爱心值排序）

## 4) 订单：写入下单人 placedBy / 接单人 acceptedBy

影响数据模型：`Order`

新增字段：

```json
{
  "placedBy": { "userId": "u_123", "name": "小熊" },
  "acceptedBy": { "userId": "u_456", "name": "妈妈" }
}
```

前端改动：
- “做菜/订单看板”中展示下单人信息
- “我的订单”中展示接单人信息（若已接单）

## 5) 订单结算：下单扣爱心、完成返爱心

影响接口：
- `POST /api/orders`：下单成功后下单人爱心值减少 `totalCent` 对应的 `loveMilli`
- `POST /api/orders/:orderId/finish`：完成订单后接单人爱心值增加对应 `loveMilli`

请求增量：
- `POST /api/orders` 支持增加 `note` 字段作为下单备注（可选）

```json
{
  "items": [{ "dishId": "tomato-egg", "qty": 1 }],
  "note": "少盐少油，不要香菜"
}
```

订单数据模型增量：

```json
{ "placedNote": "少盐少油，不要香菜" }
```

前端改动：
- 点单弹窗增加“订单备注”输入框，下单时随请求提交；订单详情里展示备注

相关错误（示例）：
- `INSUFFICIENT_LOVE`：爱心值不足，无法下单

响应增量（便于前端无需额外请求刷新余额）：

- `POST /api/orders` 返回 `data.user` 或 `data.me`（更新后的当前用户）
- `POST /api/orders/:orderId/finish` 如返回 `data.user` 或 `data.me`，前端可直接更新余额

示例（下单）：

```json
{
  "success": true,
  "data": {
    "order": { "id": "ODK2F3A9", "totalCent": 3600 },
    "me": { "id": "u_123", "loveMilli": 96400 }
  }
}
```

前端改动：
- 下单区域提示“将扣除爱心 …”
- 做菜看板显示“完成可获得爱心 …”
- 下单与完成后刷新当前用户爱心值

## 6) 菜谱管理：创建与删除自己的菜谱

新增/增强接口：
- `POST /api/dishes`：创建菜谱（需要登录）
- `GET /api/dishes?scope=mine`：获取我创建的菜谱（需要登录）
- `DELETE /api/dishes/:dishId`：删除我创建的菜谱（需要登录）

请求：

```json
{
  "name": "葱油拌面",
  "category": "home",
  "timeText": "15 分钟",
  "level": "easy",
  "tags": ["快手", "下饭"],
  "priceCent": 1800,
  "story": "热气腾腾的一碗面，简单又满足",
  "imageUrl": "https://...",
  "badge": "快手",
  "details": { "ingredients": ["面 1 份"], "steps": ["煮面", "拌匀"] }
}
```

响应：

```json
{ "success": true, "data": { "dish": { "id": "scallion-noodle" } } }
```

数据模型增量：

```json
{
  "createdBy": { "userId": "u_123", "name": "小熊" }
}
```

前端改动：
- 顶部入口改为“菜谱管理”，支持查看“我的菜谱”、创建新菜谱、删除自己的菜谱

## 7) 订单：下单人可取消（接单前）

新增接口：`POST /api/orders/:orderId/cancel`

说明：
- 仅允许下单人取消，且只能在订单 `status=placed`（未接单）时取消
- 成功后订单 `status` 变为 `cancelled`
- 若该订单下单时扣除了爱心，取消成功后应退回对应爱心（见 5）

响应（示例）：

```json
{
  "success": true,
  "data": {
    "order": { "id": "ODK2F3A9", "status": "cancelled", "updatedAt": 1730000005000 },
    "me": { "id": "u_123", "loveMilli": 100000 }
  }
}
```

前端改动：
- “我的订单”与“订单详情”增加“取消订单”入口（仅接单前展示）

## 8) 订单完成：上传完成图片

接口变更：`POST /api/orders/:orderId/finish`

变更：
- 支持 `multipart/form-data` 上传完成图片
- 表单字段：
  - `images`：图片文件（可多张，前端最多 3 张）
  - `note`：可选文字备注

订单数据模型增量：

```json
{
  "finishedAt": 1730000010000,
  "finishImages": ["https://.../order/ODK2F3A9/1.jpg"]
}
```

前端改动：
- “做菜”点击完成时弹窗选择图片与备注，再提交完成
- 下单人查看订单详情时可看到完成图片

## 9) 订单详情与评价

接口增强：`GET /api/orders/:orderId`

说明：
- 返回订单详情时包含 `finishImages`、`finishedAt`、`review`（若已评价）

新增接口：`POST /api/orders/:orderId/review`

请求：`multipart/form-data`
- `rating`：1-5
- `content`：评价文字
- `images`：评价图片（可多张，前端最多 3 张）

响应（示例）：

```json
{
  "success": true,
  "data": {
    "order": {
      "id": "ODK2F3A9",
      "review": { "rating": 5, "content": "好吃！", "images": ["https://.../r1.jpg"], "createdAt": 1730000020000 }
    }
  }
}
```

前端改动：
- “我的订单”点击进入订单详情：可查看完成情况、图片；完成后可上传评价（文字/评分/图片）

## 当前任务状态（后端）

- 2026-03-21：1) 已完成：登录支持 `passwordHash`，并兼容旧 `password`（老账号用 `password` 成功登录后会自动升级）
- 2026-03-21：2) 已完成：新增 WebSocket `ws(s)://<host>/api/ws/orders?scope=mine|all&token=<JWT>`，连接后推送 `order.snapshot`（最多 50 条），并在接单/完成时广播 `order.updated`
- 2026-03-21：3) 已完成：`User` 增加 `loveMilli`，并在 `/api/me`、登录/注册响应中返回
- 2026-03-21：4) 已完成：`Order` 增加 `acceptedBy`，接单时写入并在查询/推送中返回
- 2026-03-21：5) 已完成：下单扣减 `loveMilli`（不足返回 `INSUFFICIENT_LOVE`），支持下单备注 `note`（订单字段 `placedNote`），完成订单给接单人返还 `loveMilli`，接口响应返回 `me` 便于前端刷新余额
- 2026-03-21：6) 已完成：菜谱管理接口已支持 `POST /api/dishes` 创建（写入 `createdBy`）、`GET /api/dishes?scope=mine` 查询我的菜谱、`DELETE /api/dishes/:dishId` 删除自己的菜谱
- 2026-03-21：7) 已完成：新增 `POST /api/orders/:orderId/cancel`（仅下单人、仅 placed 可取消、取消退回爱心），并广播 `order.updated`
- 2026-03-21：8) 已完成：`POST /api/orders/:orderId/finish` 支持 `multipart/form-data` 上传完成图片与备注，订单返回包含 `finishedAt` / `finishImages`
- 2026-03-21：9) 已完成：新增 `POST /api/orders/:orderId/review`（multipart 评分/文字/图片），订单详情返回包含 `review`
