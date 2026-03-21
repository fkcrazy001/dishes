import Fastify, { type FastifyReply, type FastifyRequest } from 'fastify'
import cors from '@fastify/cors'
import jwt from '@fastify/jwt'
import multipart from '@fastify/multipart'
import fastifyStatic from '@fastify/static'
import { EventEmitter } from 'node:events'
import { createWriteStream } from 'node:fs'
import { mkdir } from 'node:fs/promises'
import path from 'node:path'
import { pipeline } from 'node:stream/promises'
import { randomBytes } from 'node:crypto'
import { WebSocketServer } from 'ws'
import { createStore, type OrderStatus, type Store } from './store.js'
import type { DishCategory, DishLevel } from './seed.js'

type ApiErrorCode =
  | 'ACCOUNT_EXISTS'
  | 'INVALID_PASSWORD'
  | 'INVALID_CREDENTIALS'
  | 'UNAUTHORIZED'
  | 'VALIDATION_ERROR'
  | 'DISH_NOT_FOUND'
  | 'INVALID_QTY'
  | 'INSUFFICIENT_LOVE'
  | 'ORDER_NOT_FOUND'
  | 'ORDER_INVALID_STATUS'
  | 'ORDER_ALREADY_REVIEWED'
  | 'INTERNAL_ERROR'

type ApiError = {
  success: false
  error: {
    code: ApiErrorCode | string
    message: string
    details?: unknown
  }
}

type ApiSuccess<T> = { success: true; data: T }

const ok = <T>(reply: FastifyReply, data: T) => reply.send({ success: true, data } satisfies ApiSuccess<T>)

const fail = (reply: FastifyReply, code: ApiErrorCode | string, message: string, details?: unknown, statusCode = 400) =>
  reply.status(statusCode).send({
    success: false,
    error: { code, message, details }
  } satisfies ApiError)

type JwtPayload = { userId: string }

declare module '@fastify/jwt' {
  interface FastifyJWT {
    payload: JwtPayload
    user: JwtPayload
  }
}

type OrderUpdatedEvent = {
  type: 'order.updated'
  data: {
    orderId: string
    status: OrderStatus
    updatedAt: number
  }
}

const eventBus = new EventEmitter()

const parseIntParam = (value: unknown, fallback: number) => {
  const n = typeof value === 'string' ? Number.parseInt(value, 10) : typeof value === 'number' ? value : NaN
  return Number.isFinite(n) && n > 0 ? n : fallback
}

const pickDishForList = (dish: {
  id: string
  name: string
  category: DishCategory
  timeText: string
  level: DishLevel
  tags: string[]
  priceCent: number
  story: string
  imageUrl: string
  badge: string
}) => dish

const requireAuth = async (req: FastifyRequest, reply: FastifyReply) => {
  try {
    await req.jwtVerify()
  } catch {
    return fail(reply, 'UNAUTHORIZED', '请先登录', undefined, 401)
  }
}

const ensureUser = (store: Store, userId: string) => {
  const u = store.getUserById(userId)
  if (!u) {
    const err = new Error('UNAUTHORIZED')
    ;(err as any).code = 'UNAUTHORIZED'
    throw err
  }
  return u
}

const asString = (value: unknown) => (typeof value === 'string' ? value : typeof value === 'number' ? String(value) : undefined)

const start = async () => {
  const store = await createStore()

  const app = Fastify({ logger: true })
  await app.register(cors, { origin: true })
  await app.register(jwt, { secret: process.env.JWT_SECRET ?? 'dev-secret' })
  await app.register(multipart, {
    limits: { files: 6, fileSize: 8 * 1024 * 1024 }
  })

  const uploadRoot = path.join(process.cwd(), 'data', 'uploads')
  await mkdir(uploadRoot, { recursive: true })
  await app.register(fastifyStatic, { root: uploadRoot, prefix: '/uploads/', decorateReply: false })

  const saveMultipartImages = async (req: FastifyRequest, input: { kind: 'order' | 'review'; orderId: string }) => {
    const images: string[] = []
    let note: string | undefined
    let rating: number | undefined
    let content: string | undefined

    const anyReq = req as any
    if (!anyReq.isMultipart?.()) return { images, note, rating, content }
    const dir = path.join(uploadRoot, input.kind, input.orderId)
    await mkdir(dir, { recursive: true })

    for await (const part of anyReq.parts()) {
      if (part.type === 'file') {
        if (part.fieldname !== 'images') continue
        if (typeof part.mimetype !== 'string' || !part.mimetype.startsWith('image/')) continue
        if (images.length >= 3) continue

        const ext = typeof part.filename === 'string' && path.extname(part.filename) ? path.extname(part.filename) : '.jpg'
        const fileName = `${Date.now()}-${randomBytes(6).toString('hex')}${ext}`
        const abs = path.join(dir, fileName)
        await pipeline(part.file, createWriteStream(abs))
        images.push(`/uploads/${input.kind}/${input.orderId}/${fileName}`)
        continue
      }

      if (part.type === 'field') {
        if (part.fieldname === 'note') note = String(part.value)
        if (part.fieldname === 'content') content = String(part.value)
        if (part.fieldname === 'rating') {
          const n = Number.parseInt(String(part.value), 10)
          if (Number.isFinite(n)) rating = n
        }
      }
    }

    return { images, note, rating, content }
  }

  type WsScope = 'mine' | 'all'
  type WsClient = { ws: import('ws').WebSocket; userId: string; scope: WsScope }
  const wsClients = new Set<WsClient>()
  const wss = new WebSocketServer({ noServer: true })

  const closeHttp = (socket: import('node:stream').Duplex, status: number, message: string) => {
    socket.write(`HTTP/1.1 ${status} ${message}\r\nConnection: close\r\n\r\n`)
    socket.destroy()
  }

  const safeSend = (ws: import('ws').WebSocket, payload: unknown) => {
    if (ws.readyState !== ws.OPEN) return
    ws.send(typeof payload === 'string' ? payload : JSON.stringify(payload))
  }

  app.server.on('upgrade', (req, socket, head) => {
    const host = req.headers.host ?? 'localhost'
    const url = new URL(req.url ?? '/', `http://${host}`)
    if (url.pathname !== '/api/ws/orders') return

    const tokenRaw = url.searchParams.get('token')?.trim()
    const token = tokenRaw?.replace(/^Bearer\s+/i, '').trim()
    if (!token) return closeHttp(socket, 401, 'Unauthorized')

    const scopeParam = url.searchParams.get('scope')
    const scope: WsScope = scopeParam === 'all' ? 'all' : 'mine'

    void (async () => {
      let userId: string
      try {
        const payload = (await app.jwt.verify(token)) as JwtPayload
        if (!payload?.userId) return closeHttp(socket, 401, 'Unauthorized')
        userId = payload.userId
      } catch {
        return closeHttp(socket, 401, 'Unauthorized')
      }

      wss.handleUpgrade(req, socket, head, (ws) => {
        const client: WsClient = { ws, userId, scope }
        wsClients.add(client)
        ws.on('close', () => wsClients.delete(client))
        ws.on('error', () => wsClients.delete(client))

        const { items } = store.listOrders({ userId, scope, page: 1, pageSize: 50 })
        for (const order of items) {
          safeSend(ws, { type: 'order.snapshot', data: { order } })
        }
      })
    })()
  })

  const onOrderUpdatedForWs = (evt: OrderUpdatedEvent) => {
    const order = store.getOrderById(evt.data.orderId)
    const ownerUserId = order?.placedBy.userId
    for (const c of wsClients) {
      if (c.scope === 'all') {
        safeSend(c.ws, evt)
        if (order) safeSend(c.ws, { type: 'order.snapshot', data: { order } })
        continue
      }
      if (ownerUserId && ownerUserId === c.userId) safeSend(c.ws, evt)
      if (order && ownerUserId && ownerUserId === c.userId) safeSend(c.ws, { type: 'order.snapshot', data: { order } })
    }
  }
  eventBus.on('order.updated', onOrderUpdatedForWs)

  app.setErrorHandler((err, _req, reply) => {
    const anyErr = err as any
    const code = typeof anyErr?.code === 'string' ? anyErr.code : 'INTERNAL_ERROR'
    const details = anyErr?.details
    if (code !== 'INTERNAL_ERROR') {
      const messageByCode: Record<string, string> = {
        ACCOUNT_EXISTS: '账号已存在',
        INVALID_PASSWORD: '密码至少 6 位',
        INVALID_CREDENTIALS: '账号或密码错误',
        UNAUTHORIZED: '请先登录',
        VALIDATION_ERROR: '参数错误',
        DISH_NOT_FOUND: '菜谱不存在',
        INVALID_QTY: '数量不合法',
        INSUFFICIENT_LOVE: '爱心值不足',
        ORDER_NOT_FOUND: '订单不存在',
        ORDER_INVALID_STATUS: '订单状态不允许该操作',
        ORDER_ALREADY_REVIEWED: '订单已评价'
      }
      return fail(reply, code, messageByCode[code] ?? code, details, 400)
    }
    reply.log.error(err)
    return fail(reply, 'INTERNAL_ERROR', '服务内部错误', undefined, 500)
  })

  app.register(
    async (api) => {
      api.post('/auth/register', async (req, reply) => {
        const body = req.body as any
        const account = asString(body?.account)
        const name = asString(body?.name)
        const password = asString(body?.password)
        const passwordHash = asString(body?.passwordHash)
        if (!body || !account || !name || (!password && !passwordHash)) {
          return fail(reply, 'VALIDATION_ERROR', '参数错误')
        }
        const result = store.register({
          account,
          name,
          password,
          passwordHash
        })
        await store.persist()
        return ok(reply, { user: result.user })
      })

      api.post('/auth/login', async (req, reply) => {
        const body = req.body as any
        const account = asString(body?.account)
        const password = asString(body?.password)
        const passwordHash = asString(body?.passwordHash)
        if (!body || !account || (!password && !passwordHash)) {
          return fail(reply, 'VALIDATION_ERROR', '参数错误')
        }
        const result = store.login({
          account,
          password,
          passwordHash
        })
        const accessToken = api.jwt.sign({ userId: result.user.id }, { expiresIn: '7d' })
        return ok(reply, { accessToken, user: result.user })
      })

      api.get('/me', { preHandler: requireAuth }, async (req, reply) => {
        const userId = (req.user as JwtPayload).userId
        const u = ensureUser(store, userId)
        return ok(reply, { user: u })
      })

      api.get('/users', async (req, reply) => {
        const sort = asString((req.query as any)?.sort) as 'loveMilli_desc' | undefined
        const limit = Math.min(200, parseIntParam((req.query as any)?.limit, 50))
        const { items } = store.listUsers({ sort, limit })
        return ok(reply, { items })
      })

      api.get('/dishes', async (req, reply) => {
        const q = (req.query as any)?.q
        const category = (req.query as any)?.category as DishCategory | undefined
        const scope = (req.query as any)?.scope as 'mine' | undefined
        const page = parseIntParam((req.query as any)?.page, 1)
        const pageSize = parseIntParam((req.query as any)?.pageSize, 20)

        let createdByUserId: string | undefined
        if (scope === 'mine') {
          const authResult = await requireAuth(req, reply)
          if (authResult) return authResult
          const userId = (req.user as JwtPayload).userId
          ensureUser(store, userId)
          createdByUserId = userId
        }

        const { items, total } = store.listDishes({ category, q, page, pageSize, createdByUserId })
        return ok(reply, { items: items.map(pickDishForList), page, pageSize, total })
      })

      api.get('/dishes/:dishId', async (req, reply) => {
        const dishId = (req.params as any)?.dishId
        if (!dishId || typeof dishId !== 'string') return fail(reply, 'VALIDATION_ERROR', '参数错误')
        const dish = store.getDishById(dishId)
        if (!dish) return fail(reply, 'DISH_NOT_FOUND', '菜谱不存在', { dishId }, 404)
        return ok(reply, { dish })
      })

      api.post('/dishes', { preHandler: requireAuth }, async (req, reply) => {
        const body = req.body as any
        const userId = (req.user as JwtPayload).userId
        const me = ensureUser(store, userId)
        if (
          !body ||
          typeof body.name !== 'string' ||
          typeof body.category !== 'string' ||
          typeof body.timeText !== 'string' ||
          typeof body.level !== 'string' ||
          !Array.isArray(body.tags) ||
          typeof body.priceCent !== 'number' ||
          typeof body.story !== 'string' ||
          typeof body.imageUrl !== 'string' ||
          typeof body.badge !== 'string' ||
          typeof body.details !== 'object' ||
          body.details === null ||
          !Array.isArray(body.details.ingredients) ||
          !Array.isArray(body.details.steps)
        ) {
          return fail(reply, 'VALIDATION_ERROR', '参数错误')
        }

        const dish = store.createDish({
          name: body.name,
          category: body.category,
          timeText: body.timeText,
          level: body.level,
          tags: body.tags,
          priceCent: body.priceCent,
          story: body.story,
          imageUrl: body.imageUrl,
          badge: body.badge,
          details: { ingredients: body.details.ingredients, steps: body.details.steps },
          createdBy: { userId: me.id, name: me.name }
        })
        await store.persist()
        return ok(reply, { dish })
      })

      api.delete('/dishes/:dishId', { preHandler: requireAuth }, async (req, reply) => {
        const userId = (req.user as JwtPayload).userId
        ensureUser(store, userId)

        const dishId = (req.params as any)?.dishId
        if (!dishId || typeof dishId !== 'string') return fail(reply, 'VALIDATION_ERROR', '参数错误')

        const dish = store.getDishById(dishId)
        if (!dish) return fail(reply, 'DISH_NOT_FOUND', '菜谱不存在', { dishId }, 404)
        if (dish.createdBy?.userId !== userId) return fail(reply, 'UNAUTHORIZED', '无权限删除该菜谱', undefined, 403)

        const okDeleted = store.deleteDish({ dishId })
        await store.persist()
        if (!okDeleted) return fail(reply, 'DISH_NOT_FOUND', '菜谱不存在', { dishId }, 404)
        return ok(reply, { dish: { id: dishId } })
      })

      api.post('/orders', { preHandler: requireAuth }, async (req, reply) => {
        const body = req.body as any
        const userId = (req.user as JwtPayload).userId
        const u = ensureUser(store, userId)
        const items = body?.items
        if (!Array.isArray(items) || items.length === 0) return fail(reply, 'VALIDATION_ERROR', '参数错误')
        const note = typeof body?.note === 'string' ? body.note : undefined
        const order = store.createOrder({
          userId,
          userName: u.name,
          items: items.map((it: any) => ({ dishId: it?.dishId, qty: it?.qty })),
          note
        })
        await store.persist()
        const me = ensureUser(store, userId)
        return ok(reply, {
          order,
          me
        })
      })

      api.get('/orders', { preHandler: requireAuth }, async (req, reply) => {
        const userId = (req.user as JwtPayload).userId
        ensureUser(store, userId)

        const scope = ((req.query as any)?.scope as 'mine' | 'all' | undefined) ?? 'mine'
        const status = (req.query as any)?.status as OrderStatus | undefined
        const page = parseIntParam((req.query as any)?.page, 1)
        const pageSize = parseIntParam((req.query as any)?.pageSize, 20)

        const { items, total } = store.listOrders({ userId, scope, status, page, pageSize })
        return ok(reply, {
          items,
          page,
          pageSize,
          total
        })
      })

      api.get('/orders/:orderId', { preHandler: requireAuth }, async (req, reply) => {
        const userId = (req.user as JwtPayload).userId
        ensureUser(store, userId)

        const orderId = (req.params as any)?.orderId
        if (!orderId || typeof orderId !== 'string') return fail(reply, 'VALIDATION_ERROR', '参数错误')
        const order = store.getOrderById(orderId)
        if (!order) return fail(reply, 'ORDER_NOT_FOUND', '订单不存在', { orderId }, 404)
        const canRead = order.placedBy.userId === userId || order.acceptedBy?.userId === userId
        if (!canRead) return fail(reply, 'UNAUTHORIZED', '无权限查看该订单', undefined, 403)
        return ok(reply, { order })
      })

      api.post('/orders/:orderId/accept', { preHandler: requireAuth }, async (req, reply) => {
        const userId = (req.user as JwtPayload).userId
        const me = ensureUser(store, userId)

        const orderId = (req.params as any)?.orderId
        if (!orderId || typeof orderId !== 'string') return fail(reply, 'VALIDATION_ERROR', '参数错误')
        const order = store.acceptOrder({ orderId, acceptedBy: { userId, name: me.name } })
        await store.persist()
        const evt: OrderUpdatedEvent = { type: 'order.updated', data: { orderId: order.id, status: order.status, updatedAt: order.updatedAt } }
        eventBus.emit('order.updated', evt)
        return ok(reply, { order })
      })

      api.post('/orders/:orderId/cancel', { preHandler: requireAuth }, async (req, reply) => {
        const userId = (req.user as JwtPayload).userId
        ensureUser(store, userId)

        const orderId = (req.params as any)?.orderId
        if (!orderId || typeof orderId !== 'string') return fail(reply, 'VALIDATION_ERROR', '参数错误')
        const result = store.cancelOrder({ orderId, userId })
        await store.persist()
        const evt: OrderUpdatedEvent = {
          type: 'order.updated',
          data: { orderId: result.order.id, status: result.order.status, updatedAt: result.order.updatedAt }
        }
        eventBus.emit('order.updated', evt)
        return ok(reply, { order: result.order, me: result.me })
      })

      api.post('/orders/:orderId/finish', { preHandler: requireAuth }, async (req, reply) => {
        const userId = (req.user as JwtPayload).userId
        ensureUser(store, userId)

        const orderId = (req.params as any)?.orderId
        if (!orderId || typeof orderId !== 'string') return fail(reply, 'VALIDATION_ERROR', '参数错误')
        const anyReq = req as any
        const { images, note } = await saveMultipartImages(anyReq, { kind: 'order', orderId })
        const result = store.finishOrder({ orderId, finishedByUserId: userId, finishImages: images, note })
        await store.persist()
        const evt: OrderUpdatedEvent = {
          type: 'order.updated',
          data: { orderId: result.order.id, status: result.order.status, updatedAt: result.order.updatedAt }
        }
        eventBus.emit('order.updated', evt)
        return ok(reply, { order: result.order, me: result.me })
      })

      api.post('/orders/:orderId/review', { preHandler: requireAuth }, async (req, reply) => {
        const userId = (req.user as JwtPayload).userId
        ensureUser(store, userId)

        const orderId = (req.params as any)?.orderId
        if (!orderId || typeof orderId !== 'string') return fail(reply, 'VALIDATION_ERROR', '参数错误')

        const anyReq = req as any
        const { images, rating, content } = await saveMultipartImages(anyReq, { kind: 'review', orderId })
        if (!rating || rating < 1 || rating > 5 || !content) return fail(reply, 'VALIDATION_ERROR', '参数错误')
        const order = store.createReview({ orderId, userId, rating, content, images })
        await store.persist()
        const evt: OrderUpdatedEvent = { type: 'order.updated', data: { orderId: order.id, status: order.status, updatedAt: order.updatedAt } }
        eventBus.emit('order.updated', evt)
        return ok(reply, { order })
      })

      api.get('/orders/stream', { preHandler: requireAuth }, async (req, reply) => {
        const userId = (req.user as JwtPayload).userId
        ensureUser(store, userId)

        const scope = ((req.query as any)?.scope as 'mine' | 'all' | undefined) ?? 'mine'
        reply.raw.writeHead(200, {
          'Content-Type': 'text/event-stream; charset=utf-8',
          'Cache-Control': 'no-cache, no-transform',
          Connection: 'keep-alive'
        })

        const writeEvent = (evt: OrderUpdatedEvent) => {
          if (scope === 'mine') {
            const order = store.getOrderById(evt.data.orderId)
            if (!order || order.placedBy.userId !== userId) return
          }
          reply.raw.write(`data: ${JSON.stringify(evt)}\n\n`)
        }

        const onUpdate = (evt: OrderUpdatedEvent) => writeEvent(evt)
        eventBus.on('order.updated', onUpdate)

        const ping = setInterval(() => {
          reply.raw.write(`event: ping\ndata: ${Date.now()}\n\n`)
        }, 15000)

        req.raw.on('close', () => {
          clearInterval(ping)
          eventBus.off('order.updated', onUpdate)
        })
      })
    },
    { prefix: '/api' }
  )

  const webRoot = path.resolve(process.cwd(), '..', 'dishes-front', 'dist')
  await app.register(fastifyStatic, { root: webRoot, prefix: '/', index: ['index.html'] })
  app.setNotFoundHandler((req, reply) => {
    if (req.url.startsWith('/api') || req.url.startsWith('/uploads')) {
      reply.code(404).send({
        success: false,
        error: { code: 'NOT_FOUND', message: '接口不存在' }
      })
      return
    }
    reply.type('text/html; charset=utf-8').sendFile('index.html')
  })

  const port = Number.parseInt(process.env.PORT ?? '3000', 10)
  const host = process.env.HOST ?? '0.0.0.0'
  await app.listen({ port, host })
}

start().catch((err) => {
  process.stderr.write(`${String(err)}\n`)
  process.exit(1)
})
