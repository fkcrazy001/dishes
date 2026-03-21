# 家庭菜谱 + 点菜系统（前端原型）API 需求说明

本文档描述前端需要后端提供的接口与数据格式，用于替换当前原型中的本地模拟数据与状态。

## 统一约定

### Base URL

`/api`

### 时间与金额

- 时间戳：`createdAt` / `updatedAt` 使用毫秒时间戳（`number`）
- 金额：统一使用“分”存储（`amountCent: number`），前端展示时再格式化为 `¥xx.xx`

### 枚举

- 订单状态 `OrderStatus`：
  - `placed`：已下单（待接单）
  - `accepted`：制作中（已接单）
  - `done`：已完成
  - `cancelled`：已取消（可选）

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

### 认证

采用 Bearer Token：

- 登录成功返回 `accessToken`
- 需要登录的接口请求头携带：`Authorization: Bearer <accessToken>`

## 数据模型

### Dish（菜谱）

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
  "imageUrl": "https://cdn.example.com/dishes/tomato-egg.jpg",
  "badge": "经典",
  "details": {
    "ingredients": ["番茄 2 个", "鸡蛋 3 个", "葱花 少许"],
    "steps": ["番茄切块，鸡蛋打散。", "鸡蛋滑炒盛出。", "炒番茄出汁调味，倒回鸡蛋翻匀。"]
  }
}
```

字段说明：
- `category`：`home | soup | sweet | quick`
- `level`：建议后端用枚举 `easy | medium | hard`，前端映射为 `简单/中等/困难`
- `imageUrl`：点击图片弹窗展示大图；也可提供多图（可选扩展 `imageUrls: string[]`）

### Order（订单）

```json
{
  "id": "ODK2F3A9",
  "createdAt": 1730000000000,
  "updatedAt": 1730000005000,
  "status": "placed",
  "placedBy": {
    "userId": "u_123",
    "name": "小熊"
  },
  "items": [
    { "dishId": "tomato-egg", "dishName": "番茄炒蛋", "qty": 2, "priceCent": 1800 },
    { "dishId": "milk-pudding", "dishName": "桂花牛奶布丁", "qty": 1, "priceCent": 2000 }
  ],
  "totalCent": 5600
}
```

说明：
- `items[].dishName/priceCent` 推荐做快照，避免菜谱变更影响历史订单展示

### User（用户）

```json
{
  "id": "u_123",
  "account": "13800000000",
  "name": "小熊"
}
```

## 接口列表

### 1) 注册

`POST /api/auth/register`

请求：

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
    "user": { "id": "u_123", "account": "13800000000", "name": "小熊" }
  }
}
```

常见错误：
- `ACCOUNT_EXISTS`
- `INVALID_PASSWORD`

### 2) 登录

`POST /api/auth/login`

请求：

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
    "accessToken": "JWT_OR_OPAQUE_TOKEN",
    "user": { "id": "u_123", "account": "13800000000", "name": "小熊" }
  }
}
```

常见错误：
- `INVALID_CREDENTIALS`

### 3) 获取当前用户信息

`GET /api/me`

响应：

```json
{
  "success": true,
  "data": {
    "user": { "id": "u_123", "account": "13800000000", "name": "小熊" }
  }
}
```

### 4) 获取菜谱列表（点菜页）

`GET /api/dishes?category=home&q=番茄&page=1&pageSize=20`

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
        "tags": ["下饭", "孩子喜欢"],
        "priceCent": 1800,
        "story": "酸甜番茄遇上嫩滑鸡蛋…",
        "imageUrl": "https://cdn.example.com/dishes/tomato-egg.jpg",
        "badge": "经典"
      }
    ],
    "page": 1,
    "pageSize": 20,
    "total": 100
  }
}
```

说明：
- 列表接口可不返回 `details`，点击图片弹窗时再请求详情

### 5) 获取菜谱详情（点击图片弹窗）

`GET /api/dishes/{dishId}`

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
      "imageUrl": "https://cdn.example.com/dishes/tomato-egg.jpg",
      "badge": "经典",
      "details": {
        "ingredients": ["番茄 2 个", "鸡蛋 3 个", "葱花 少许"],
        "steps": ["番茄切块，鸡蛋打散。", "鸡蛋滑炒盛出。", "炒番茄出汁调味，倒回鸡蛋翻匀。"]
      }
    }
  }
}
```

### 6) 创建订单（下单）

`POST /api/orders`

请求：

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
        { "dishId": "tomato-egg", "dishName": "番茄炒蛋", "qty": 2, "priceCent": 1800 },
        { "dishId": "milk-pudding", "dishName": "桂花牛奶布丁", "qty": 1, "priceCent": 2000 }
      ],
      "totalCent": 5600
    }
  }
}
```

### 7) 获取我的订单列表（下单人看状态）

`GET /api/orders?scope=mine&status=placed&page=1&pageSize=20`

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
- 前端可轮询该接口或用 WebSocket/SSE 做实时更新

### 8) 获取订单详情

`GET /api/orders/{orderId}`

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

### 9) 接单（做菜）

`POST /api/orders/{orderId}/accept`

响应：

```json
{
  "success": true,
  "data": {
    "order": { "id": "ODK2F3A9", "status": "accepted", "updatedAt": 1730000005000 }
  }
}
```

### 10) 完成订单（做菜）

`POST /api/orders/{orderId}/finish`

响应：

```json
{
  "success": true,
  "data": {
    "order": { "id": "ODK2F3A9", "status": "done", "updatedAt": 1730000010000 }
  }
}
```

### 11) 订单状态实时推送（可选）

建议支持其一：

- SSE：`GET /api/orders/stream?scope=mine`
- WebSocket：`wss://.../api/ws`

事件格式：

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

