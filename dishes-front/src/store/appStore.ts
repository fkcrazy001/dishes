import { computed, reactive, ref } from 'vue'

export type CategoryId = 'all' | 'home' | 'soup' | 'sweet' | 'quick'
export type PageMode = 'order' | 'cook' | 'mine' | 'dishes'

export type DishLevel = 'easy' | 'medium' | 'hard'

export type DishDetails = {
  ingredients: string[]
  steps: string[]
}

export type Dish = {
  id: string
  name: string
  category: Exclude<CategoryId, 'all'>
  timeText: string
  level: DishLevel
  tags: string[]
  priceCent: number
  story: string
  imageUrl: string
  badge: string
  details: DishDetails
  createdBy?: { userId: string; name: string }
}

export type OrderStatus = 'placed' | 'accepted' | 'done' | 'cancelled'

export type OrderItem = {
  dishId: string
  dishName: string
  qty: number
  priceCent: number
}

export type Order = {
  id: string
  createdAt: number
  updatedAt: number
  status: OrderStatus
  placedBy?: { userId: string; name: string }
  acceptedBy?: { userId: string; name: string }
  placedNote?: string
  finishedAt?: number
  finishImages?: string[]
  review?: { rating: number; content: string; images?: string[]; createdAt: number }
  items: OrderItem[]
  totalCent: number
}

export type User = {
  id: string
  account: string
  name: string
  loveMilli: number
}

export type UserLoveRankItem = {
  id: string
  name: string
  loveMilli: number
}

type ApiErrorBody = { code: string; message: string; details?: unknown }
type ApiSuccess<T> = { success: true; data: T }
type ApiFailure = { success: false; error: ApiErrorBody }
type ApiResponse<T> = ApiSuccess<T> | ApiFailure

class ApiException extends Error {
  code: string
  status: number
  details?: unknown

  constructor(input: { code: string; message: string; status: number; details?: unknown }) {
    super(input.message)
    this.code = input.code
    this.status = input.status
    this.details = input.details
  }
}

const TOKEN_KEY = 'dishes_access_token'

const token = ref<string | null>(localStorage.getItem(TOKEN_KEY) ?? null)

const categories: { id: CategoryId; name: string }[] = [
  { id: 'all', name: '全部' },
  { id: 'home', name: '家常' },
  { id: 'soup', name: '汤羹' },
  { id: 'sweet', name: '甜点' },
  { id: 'quick', name: '快手' },
]

const auth = reactive({
  loggedIn: Boolean(token.value),
  userId: '',
  account: '',
  name: '',
  loveMilli: 100000,
})

const dishes = ref<Dish[]>([])
const dishesTotal = ref(0)
const dishesLoading = ref(false)
const dishDetailById = ref<Record<string, Dish>>({})
const dishesMine = ref<Dish[]>([])

const cart = ref<Record<string, number>>({})
const ordersMine = ref<Order[]>([])
const ordersAll = ref<Order[]>([])
const mode = ref<PageMode>('order')
const usersLoveRank = ref<UserLoveRankItem[]>([])

const toastText = ref('')
const toastShow = ref(false)
let toastTimer: number | undefined

const showToast = (text: string) => {
  toastText.value = text
  toastShow.value = true
  if (toastTimer) window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => {
    toastShow.value = false
  }, 1500)
}

type DialogTone = 'info' | 'error' | 'success'

const dialogShow = ref(false)
const dialogTitle = ref('')
const dialogMessage = ref('')
const dialogTone = ref<DialogTone>('info')

const openDialog = (input: { title?: string; message: string; tone?: DialogTone }) => {
  dialogTitle.value = input.title ?? '提示'
  dialogMessage.value = input.message
  dialogTone.value = input.tone ?? 'info'
  dialogShow.value = true
}

const closeDialog = () => {
  dialogShow.value = false
  dialogTitle.value = ''
  dialogMessage.value = ''
  dialogTone.value = 'info'
}

const getFriendlyErrorMessage = (e: unknown, fallback: string) => {
  if (e instanceof ApiException) {
    if (e.code === 'INVALID_CREDENTIALS') return '账号或密码错误'
    if (e.code === 'ACCOUNT_EXISTS') return '这个账号已经注册过啦～'
    if (e.code === 'INVALID_PASSWORD') return '密码至少 6 位哦'
    if (e.code === 'VALIDATION_ERROR') return '信息填写不太对，请检查一下'
    if (e.code === 'UNAUTHORIZED') return '登录状态已失效，请重新登录'
    if (e.code === 'DISH_NOT_FOUND') return '这道菜找不到了，刷新一下试试'
    if (e.code === 'INVALID_QTY') return '数量不太对，请调整后再试'
    if (e.code === 'INSUFFICIENT_LOVE') return '爱心值不够啦～先攒一攒再下单'
    if (e.code === 'ORDER_NOT_FOUND') return '订单找不到了，可能已被处理'
    if (e.code === 'ORDER_INVALID_STATUS') return '当前订单状态不支持这个操作'
    if (e.message) return e.message
    return fallback
  }
  const err = e as { message?: string }
  return err?.message ?? fallback
}

const openErrorDialog = (title: string, e: unknown, fallback: string) => {
  openDialog({ title, message: getFriendlyErrorMessage(e, fallback), tone: 'error' })
}

const loveMilliFromCent = (cent: number) => cent

const formatLoveMilli = (milli: number) => {
  const v = milli / 1000
  return Number.isInteger(v) ? String(v) : v.toFixed(1)
}

const formatMoneyCent = (cent: number) => {
  const yuan = cent / 100
  const s = Number.isInteger(yuan) ? String(yuan) : yuan.toFixed(2)
  return `¥${s}`
}

const dishLevelText = (level: DishLevel) => {
  if (level === 'easy') return '简单'
  if (level === 'medium') return '中等'
  return '困难'
}

const safeParseJson = async (res: Response) => {
  const text = await res.text()
  if (!text) return null
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}

const sha256Hex = async (text: string): Promise<string | null> => {
  const subtle = globalThis.crypto?.subtle
  if (!subtle) return null
  if (globalThis.isSecureContext === false) return null
  const data = new TextEncoder().encode(text)
  const buf = await subtle.digest('SHA-256', data)
  return Array.from(new Uint8Array(buf))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('')
}

const apiFetch = async <T>(input: { method: string; path: string; body?: unknown; auth?: boolean }): Promise<T> => {
  const headers: Record<string, string> = { Accept: 'application/json' }
  const isForm = typeof FormData !== 'undefined' && input.body instanceof FormData
  if (input.body !== undefined && !isForm) headers['Content-Type'] = 'application/json'
  if (input.auth !== false && token.value) headers.Authorization = `Bearer ${token.value}`

  const res = await fetch(`/api${input.path}`, {
    method: input.method,
    headers,
    body: input.body === undefined ? undefined : isForm ? (input.body as FormData) : JSON.stringify(input.body),
  })

  const json = (await safeParseJson(res)) as ApiResponse<T> | null

  if (res.status === 401) {
    token.value = null
    localStorage.removeItem(TOKEN_KEY)
    auth.loggedIn = false
    auth.userId = ''
    auth.account = ''
    auth.name = ''
    auth.loveMilli = 100000
  }

  if (!json) {
    throw new ApiException({ code: 'INTERNAL_ERROR', message: '服务响应解析失败', status: res.status })
  }
  if (!json.success) {
    throw new ApiException({
      code: json.error.code,
      message: json.error.message,
      status: res.status,
      details: json.error.details,
    })
  }
  return json.data
}

const buildQuery = (params: Record<string, string | number | undefined>) => {
  const usp = new URLSearchParams()
  Object.entries(params).forEach(([k, v]) => {
    if (v === undefined || v === '') return
    usp.set(k, String(v))
  })
  const s = usp.toString()
  return s ? `?${s}` : ''
}

const bootstrap = async () => {
  if (!token.value) return
  try {
    const data = await apiFetch<{ user: User }>({ method: 'GET', path: '/me' })
    auth.loggedIn = true
    auth.userId = data.user.id
    auth.account = data.user.account
    auth.name = data.user.name
    auth.loveMilli = data.user.loveMilli
  } catch {
    return
  }
}

const refreshMe = async () => {
  const data = await apiFetch<{ user: User }>({ method: 'GET', path: '/me' })
  auth.loggedIn = true
  auth.userId = data.user.id
  auth.account = data.user.account
  auth.name = data.user.name
  auth.loveMilli = data.user.loveMilli
}

const loginWithPassword = async (input: { account: string; password: string; remember: boolean }) => {
  let passwordHash: string | null = null
  try {
    passwordHash = await sha256Hex(input.password)
  } catch {
    passwordHash = null
  }
  const data = await apiFetch<{ accessToken: string; user: User }>({
    method: 'POST',
    path: '/auth/login',
    body: passwordHash ? { account: input.account, passwordHash } : { account: input.account, password: input.password },
    auth: false,
  })
  token.value = data.accessToken
  if (input.remember) localStorage.setItem(TOKEN_KEY, data.accessToken)
  else localStorage.removeItem(TOKEN_KEY)

  auth.loggedIn = true
  auth.userId = data.user.id
  auth.account = data.user.account
  auth.name = data.user.name
  auth.loveMilli = data.user.loveMilli
  cart.value = {}
  ordersMine.value = []
  ordersAll.value = []
  mode.value = 'order'
}

const register = async (input: { account: string; password: string; name: string }) => {
  const data = await apiFetch<{ user: User }>({
    method: 'POST',
    path: '/auth/register',
    body: { account: input.account, password: input.password, name: input.name },
    auth: false,
  })
  return data.user
}

const logout = () => {
  token.value = null
  localStorage.removeItem(TOKEN_KEY)
  auth.loggedIn = false
  auth.userId = ''
  auth.account = ''
  auth.name = ''
  auth.loveMilli = 100000
  cart.value = {}
  dishesMine.value = []
  ordersMine.value = []
  ordersAll.value = []
  mode.value = 'order'
}

const fetchDishes = async (input: { category?: Exclude<CategoryId, 'all'>; q?: string; page?: number; pageSize?: number }) => {
  dishesLoading.value = true
  try {
    const data = await apiFetch<{ items: Dish[]; page: number; pageSize: number; total: number }>({
      method: 'GET',
      path: `/dishes${buildQuery({ category: input.category, q: input.q, page: input.page ?? 1, pageSize: input.pageSize ?? 20 })}`,
      auth: false,
    })
    dishes.value = data.items
    dishesTotal.value = data.total
    const nextCache: Record<string, Dish> = { ...dishDetailById.value }
    data.items.forEach((d) => {
      nextCache[d.id] = d
    })
    dishDetailById.value = nextCache
  } finally {
    dishesLoading.value = false
  }
}

const fetchDishDetail = async (dishId: string) => {
  const cached = dishDetailById.value[dishId]
  if (cached && cached.details?.ingredients?.length) return cached
  const data = await apiFetch<{ dish: Dish }>({ method: 'GET', path: `/dishes/${encodeURIComponent(dishId)}`, auth: false })
  dishDetailById.value = { ...dishDetailById.value, [dishId]: data.dish }
  return data.dish
}

const createDish = async (input: {
  name: string
  category: Exclude<CategoryId, 'all'>
  timeText: string
  level: DishLevel
  tags: string[]
  priceCent: number
  story: string
  imageUrl?: string
  badge?: string
  details: DishDetails
}) => {
  const data = await apiFetch<{ dish: Dish }>({
    method: 'POST',
    path: '/dishes',
    body: {
      name: input.name,
      category: input.category,
      timeText: input.timeText,
      level: input.level,
      tags: input.tags,
      priceCent: input.priceCent,
      story: input.story,
      imageUrl: input.imageUrl ?? '',
      badge: input.badge ?? '',
      details: input.details,
    },
  })

  dishes.value = [data.dish, ...dishes.value]
  dishesTotal.value = dishesTotal.value + 1
  dishDetailById.value = { ...dishDetailById.value, [data.dish.id]: data.dish }
  dishesMine.value = [data.dish, ...dishesMine.value]
  return data.dish
}

const fetchMyDishes = async (input?: { page?: number; pageSize?: number }) => {
  const page = input?.page ?? 1
  const pageSize = input?.pageSize ?? 50
  const data = await apiFetch<{ items: Dish[]; page: number; pageSize: number; total: number }>({
    method: 'GET',
    path: `/dishes${buildQuery({ scope: 'mine', page, pageSize })}`,
  })
  dishesMine.value = data.items
  data.items.forEach((d) => {
    dishDetailById.value = { ...dishDetailById.value, [d.id]: d }
  })
  return data
}

const fetchUsersLoveRank = async (input?: { limit?: number }) => {
  const data = await apiFetch<{ items: UserLoveRankItem[] }>({
    method: 'GET',
    path: `/users${buildQuery({ sort: 'loveMilli_desc', limit: input?.limit ?? 50 })}`,
  })
  usersLoveRank.value = data.items
  return data.items
}

const deleteDish = async (dishId: string) => {
  await apiFetch<unknown>({ method: 'DELETE', path: `/dishes/${encodeURIComponent(dishId)}` })
  dishesMine.value = dishesMine.value.filter((d) => d.id !== dishId)
  dishes.value = dishes.value.filter((d) => d.id !== dishId)
  const nextCache = { ...dishDetailById.value }
  delete nextCache[dishId]
  dishDetailById.value = nextCache
}

const createDishImage = (dish: { id: string; name: string; imageUrl?: string }) => {
  if (dish.imageUrl) return dish.imageUrl
  const hash = Array.from(dish.id).reduce((s, ch) => s + ch.charCodeAt(0), 0)
  const colors = [
    ['#f6d365', '#fda085', '#ff9a9e'],
    ['#f2a65a', '#e56a54', '#f6d365'],
    ['#ffecd2', '#fcb69f', '#f6d365'],
    ['#cfd9df', '#e2ebf0', '#f6d365'],
  ] as const
  const palette = colors[hash % colors.length] ?? colors[0]
  const [c1, c2, c3] = palette
  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" width="1200" height="720" viewBox="0 0 1200 720">
      <defs>
        <linearGradient id="g" x1="0" x2="1" y1="0" y2="1">
          <stop offset="0" stop-color="${c1}"/>
          <stop offset="0.55" stop-color="${c2}"/>
          <stop offset="1" stop-color="${c3}"/>
        </linearGradient>
        <filter id="s" x="-20%" y="-20%" width="140%" height="140%">
          <feDropShadow dx="0" dy="20" stdDeviation="24" flood-color="rgba(45,31,23,0.22)"/>
        </filter>
      </defs>
      <rect width="1200" height="720" fill="url(#g)"/>
      <circle cx="980" cy="140" r="120" fill="rgba(255,255,255,0.20)"/>
      <circle cx="210" cy="560" r="180" fill="rgba(255,255,255,0.16)"/>
      <g filter="url(#s)">
        <rect x="120" y="190" width="960" height="360" rx="44" fill="rgba(255,255,255,0.38)"/>
      </g>
      <text x="160" y="360" font-size="66" font-family="ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, PingFang SC, Hiragino Sans GB, Microsoft YaHei, Arial" fill="rgba(45,31,23,0.92)" font-weight="900">${dish.name}</text>
      <text x="160" y="430" font-size="34" font-family="ui-sans-serif, system-ui, -apple-system, Segoe UI, Roboto, PingFang SC, Hiragino Sans GB, Microsoft YaHei, Arial" fill="rgba(45,31,23,0.65)" font-weight="700">家庭厨房</text>
    </svg>
  `.trim()
  return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(svg)}`
}

const getDishById = (dishId: string) => dishDetailById.value[dishId] ?? dishes.value.find((d) => d.id === dishId) ?? null

const orderItems = computed(() => {
  const map = cart.value
  return dishes.value
    .map((d) => ({ dish: d, qty: map[d.id] ?? 0 }))
    .filter((x) => x.qty > 0)
    .sort((a, b) => b.qty - a.qty)
})

const cartTotalCent = computed(() => orderItems.value.reduce((s, x) => s + x.dish.priceCent * x.qty, 0))

const addToCart = (dishId: string) => {
  const next: Record<string, number> = { ...cart.value }
  next[dishId] = (next[dishId] ?? 0) + 1
  cart.value = next
  const d = getDishById(dishId)
  showToast(d ? `已加入：${d.name}` : '已加入')
}

const updateCartQty = (dishId: string, delta: number) => {
  const curr = cart.value[dishId] ?? 0
  const nextQty = curr + delta
  const next: Record<string, number> = { ...cart.value }
  if (nextQty <= 0) delete next[dishId]
  else next[dishId] = nextQty
  cart.value = next
}

const clearCart = () => {
  if (orderItems.value.length === 0) return
  cart.value = {}
  showToast('已清空点单')
}

const dedupeOrders = (input: Order[]) => {
  const indexById = new Map<string, number>()
  const out: Order[] = []

  input.forEach((o) => {
    const idx = indexById.get(o.id)
    if (idx === undefined) {
      indexById.set(o.id, out.length)
      out.push(o)
      return
    }
    const prev = out[idx]!
    if (o.updatedAt > prev.updatedAt) out[idx] = o
  })

  return out
}

const fetchOrders = async (input: { scope: 'mine' | 'all'; status?: OrderStatus; page?: number; pageSize?: number }) => {
  const data = await apiFetch<{ items: Order[]; page: number; pageSize: number; total: number }>({
    method: 'GET',
    path: `/orders${buildQuery({ scope: input.scope, status: input.status, page: input.page ?? 1, pageSize: input.pageSize ?? 20 })}`,
  })
  const items = dedupeOrders(data.items)
  if (input.scope === 'mine') ordersMine.value = items
  else ordersAll.value = items
  return data
}

const fetchAllOrders = async (scope: 'mine' | 'all') => {
  const pageSize = 50
  let page = 1
  let total = 0
  const all: Order[] = []

  for (let i = 0; i < 50; i += 1) {
    const data = await apiFetch<{ items: Order[]; page: number; pageSize: number; total: number }>({
      method: 'GET',
      path: `/orders${buildQuery({ scope, page, pageSize })}`,
    })
    total = data.total
    all.push(...data.items)
    if (data.items.length < pageSize) break
    if (page * pageSize >= total) break
    page += 1
  }

  const items = dedupeOrders(all).sort((a, b) => b.createdAt - a.createdAt)
  if (scope === 'mine') ordersMine.value = items
  else ordersAll.value = items
  return { items, total }
}

const fetchOrderDetail = async (orderId: string) => {
  const data = await apiFetch<{ order: Order }>({ method: 'GET', path: `/orders/${encodeURIComponent(orderId)}` })
  return data.order
}

const upsertOrder = (scope: 'mine' | 'all', order: Order) => {
  const list = dedupeOrders(scope === 'mine' ? ordersMine.value : ordersAll.value)
  const idx = list.findIndex((o) => o.id === order.id)
  const next =
    idx >= 0
      ? [
          ...list.slice(0, idx),
          { ...list[idx], ...order, updatedAt: Math.max(list[idx]!.updatedAt, order.updatedAt) },
          ...list.slice(idx + 1),
        ]
      : [order, ...list]
  if (scope === 'mine') ordersMine.value = next.slice(0, 50)
  else ordersAll.value = next.slice(0, 200)
}

const patchOrderStatus = async (scope: 'mine' | 'all', patch: { orderId: string; status: OrderStatus; updatedAt: number }) => {
  const list = dedupeOrders(scope === 'mine' ? ordersMine.value : ordersAll.value)
  const idx = list.findIndex((o) => o.id === patch.orderId)
  if (idx >= 0) {
    const curr = list[idx]!
    if (patch.updatedAt <= curr.updatedAt) {
      if (scope === 'mine') ordersMine.value = list
      else ordersAll.value = list
      return
    }
    const next = { ...curr, status: patch.status, updatedAt: patch.updatedAt }
    const merged = [...list.slice(0, idx), next, ...list.slice(idx + 1)]
    if (scope === 'mine') ordersMine.value = merged
    else ordersAll.value = merged
    const needPlacedBy = !next.placedBy?.userId
    const needAcceptedBy = (patch.status === 'accepted' || patch.status === 'done') && !next.acceptedBy?.userId
    const needFinishInfo = patch.status === 'done' && (!next.finishedAt || !next.finishImages)
    if (needPlacedBy || needAcceptedBy || needFinishInfo) {
      try {
        const full = await fetchOrderDetail(patch.orderId)
        upsertOrder(scope, full)
      } catch {
        return
      }
    }
    return
  }
  try {
    const full = await fetchOrderDetail(patch.orderId)
    upsertOrder(scope, full)
  } catch {
    return
  }
}

type OrderWsEvent =
  | { type: 'order.updated'; data: { orderId: string; status: OrderStatus; updatedAt: number } }
  | { type: 'order.snapshot'; data: { order: Order } }

const wsMine = ref<WebSocket | null>(null)
const wsAll = ref<WebSocket | null>(null)
let wsMineRetry = 0
let wsAllRetry = 0
let wsMineRetryTimer: number | undefined
let wsAllRetryTimer: number | undefined

const getWsUrl = (scope: 'mine' | 'all') => {
  const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const host =
    import.meta.env.DEV && import.meta.env.VITE_BACKEND_HOST
      ? String(import.meta.env.VITE_BACKEND_HOST)
      : import.meta.env.DEV
        ? '127.0.0.1:3000'
        : window.location.host
  const t = token.value
  if (!t) throw new ApiException({ code: 'UNAUTHORIZED', message: '未登录', status: 401 })
  const qs = new URLSearchParams({ scope, token: t })
  return `${protocol}://${host}/api/ws/orders?${qs.toString()}`
}

const connectOrdersWs = (scope: 'mine' | 'all') => {
  const refWs = scope === 'mine' ? wsMine : wsAll
  const retryTimerRef = scope === 'mine' ? () => wsMineRetryTimer : () => wsAllRetryTimer
  const setRetryTimer = (v: number | undefined) => {
    if (scope === 'mine') wsMineRetryTimer = v
    else wsAllRetryTimer = v
  }
  if (retryTimerRef()) {
    window.clearTimeout(retryTimerRef()!)
    setRetryTimer(undefined)
  }
  if (refWs.value && refWs.value.readyState <= 1) return

  const ws = new WebSocket(getWsUrl(scope))
  refWs.value = ws

  ws.addEventListener('open', () => {
    if (scope === 'mine') wsMineRetry = 0
    else wsAllRetry = 0
  })

  ws.addEventListener('message', async (evt) => {
    if (typeof evt.data !== 'string') return
    let json: unknown
    try {
      json = JSON.parse(evt.data)
    } catch {
      return
    }
    const msg = json as Partial<OrderWsEvent>
    if (msg.type === 'order.updated' && msg.data) {
      await patchOrderStatus(scope, msg.data)
    } else if (msg.type === 'order.snapshot' && msg.data && 'order' in msg.data && msg.data.order) {
      const incoming = msg.data.order
      const existing = (scope === 'mine' ? ordersMine.value : ordersAll.value).find((o) => o.id === incoming.id)
      if (existing && existing.updatedAt >= incoming.updatedAt) return
      upsertOrder(scope, incoming)
    }
  })

  ws.addEventListener('close', (evt) => {
    if (refWs.value === ws) refWs.value = null
    if (!token.value) return
    if (evt.code === 1008 || evt.code === 4401) {
      logout()
      openErrorDialog('连接已断开', new ApiException({ code: 'UNAUTHORIZED', message: '登录状态已失效，请重新登录', status: 401 }), '登录状态已失效')
      return
    }
    const retry = scope === 'mine' ? (wsMineRetry += 1) : (wsAllRetry += 1)
    const delay = Math.min(30000, 800 * 2 ** Math.min(6, retry))
    setRetryTimer(window.setTimeout(() => connectOrdersWs(scope), delay))
  })

  ws.addEventListener('error', () => {
    try {
      ws.close()
    } catch {
      return
    }
  })
}

const disconnectOrdersWs = (scope: 'mine' | 'all') => {
  const refWs = scope === 'mine' ? wsMine : wsAll
  const timer = scope === 'mine' ? wsMineRetryTimer : wsAllRetryTimer
  if (timer) window.clearTimeout(timer)
  if (scope === 'mine') wsMineRetryTimer = undefined
  else wsAllRetryTimer = undefined
  wsMineRetry = 0
  wsAllRetry = 0
  if (refWs.value) {
    try {
      refWs.value.close()
    } catch {
      return
    } finally {
      refWs.value = null
    }
  }
}

const placeOrder = async (input?: { note?: string }) => {
  if (orderItems.value.length === 0) {
    showToast('还没点菜哦～')
    return
  }
  const payload = {
    items: orderItems.value.map((x) => ({ dishId: x.dish.id, qty: x.qty })),
    note: input?.note?.trim() || undefined,
  }
  const data = await apiFetch<{ order: Order; user?: User; me?: User }>({ method: 'POST', path: '/orders', body: payload })
  showToast(`下单成功：${data.order.id}`)
  cart.value = {}
  ordersMine.value = [data.order, ...ordersMine.value].slice(0, 20)
  const u = data.user ?? data.me
  if (u) auth.loveMilli = u.loveMilli
  else await refreshMe()
}

const acceptOrder = async (orderId: string) => {
  await apiFetch<{ order: { id: string; status: OrderStatus; updatedAt: number } }>({
    method: 'POST',
    path: `/orders/${encodeURIComponent(orderId)}/accept`,
  })
  showToast(`已接单：${orderId}`)
}

const cancelOrder = async (orderId: string) => {
  const data = await apiFetch<{ order: Order; me?: User; user?: User }>({
    method: 'POST',
    path: `/orders/${encodeURIComponent(orderId)}/cancel`,
  })
  upsertOrder('mine', data.order)
  const u = data.me ?? data.user
  if (u) auth.loveMilli = u.loveMilli
  else await refreshMe()
  showToast(`已取消：${orderId}`)
}

const finishOrder = async (input: { orderId: string; images?: File[]; note?: string }) => {
  const { orderId, images, note } = input
  const path = `/orders/${encodeURIComponent(orderId)}/finish`

  const hasImages = Boolean(images && images.length)
  const data = await apiFetch<{ order: Order; me?: User; user?: User }>({
    method: 'POST',
    path,
    body: hasImages
      ? (() => {
          const fd = new FormData()
          ;(images ?? []).forEach((f) => fd.append('images', f))
          if (note) fd.append('note', note)
          return fd
        })()
      : undefined,
  })

  upsertOrder('all', data.order)
  upsertOrder('mine', data.order)
  const u = data.me ?? data.user
  if (u) auth.loveMilli = u.loveMilli
  else await refreshMe()
  showToast(`已完成：${orderId}`)
}

const submitOrderReview = async (input: { orderId: string; rating: number; content: string; images?: File[] }) => {
  const fd = new FormData()
  fd.append('rating', String(input.rating))
  fd.append('content', input.content)
  ;(input.images ?? []).forEach((f) => fd.append('images', f))
  const data = await apiFetch<{ order: Order }>({
    method: 'POST',
    path: `/orders/${encodeURIComponent(input.orderId)}/review`,
    body: fd,
  })
  upsertOrder('mine', data.order)
  upsertOrder('all', data.order)
  showToast('已提交评价')
}

const placedOrders = computed(() => ordersAll.value.filter((o) => o.status === 'placed').sort((a, b) => b.createdAt - a.createdAt))
const acceptedOrders = computed(() => ordersAll.value.filter((o) => o.status === 'accepted').sort((a, b) => b.createdAt - a.createdAt))

const formatOrderTime = (ts: number) => {
  const d = new Date(ts)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}

export const useAppStore = () => ({
  token,
  auth,
  categories,
  dishes,
  dishesMine,
  dishesTotal,
  dishesLoading,
  dishDetailById,
  cart,
  ordersMine,
  ordersAll,
  placedOrders,
  acceptedOrders,
  usersLoveRank,
  mode,
  toastText,
  toastShow,
  showToast,
  dialogShow,
  dialogTitle,
  dialogMessage,
  dialogTone,
  openDialog,
  closeDialog,
  openErrorDialog,
  loveMilliFromCent,
  formatLoveMilli,
  formatMoneyCent,
  dishLevelText,
  formatOrderTime,
  createDishImage,
  getDishById,
  orderItems,
  cartTotalCent,
  addToCart,
  updateCartQty,
  clearCart,
  bootstrap,
  refreshMe,
  loginWithPassword,
  register,
  logout,
  fetchDishes,
  fetchDishDetail,
  createDish,
  fetchMyDishes,
  fetchUsersLoveRank,
  deleteDish,
  fetchOrders,
  fetchAllOrders,
  fetchOrderDetail,
  placeOrder,
  acceptOrder,
  cancelOrder,
  finishOrder,
  submitOrderReview,
  connectOrdersWs,
  disconnectOrdersWs,
  ApiException,
})
