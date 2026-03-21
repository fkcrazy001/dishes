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
- `ORDER_NOT_FOUND`：订单不存在
- `ORDER_INVALID_STATUS`：订单状态不允许该操作
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
  "name": "小熊"
}
```

字段：

- `id: string`
- `account: string`
- `name: string`

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
  "name": "小熊"
}
```

响应：

```json
{
  "success": true,
  "data": {
    "user": { "id": "u_2b0b9a6f7b31", "account": "13800000000", "name": "小熊" }
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
  "password": "123456"
}
```

响应：

```json
{
  "success": true,
  "data": {
    "accessToken": "JWT_TOKEN",
    "user": { "id": "u_2b0b9a6f7b31", "account": "13800000000", "name": "小熊" }
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
    "user": { "id": "u_2b0b9a6f7b31", "account": "13800000000", "name": "小熊" }
  }
}
```

可能错误：

- 401 + `UNAUTHORIZED`：未登录/Token 无效

### Dishes

#### 4) 获取菜谱列表

`GET /api/dishes`

查询参数（均可选）：

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
      "details": { "ingredients": ["..."], "steps": ["..."] }
    }
  }
}
```

可能错误：

- 404 + `DISH_NOT_FOUND`（`error.details.dishId` 会回传）

### Orders

#### 6) 创建订单（下单）

`POST /api/orders`

需要登录：是

请求 body：

```json
{
  "items": [
    { "dishId": "tomato-egg", "qty": 2 },
    { "dishId": "milk-pudding", "qty": 1 }
  ]
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
      "items": [
        { "dishId": "tomato-egg", "dishName": "番茄炒蛋", "qty": 2, "priceCent": 1800 }
      ],
      "totalCent": 3600
    }
  }
}
```

可能错误：

- `VALIDATION_ERROR`：`items` 不是数组/为空
- `DISH_NOT_FOUND`：下单项中的菜谱不存在（400，`error.details.dishId` 会回传）
- `INVALID_QTY`：`qty` 不是正整数（400，`error.details.dishId` 会回传）

#### 7) 获取订单列表

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

#### 8) 获取订单详情

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

#### 9) 接单

`POST /api/orders/:orderId/accept`

需要登录：是

响应：

```json
{
  "success": true,
  "data": {
    "order": { "id": "ODK2F3A9", "status": "accepted", "updatedAt": 1730000005000 }
  }
}
```

可能错误：

- `ORDER_NOT_FOUND`
- `ORDER_INVALID_STATUS`（`error.details.status` 会回传当前状态）

#### 10) 完成订单

`POST /api/orders/:orderId/finish`

需要登录：是

响应：

```json
{
  "success": true,
  "data": {
    "order": { "id": "ODK2F3A9", "status": "done", "updatedAt": 1730000010000 }
  }
}
```

可能错误：

- `ORDER_NOT_FOUND`
- `ORDER_INVALID_STATUS`（`error.details.status` 会回传当前状态）

#### 11) 订单状态实时推送（SSE）

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

