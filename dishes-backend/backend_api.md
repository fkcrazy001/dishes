# dishes-backend 对外 API（以当前代码实现为准）

## Base URL

`/api`

## 通用约定

### Content-Type

- 请求体：`application/json`
- 响应体：`application/json`（SSE 除外）

### 时间与金额

- 时间戳：`createdAt` / `updatedAt` 使用毫秒时间戳（`number`）
- 金额：统一使用“分”存储（`priceCent` / `totalCent`，`number`）

### 统一响应结构

成功：

```json
{
  "success": true,
  "data": {}
}
```

失败：

```json
{
  "success": false,
  "error": {
    "code": "STRING_CODE",
    "message": "给用户看的简短信息",
    "details": {}
  }
}
```

### 错误码

后端当前可能返回的 `error.code`：

- `ACCOUNT_EXISTS`：账号已存在
- `INVALID_PASSWORD`：密码至少 6 位
- `INVALID_CREDENTIALS`：账号或密码错误
- `UNAUTHORIZED`：未登录 / 无权限
- `VALIDATION_ERROR`：参数错误
- `DISH_NOT_FOUND`：菜谱不存在
- `INVALID_QTY`：数量不合法
- `INSUFFICIENT_LOVE`：爱心值不足
- `ORDER_NOT_FOUND`：订单不存在
- `ORDER_INVALID_STATUS`：订单状态不允许该操作
- `ORDER_ALREADY_REVIEWED`：订单已评价
- `INTERNAL_ERROR`：服务内部错误

### HTTP 状态码

当前实现中的常见状态码：

- 200：成功（以及大多数业务错误也可能是 400）
- 400：参数错误/业务错误（例如 `ORDER_INVALID_STATUS`、下单时 `DISH_NOT_FOUND`）
- 401：未携带或无效 Token（鉴权中间件拦截）
- 403：无权限（例如查看不属于自己的订单）
- 404：资源不存在（仅部分路由显式返回：菜谱详情/订单详情）
- 500：服务内部错误

## 认证

采用 Bearer Token（JWT）：

- 登录成功返回 `accessToken`
- 需要登录的接口请求头携带：`Authorization: Bearer <accessToken>`
- Token 有效期：7 天

## 数据模型

### User

```json
{
  "id": "u_2b0b9a6f7b31",
  "account": "13800000000",
  "name": "小熊",
  "loveMilli": 100000
}
```

字段：

- `id: string`
- `account: string`
- `name: string`
- `loveMilli: number`：单位为“千分之一爱心”，展示时 `loveMilli / 1000`

### Dish

```json
{
  "id": "tomato-egg",
  "name": "番茄炒蛋",
  "category": "home",
  "timeText": "10 分钟",
  "level": "easy",
  "tags": ["下饭", "孩子喜欢", "酸甜"],
  "priceCent": 1800,
  "story": "酸甜番茄遇上嫩滑鸡蛋，永远的家常顶流。",
  "imageUrl": "https://picsum.photos/seed/tomato-egg/1200/720",
  "badge": "经典",
  "createdBy": { "userId": "u_2b0b9a6f7b31", "name": "小熊" },
  "details": {
    "ingredients": ["番茄 2 个", "鸡蛋 3 个"],
    "steps": ["..."]
  }
}
```

字段：

- `id: string`
- `name: string`
- `category: "home" | "soup" | "sweet" | "quick"`
- `timeText: string`
- `level: "easy" | "medium" | "hard"`
- `tags: string[]`
- `priceCent: number`
- `story: string`
- `imageUrl: string`
- `badge: string`
- `createdBy?: { userId: string; name: string }`
- `details.ingredients: string[]`
- `details.steps: string[]`

### Order / OrderItem

```json
{
  "id": "ODK2F3A9",
  "createdAt": 1730000000000,
  "updatedAt": 1730000005000,
  "status": "placed",
  "placedBy": { "userId": "u_2b0b9a6f7b31", "name": "小熊" },
  "acceptedBy": { "userId": "u_9a8b7c6d5e4f", "name": "妈妈" },
  "finishedAt": 1730000010000,
  "finishImages": ["https://.../order/ODK2F3A9/1.jpg"],
  "review": { "rating": 5, "content": "好吃！", "images": ["https://.../r1.jpg"], "createdAt": 1730000020000 },
  "items": [
    { "dishId": "tomato-egg", "dishName": "番茄炒蛋", "qty": 2, "priceCent": 1800 }
  ],
  "totalCent": 3600
}
```

字段：

- `id: string`：订单号（8 位，字母数字，排除易混字符）
- `createdAt: number`
- `updatedAt: number`
- `status: "placed" | "accepted" | "done" | "cancelled"`
- `placedBy.userId: string`
- `placedBy.name: string`
- `placedNote?: string`
- `acceptedBy.userId?: string`
- `acceptedBy.name?: string`
- `finishedAt?: number`
- `finishImages?: string[]`
- `finishNote?: string`
- `review?: { rating: number; content: string; images: string[]; createdAt: number }`
- `items[].dishId: string`
- `items[].dishName: string`：下单时快照
- `items[].qty: number`
- `items[].priceCent: number`：下单时快照
- `totalCent: number`

## 路由列表

### Auth

#### 1) 注册

`POST /api/auth/register`

请求 body：

```json
{
  "account": "13800000000",
  "password": "123456",
  "passwordHash": "c4a6...（可选，优先使用）",
  "name": "小熊"
}
```

说明：

- `passwordHash`：`password` 的 SHA-256 十六进制字符串（64 位）；如果传了 `passwordHash`，后端会优先使用它

响应：

```json
{
  "success": true,
  "data": {
    "user": { "id": "u_2b0b9a6f7b31", "account": "13800000000", "name": "小熊", "loveMilli": 100000 }
  }
}
```

可能错误：

- `VALIDATION_ERROR`：缺少字段或字段类型不对
- `ACCOUNT_EXISTS`
- `INVALID_PASSWORD`

#### 2) 登录

`POST /api/auth/login`

请求 body：

```json
{
  "account": "13800000000",
  "passwordHash": "c4a6...（推荐）",
  "password": "123456（兼容旧版本，可选）"
}
```

响应：

```json
{
  "success": true,
  "data": {
    "accessToken": "JWT_TOKEN",
    "user": { "id": "u_2b0b9a6f7b31", "account": "13800000000", "name": "小熊", "loveMilli": 100000 }
  }
}
```

可能错误：

- `VALIDATION_ERROR`
- `INVALID_CREDENTIALS`

### Me

#### 3) 获取当前用户信息

`GET /api/me`

需要登录：是

响应：

```json
{
  "success": true,
  "data": {
    "user": { "id": "u_2b0b9a6f7b31", "account": "13800000000", "name": "小熊", "loveMilli": 100000 }
  }
}
```

可能错误：

- 401 + `UNAUTHORIZED`：未登录/Token 无效

### Dishes

#### 4) 获取菜谱列表

`GET /api/dishes`

查询参数（均可选）：

- `scope: "all" | "mine"`：默认 `all`；`mine` 表示只返回“我创建的菜谱”（需要登录）
- `category: "home" | "soup" | "sweet" | "quick"`
- `q: string`：关键词（匹配菜名或标签，包含匹配）
- `page: number`：正整数，默认 1；非法值会回退到默认值
- `pageSize: number`：正整数，默认 20；非法值会回退到默认值

响应：

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "tomato-egg",
        "name": "番茄炒蛋",
        "category": "home",
        "timeText": "10 分钟",
        "level": "easy",
        "tags": ["下饭", "孩子喜欢", "酸甜"],
        "priceCent": 1800,
        "story": "酸甜番茄遇上嫩滑鸡蛋，永远的家常顶流。",
        "imageUrl": "https://picsum.photos/seed/tomato-egg/1200/720",
        "badge": "经典",
        "createdBy": { "userId": "u_2b0b9a6f7b31", "name": "小熊" },
        "details": { "ingredients": ["..."], "steps": ["..."] }
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 6
  }
}
```

说明：

- 当前实现里 `items[]` 会携带 `details` 字段（前端可按需忽略）；如需仅返回列表字段，需要后续在后端显式剔除。
- `scope=mine` 时只会返回通过 `POST /api/dishes` 创建的菜谱（seed 菜谱不属于任何用户）

#### 5) 获取菜谱详情

`GET /api/dishes/:dishId`

路径参数：

- `dishId: string`

响应：

```json
{
  "success": true,
  "data": {
    "dish": {
      "id": "tomato-egg",
      "name": "番茄炒蛋",
      "category": "home",
      "timeText": "10 分钟",
      "level": "easy",
      "tags": ["下饭", "孩子喜欢", "酸甜"],
      "priceCent": 1800,
      "story": "酸甜番茄遇上嫩滑鸡蛋，永远的家常顶流。",
      "imageUrl": "https://picsum.photos/seed/tomato-egg/1200/720",
      "badge": "经典",
      "createdBy": { "userId": "u_2b0b9a6f7b31", "name": "小熊" },
      "details": { "ingredients": ["..."], "steps": ["..."] }
    }
  }
}
```

可能错误：

- 404 + `DISH_NOT_FOUND`（`error.details.dishId` 会回传）

#### 6) 创建菜谱

`POST /api/dishes`

需要登录：是

请求 body：

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
{
  "success": true,
  "data": {
    "dish": { "id": "scallion-noodle", "name": "葱油拌面", "createdBy": { "userId": "u_2b0b9a6f7b31", "name": "小熊" } }
  }
}
```

#### 6.1) 删除我的菜谱

`DELETE /api/dishes/:dishId`

需要登录：是

说明：

- 仅允许删除自己创建的菜谱（`dish.createdBy.userId === me.id`）

响应：

```json
{
  "success": true,
  "data": { "dish": { "id": "scallion-noodle" } }
}
```

可能错误：

- 404 + `DISH_NOT_FOUND`
- 403 + `UNAUTHORIZED`：无权限删除该菜谱

### Orders

#### 7) 创建订单（下单）

`POST /api/orders`

需要登录：是

请求 body：

```json
{
  "items": [
    { "dishId": "tomato-egg", "qty": 2 },
    { "dishId": "milk-pudding", "qty": 1 }
  ],
  "note": "少盐少油，不要香菜"
}
```

响应：

```json
{
  "success": true,
  "data": {
    "order": {
      "id": "ODK2F3A9",
      "createdAt": 1730000000000,
      "updatedAt": 1730000000000,
      "status": "placed",
      "placedBy": { "userId": "u_2b0b9a6f7b31", "name": "小熊" },
      "placedNote": "少盐少油，不要香菜",
      "items": [
        { "dishId": "tomato-egg", "dishName": "番茄炒蛋", "qty": 2, "priceCent": 1800 }
      ],
      "totalCent": 3600
    },
    "me": { "id": "u_2b0b9a6f7b31", "account": "13800000000", "name": "小熊", "loveMilli": 96400 }
  }
}
```

可能错误：

- `VALIDATION_ERROR`：`items` 不是数组/为空
- `DISH_NOT_FOUND`：下单项中的菜谱不存在（400，`error.details.dishId` 会回传）
- `INVALID_QTY`：`qty` 不是正整数（400，`error.details.dishId` 会回传）
- `INSUFFICIENT_LOVE`：爱心值不足（`error.details.loveMilli` / `error.details.required`）

#### 8) 获取订单列表

`GET /api/orders`

需要登录：是

查询参数（均可选）：

- `scope: "mine" | "all"`：默认 `mine`
- `status: "placed" | "accepted" | "done" | "cancelled"`
- `page: number`：正整数，默认 1；非法值回退
- `pageSize: number`：正整数，默认 20；非法值回退

响应：

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": "ODK2F3A9",
        "createdAt": 1730000000000,
        "updatedAt": 1730000005000,
        "status": "accepted",
        "placedBy": { "userId": "u_2b0b9a6f7b31", "name": "小熊" },
        "acceptedBy": { "userId": "u_9a8b7c6d5e4f", "name": "妈妈" },
        "items": [
          { "dishId": "tomato-egg", "dishName": "番茄炒蛋", "qty": 2, "priceCent": 1800 }
        ],
        "totalCent": 3600
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 3
  }
}
```

说明：

- `scope=all` 当前实现不做角色区分：任何登录用户都可以请求全量订单（如果前端需要“厨师端/管理员端”能力，建议后续补角色与权限控制）。

#### 9) 获取订单详情

`GET /api/orders/:orderId`

需要登录：是

路径参数：

- `orderId: string`

响应：

```json
{
  "success": true,
  "data": {
    "order": {
      "id": "ODK2F3A9",
      "createdAt": 1730000000000,
      "updatedAt": 1730000005000,
      "status": "accepted",
      "placedBy": { "userId": "u_2b0b9a6f7b31", "name": "小熊" },
      "acceptedBy": { "userId": "u_9a8b7c6d5e4f", "name": "妈妈" },
      "items": [
        { "dishId": "tomato-egg", "dishName": "番茄炒蛋", "qty": 2, "priceCent": 1800 }
      ],
      "totalCent": 3600
    }
  }
}
```

可能错误：

- 404 + `ORDER_NOT_FOUND`（`error.details.orderId` 会回传）
- 403 + `UNAUTHORIZED`：订单不属于当前用户（`message` 为“无权限查看该订单”）

#### 10) 接单

`POST /api/orders/:orderId/accept`

需要登录：是

响应：

```json
{
  "success": true,
  "data": {
    "order": {
      "id": "ODK2F3A9",
      "status": "accepted",
      "updatedAt": 1730000005000,
      "placedBy": { "userId": "u_2b0b9a6f7b31", "name": "小熊" },
      "acceptedBy": { "userId": "u_9a8b7c6d5e4f", "name": "妈妈" }
    }
  }
}
```

可能错误：

- `ORDER_NOT_FOUND`
- `ORDER_INVALID_STATUS`（`error.details.status` 会回传当前状态）

#### 11) 取消订单（仅下单人，且未接单）

`POST /api/orders/:orderId/cancel`

需要登录：是

响应：

```json
{
  "success": true,
  "data": {
    "order": { "id": "ODK2F3A9", "status": "cancelled", "updatedAt": 1730000005000 },
    "me": { "id": "u_123", "loveMilli": 100000 }
  }
}
```

可能错误：

- `ORDER_NOT_FOUND`
- `ORDER_INVALID_STATUS`（仅允许 `status=placed`）
- 403 + `UNAUTHORIZED`：非下单人取消

#### 12) 完成订单（支持上传图片）

`POST /api/orders/:orderId/finish`

需要登录：是

请求：

- `multipart/form-data`
- 字段：
  - `images`：图片文件（可多张，最多 3 张）
  - `note`：可选文字备注

响应：

```json
{
  "success": true,
  "data": {
    "order": {
      "id": "ODK2F3A9",
      "status": "done",
      "updatedAt": 1730000010000,
      "finishedAt": 1730000010000,
      "finishImages": ["https://.../order/ODK2F3A9/1.jpg"]
    },
    "me": { "id": "u_9a8b7c6d5e4f", "account": "13800000000", "name": "妈妈", "loveMilli": 100000 }
  }
}
```

可能错误：

- `ORDER_NOT_FOUND`
- `ORDER_INVALID_STATUS`（`error.details.status` 会回传当前状态）

#### 13) 订单评价（支持上传图片）

`POST /api/orders/:orderId/review`

需要登录：是

请求：

- `multipart/form-data`
- 字段：
  - `rating`：1-5
  - `content`：评价文字
  - `images`：评价图片（可多张，最多 3 张）

响应：

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

可能错误：

- `ORDER_NOT_FOUND`
- `ORDER_INVALID_STATUS`（仅允许 `status=done`）
- `ORDER_ALREADY_REVIEWED`
- 403 + `UNAUTHORIZED`：非下单人评价

#### 14) 订单状态实时推送（SSE）

`GET /api/orders/stream?scope=mine`

需要登录：是

查询参数：

- `scope: "mine" | "all"`：默认 `mine`

响应：

- `Content-Type: text/event-stream; charset=utf-8`
- 连接会每 15 秒发送一次 ping：

```
event: ping
data: 1730000000000

```

- 当订单状态更新（接单/完成）会推送一条消息（仅使用 `data:` 字段）：

```
data: {"type":"order.updated","data":{"orderId":"ODK2F3A9","status":"accepted","updatedAt":1730000005000}}

```

事件结构（`data:` 内的 JSON）：

```json
{
  "type": "order.updated",
  "data": {
    "orderId": "ODK2F3A9",
    "status": "accepted",
    "updatedAt": 1730000005000
  }
}
```

#### 15) 订单状态实时推送（WebSocket）

`GET ws(s)://<host>/api/ws/orders?scope=mine&token=<JWT>`

需要登录：是

查询参数：

- `scope: "mine" | "all"`：默认 `mine`
- `token: string`：登录接口返回的 `accessToken`（JWT）；也兼容 `token=Bearer <JWT>` 形式

消息格式（WebSocket 文本消息为 JSON）：

```json
{
  "type": "order.updated",
  "data": {
    "orderId": "ODK2F3A9",
    "status": "accepted",
    "updatedAt": 1730000005000
  }
}
```

连接建立后，服务端会先推送现有订单快照（最多 50 条）：

```json
{
  "type": "order.snapshot",
  "data": {
    "order": {
      "id": "ODK2F3A9",
      "createdAt": 1730000000000,
      "updatedAt": 1730000005000,
      "status": "accepted",
      "placedBy": { "userId": "u_xxx", "name": "小熊" },
      "items": [{ "dishId": "tomato-egg", "dishName": "番茄炒蛋", "qty": 2, "priceCent": 1800 }],
      "totalCent": 3600
    }
  }
}
```
