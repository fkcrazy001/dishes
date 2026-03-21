import { createHash, randomBytes, scryptSync, timingSafeEqual } from 'node:crypto'
import path from 'node:path'
import { mkdir } from 'node:fs/promises'
import { DatabaseSync } from 'node:sqlite'
import { seedDishes, type Dish, type DishCategory } from './seed.js'

export type OrderStatus = 'placed' | 'accepted' | 'done' | 'cancelled'

export type User = {
  id: string
  account: string
  name: string
  password: {
    salt: string
    hash: string
  }
  createdAt: number
  loveMilli: number
}

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
  placedBy: { userId: string; name: string }
  placedNote?: string
  acceptedBy?: { userId: string; name: string }
  finishedAt?: number
  finishImages?: string[]
  finishNote?: string
  review?: { rating: number; content: string; images: string[]; createdAt: number }
  items: OrderItem[]
  totalCent: number
}

export type DbShape = {
  users: User[]
  dishes: Dish[]
  orders: Order[]
}

const createUserId = () => `u_${randomBytes(6).toString('hex')}`

const createOrderId = () => {
  const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789'
  const bytes = randomBytes(8)
  let out = ''
  for (let i = 0; i < 8; i++) out += alphabet[bytes[i] % alphabet.length]
  return out
}

const hashPassword = (password: string, saltHex?: string) => {
  const salt = saltHex ? Buffer.from(saltHex, 'hex') : randomBytes(16)
  const key = scryptSync(password, salt, 32)
  return { saltHex: salt.toString('hex'), hashHex: key.toString('hex') }
}

const verifyPassword = (password: string, saltHex: string, hashHex: string) => {
  const { hashHex: next } = hashPassword(password, saltHex)
  return timingSafeEqual(Buffer.from(next, 'hex'), Buffer.from(hashHex, 'hex'))
}

const sha256HexLower = (value: string) => createHash('sha256').update(value).digest('hex')

const normalizeSha256Hex = (raw: string) => {
  const v = raw.trim().toLowerCase()
  if (v.length !== 64) return
  if (!/^[0-9a-f]{64}$/.test(v)) return
  return v
}

const defaultDb = (): DbShape => ({
  users: [],
  dishes: seedDishes(),
  orders: []
})

export type Store = {
  getDb: () => DbShape
  persist: () => Promise<void>
  register: (input: { account: string; name: string; password?: string; passwordHash?: string }) => {
    user: Pick<User, 'id' | 'account' | 'name' | 'loveMilli'>
  }
  login: (input: { account: string; password?: string; passwordHash?: string }) => { user: Pick<User, 'id' | 'account' | 'name' | 'loveMilli'> }
  getUserById: (id: string) => Pick<User, 'id' | 'account' | 'name' | 'loveMilli'> | undefined
  listUsers: (input: { sort?: 'loveMilli_desc'; limit: number }) => { items: Array<Pick<User, 'id' | 'name' | 'loveMilli'>> }
  listDishes: (input: { category?: DishCategory; q?: string; page: number; pageSize: number; createdByUserId?: string }) => {
    items: Dish[]
    total: number
  }
  getDishById: (dishId: string) => Dish | undefined
  createDish: (dish: Omit<Dish, 'id'> & { id?: string }) => Dish
  deleteDish: (input: { dishId: string }) => boolean
  createOrder: (input: { userId: string; userName: string; items: Array<{ dishId: string; qty: number }>; note?: string }) => Order
  listOrders: (input: { userId: string; scope: 'mine' | 'all'; status?: OrderStatus; page: number; pageSize: number }) => { items: Order[]; total: number }
  getOrderById: (orderId: string) => Order | undefined
  cancelOrder: (input: { orderId: string; userId: string }) => { order: Order; me: Pick<User, 'id' | 'account' | 'name' | 'loveMilli'> }
  acceptOrder: (input: { orderId: string; acceptedBy: { userId: string; name: string } }) => Order
  finishOrder: (input: { orderId: string; finishedByUserId: string; finishImages: string[]; note?: string }) => {
    order: Order
    me: Pick<User, 'id' | 'account' | 'name' | 'loveMilli'>
  }
  createReview: (input: { orderId: string; userId: string; rating: number; content: string; images: string[] }) => Order
}

export const createStore = async (input?: { dbFile?: string }): Promise<Store> => {
  const dbFile = input?.dbFile ?? process.env.DB_FILE ?? process.env.DATA_FILE ?? './data/db.sqlite'
  const absDbFile = path.isAbsolute(dbFile) ? dbFile : path.join(process.cwd(), dbFile)

  await mkdir(path.dirname(absDbFile), { recursive: true })
  const db = new DatabaseSync(absDbFile, { open: true })
  db.exec(`
    PRAGMA foreign_keys = ON;
    PRAGMA journal_mode = WAL;

    CREATE TABLE IF NOT EXISTS users (
      id TEXT PRIMARY KEY,
      account TEXT NOT NULL UNIQUE,
      name TEXT NOT NULL,
      password_salt TEXT NOT NULL,
      password_hash TEXT NOT NULL,
      password_kind TEXT NOT NULL DEFAULT 'plain',
      created_at INTEGER NOT NULL,
      love_milli INTEGER NOT NULL DEFAULT 100000
    );

    CREATE TABLE IF NOT EXISTS dishes (
      id TEXT PRIMARY KEY,
      name TEXT NOT NULL,
      category TEXT NOT NULL,
      time_text TEXT NOT NULL,
      level TEXT NOT NULL,
      tags_json TEXT NOT NULL,
      price_cent INTEGER NOT NULL,
      story TEXT NOT NULL,
      image_url TEXT NOT NULL,
      badge TEXT NOT NULL,
      details_json TEXT NOT NULL,
      created_by_user_id TEXT,
      created_by_name TEXT,
      created_at INTEGER
    );

    CREATE TABLE IF NOT EXISTS orders (
      id TEXT PRIMARY KEY,
      created_at INTEGER NOT NULL,
      updated_at INTEGER NOT NULL,
      status TEXT NOT NULL,
      placed_by_user_id TEXT NOT NULL,
      placed_by_name TEXT NOT NULL,
      placed_note TEXT,
      accepted_by_user_id TEXT,
      accepted_by_name TEXT,
      finished_at INTEGER,
      finish_images_json TEXT,
      finish_note TEXT,
      total_cent INTEGER NOT NULL
    );

    CREATE TABLE IF NOT EXISTS order_items (
      id INTEGER PRIMARY KEY AUTOINCREMENT,
      order_id TEXT NOT NULL,
      dish_id TEXT NOT NULL,
      dish_name TEXT NOT NULL,
      qty INTEGER NOT NULL,
      price_cent INTEGER NOT NULL,
      FOREIGN KEY(order_id) REFERENCES orders(id) ON DELETE CASCADE
    );

    CREATE INDEX IF NOT EXISTS idx_dishes_category ON dishes(category);
    CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);
    CREATE INDEX IF NOT EXISTS idx_orders_placed_by_user_id ON orders(placed_by_user_id);
    CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
  `)

  const ensureUserPasswordKind = () => {
    const cols = db.prepare(`PRAGMA table_info(users)`).all() as Array<{ name: string }>
    const has = cols.some((c) => c.name === 'password_kind')
    if (has) return
    db.exec(`ALTER TABLE users ADD COLUMN password_kind TEXT NOT NULL DEFAULT 'plain'`)
  }

  const ensureUserLoveMilli = () => {
    const cols = db.prepare(`PRAGMA table_info(users)`).all() as Array<{ name: string }>
    const has = cols.some((c) => c.name === 'love_milli')
    if (!has) db.exec(`ALTER TABLE users ADD COLUMN love_milli INTEGER NOT NULL DEFAULT 100000`)
    db.prepare(`UPDATE users SET love_milli = 100000 WHERE love_milli IS NULL`).run()
  }

  const ensureOrderAcceptedBy = () => {
    const cols = db.prepare(`PRAGMA table_info(orders)`).all() as Array<{ name: string }>
    const hasUserId = cols.some((c) => c.name === 'accepted_by_user_id')
    if (!hasUserId) db.exec(`ALTER TABLE orders ADD COLUMN accepted_by_user_id TEXT`)
    const hasName = cols.some((c) => c.name === 'accepted_by_name')
    if (!hasName) db.exec(`ALTER TABLE orders ADD COLUMN accepted_by_name TEXT`)
  }

  const ensureOrderPlacedNote = () => {
    const cols = db.prepare(`PRAGMA table_info(orders)`).all() as Array<{ name: string }>
    const has = cols.some((c) => c.name === 'placed_note')
    if (!has) db.exec(`ALTER TABLE orders ADD COLUMN placed_note TEXT`)
  }

  const ensureDishCreatedBy = () => {
    const cols = db.prepare(`PRAGMA table_info(dishes)`).all() as Array<{ name: string }>
    const hasUserId = cols.some((c) => c.name === 'created_by_user_id')
    if (!hasUserId) db.exec(`ALTER TABLE dishes ADD COLUMN created_by_user_id TEXT`)
    const hasName = cols.some((c) => c.name === 'created_by_name')
    if (!hasName) db.exec(`ALTER TABLE dishes ADD COLUMN created_by_name TEXT`)
    const hasCreatedAt = cols.some((c) => c.name === 'created_at')
    if (!hasCreatedAt) db.exec(`ALTER TABLE dishes ADD COLUMN created_at INTEGER`)
  }

  ensureUserPasswordKind()
  ensureUserLoveMilli()
  ensureDishCreatedBy()
  ensureOrderAcceptedBy()
  ensureOrderPlacedNote()

  const ensureOrderFinishFields = () => {
    const cols = db.prepare(`PRAGMA table_info(orders)`).all() as Array<{ name: string }>
    const hasFinishedAt = cols.some((c) => c.name === 'finished_at')
    if (!hasFinishedAt) db.exec(`ALTER TABLE orders ADD COLUMN finished_at INTEGER`)
    const hasImages = cols.some((c) => c.name === 'finish_images_json')
    if (!hasImages) db.exec(`ALTER TABLE orders ADD COLUMN finish_images_json TEXT`)
    const hasNote = cols.some((c) => c.name === 'finish_note')
    if (!hasNote) db.exec(`ALTER TABLE orders ADD COLUMN finish_note TEXT`)
  }

  const ensureOrderReviewTable = () => {
    db.exec(`
      CREATE TABLE IF NOT EXISTS order_reviews (
        order_id TEXT PRIMARY KEY,
        rating INTEGER NOT NULL,
        content TEXT NOT NULL,
        images_json TEXT NOT NULL,
        created_at INTEGER NOT NULL,
        created_by_user_id TEXT NOT NULL
      );
      CREATE INDEX IF NOT EXISTS idx_order_reviews_created_at ON order_reviews(created_at);
    `)
  }

  ensureOrderFinishFields()
  ensureOrderReviewTable()

  const parseJson = <T>(raw: unknown, fallback: T): T => {
    if (typeof raw !== 'string') return fallback
    try {
      return JSON.parse(raw) as T
    } catch {
      return fallback
    }
  }

  const getReviewByOrderId = (orderId: string) => {
    const r = db
      .prepare(`SELECT rating, content, images_json, created_at FROM order_reviews WHERE order_id = ? LIMIT 1`)
      .get(orderId) as any
    if (!r) return
    return {
      rating: Number(r.rating),
      content: String(r.content),
      images: parseJson<string[]>(r.images_json, []),
      createdAt: Number(r.created_at)
    }
  }

  const seedIfNeeded = () => {
    const row = db.prepare(`SELECT COUNT(1) AS cnt FROM dishes`).get() as any
    const cnt = Number(row?.cnt ?? 0)
    if (cnt > 0) return
    const insert = db.prepare(`
      INSERT INTO dishes (
        id, name, category, time_text, level, tags_json, price_cent, story, image_url, badge, details_json, created_by_user_id, created_by_name, created_at
      ) VALUES (
        $id, $name, $category, $timeText, $level, $tagsJson, $priceCent, $story, $imageUrl, $badge, $detailsJson, $createdByUserId, $createdByName, $createdAt
      )
    `)
    const dishes = seedDishes()
    db.exec('BEGIN')
    try {
      for (const d of dishes) {
        insert.run({
          id: d.id,
          name: d.name,
          category: d.category,
          timeText: d.timeText,
          level: d.level,
          tagsJson: JSON.stringify(d.tags),
          priceCent: d.priceCent,
          story: d.story,
          imageUrl: d.imageUrl,
          badge: d.badge,
          detailsJson: JSON.stringify(d.details),
          createdByUserId: null,
          createdByName: null,
          createdAt: Date.now()
        })
      }
      db.exec('COMMIT')
    } catch (e) {
      db.exec('ROLLBACK')
      throw e
    }
  }

  seedIfNeeded()

  const persist = async () => {}

  const getUserById: Store['getUserById'] = (id) => {
    const row = db
      .prepare(`SELECT id, account, name, love_milli FROM users WHERE id = ?`)
      .get(id) as { id: string; account: string; name: string; love_milli: number } | undefined
    if (!row) return
    return { id: row.id, account: row.account, name: row.name, loveMilli: Number((row as any).love_milli ?? 0) }
  }

  const register: Store['register'] = ({ account, password, passwordHash, name }) => {
    const normalized = account.trim()
    const exists = db.prepare(`SELECT 1 FROM users WHERE account = ? LIMIT 1`).get(normalized)
    if (exists) {
      const err = new Error('ACCOUNT_EXISTS')
      ;(err as any).code = 'ACCOUNT_EXISTS'
      throw err
    }
    const normalizedHash = typeof passwordHash === 'string' ? normalizeSha256Hex(passwordHash) : undefined
    const passwordInput = typeof password === 'string' ? password : undefined
    if (!normalizedHash && !passwordInput) {
      const err = new Error('VALIDATION_ERROR')
      ;(err as any).code = 'VALIDATION_ERROR'
      ;(err as any).details = { field: 'passwordHash', reason: 'missing' }
      throw err
    }
    if (!normalizedHash && passwordInput && passwordInput.trim().length < 6) {
      const err = new Error('INVALID_PASSWORD')
      ;(err as any).code = 'INVALID_PASSWORD'
      throw err
    }

    const secret = normalizedHash ?? sha256HexLower(passwordInput!.trim())
    const { saltHex, hashHex } = hashPassword(secret)
    const user: User = {
      id: createUserId(),
      account: normalized,
      name: name.trim() || normalized,
      password: { salt: saltHex, hash: hashHex },
      createdAt: Date.now(),
      loveMilli: 100000
    }
    db.prepare(
      `INSERT INTO users (id, account, name, password_salt, password_hash, created_at, password_kind, love_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
    ).run(user.id, user.account, user.name, user.password.salt, user.password.hash, user.createdAt, 'sha256', user.loveMilli)
    return { user: { id: user.id, account: user.account, name: user.name, loveMilli: user.loveMilli } }
  }

  const login: Store['login'] = ({ account, password, passwordHash }) => {
    const normalized = account.trim()
    const row = db
      .prepare(
        `SELECT id, account, name, password_salt, password_hash, password_kind, love_milli FROM users WHERE account = ? LIMIT 1`
      )
      .get(normalized) as
      | { id: string; account: string; name: string; password_salt: string; password_hash: string; love_milli: number }
      | undefined
    if (!row) {
      const err = new Error('INVALID_CREDENTIALS')
      ;(err as any).code = 'INVALID_CREDENTIALS'
      throw err
    }

    const kind = (row as any).password_kind as 'plain' | 'sha256' | undefined
    const normalizedHash = typeof passwordHash === 'string' ? normalizeSha256Hex(passwordHash) : undefined
    const passwordInput = typeof password === 'string' ? password : undefined

    let ok = false
    if (kind === 'sha256') {
      if (normalizedHash) ok = verifyPassword(normalizedHash, (row as any).password_salt, (row as any).password_hash)
      else if (passwordInput) ok = verifyPassword(sha256HexLower(passwordInput), (row as any).password_salt, (row as any).password_hash)
    } else {
      if (passwordInput) ok = verifyPassword(passwordInput, (row as any).password_salt, (row as any).password_hash)
    }

    if (!ok) {
      const err = new Error('INVALID_CREDENTIALS')
      ;(err as any).code = 'INVALID_CREDENTIALS'
      throw err
    }

    if (kind !== 'sha256' && passwordInput) {
      const nextSecret = sha256HexLower(passwordInput)
      const { saltHex, hashHex } = hashPassword(nextSecret)
      db.prepare(`UPDATE users SET password_salt = ?, password_hash = ?, password_kind = ? WHERE id = ?`).run(
        saltHex,
        hashHex,
        'sha256',
        (row as any).id
      )
    }

    return { user: { id: row.id, account: row.account, name: row.name, loveMilli: Number((row as any).love_milli ?? 0) } }
  }

  const listUsers: Store['listUsers'] = ({ sort, limit }) => {
    if (sort && sort !== 'loveMilli_desc') {
      const err = new Error('VALIDATION_ERROR')
      ;(err as any).code = 'VALIDATION_ERROR'
      ;(err as any).details = { field: 'sort', allowed: ['loveMilli_desc'] }
      throw err
    }
    const safeLimit = Math.max(1, Math.min(200, Math.trunc(limit)))
    const rows = db
      .prepare(`SELECT id, name, love_milli FROM users ORDER BY love_milli DESC, created_at ASC LIMIT ?`)
      .all(safeLimit) as any[]
    return {
      items: rows.map((r) => ({ id: String(r.id), name: String(r.name), loveMilli: Number(r.love_milli ?? 0) }))
    }
  }

  const listDishes: Store['listDishes'] = ({ category, q, page, pageSize, createdByUserId }) => {
    const keyword = typeof q === 'string' ? q.trim() : undefined
    const like = keyword ? `%${keyword}%` : undefined

    const where: string[] = []
    const params: any[] = []
    if (createdByUserId) {
      where.push(`created_by_user_id = ?`)
      params.push(createdByUserId)
    }
    if (category) {
      where.push(`category = ?`)
      params.push(category)
    }
    if (like) {
      where.push(`(name LIKE ? OR tags_json LIKE ?)`)
      params.push(like, like)
    }

    const whereSql = where.length ? `WHERE ${where.join(' AND ')}` : ''
    const totalRow = db.prepare(`SELECT COUNT(1) AS cnt FROM dishes ${whereSql}`).get(...params) as any
    const total = Number(totalRow?.cnt ?? 0)

    const offset = (page - 1) * pageSize
    const rows = db
      .prepare(
        `
          SELECT id, name, category, time_text, level, tags_json, price_cent, story, image_url, badge, details_json, created_by_user_id, created_by_name
          FROM dishes
          ${whereSql}
          ORDER BY rowid ASC
          LIMIT ? OFFSET ?
        `
      )
      .all(...params, pageSize, offset) as any[]

    const items: Dish[] = rows.map((r) => ({
      id: String(r.id),
      name: String(r.name),
      category: r.category as DishCategory,
      timeText: String(r.time_text),
      level: r.level as any,
      tags: parseJson<string[]>(r.tags_json, []),
      priceCent: Number(r.price_cent),
      story: String(r.story),
      imageUrl: String(r.image_url),
      badge: String(r.badge),
      details: parseJson<any>(r.details_json, { ingredients: [], steps: [] }),
      createdBy:
        typeof r.created_by_user_id === 'string' && typeof r.created_by_name === 'string'
          ? { userId: String(r.created_by_user_id), name: String(r.created_by_name) }
          : undefined
    }))

    return { items, total }
  }

  const getDishById: Store['getDishById'] = (dishId) => {
    const r = db
      .prepare(
        `SELECT id, name, category, time_text, level, tags_json, price_cent, story, image_url, badge, details_json, created_by_user_id, created_by_name FROM dishes WHERE id = ? LIMIT 1`
      )
      .get(dishId) as any
    if (!r) return
    return {
      id: String(r.id),
      name: String(r.name),
      category: r.category as DishCategory,
      timeText: String(r.time_text),
      level: r.level as any,
      tags: parseJson<string[]>(r.tags_json, []),
      priceCent: Number(r.price_cent),
      story: String(r.story),
      imageUrl: String(r.image_url),
      badge: String(r.badge),
      details: parseJson<any>(r.details_json, { ingredients: [], steps: [] }),
      createdBy:
        typeof r.created_by_user_id === 'string' && typeof r.created_by_name === 'string'
          ? { userId: String(r.created_by_user_id), name: String(r.created_by_name) }
          : undefined
    }
  }

  const normalizeDishId = (raw: string) => {
    const s = raw
      .trim()
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
      .replace(/^-+|-+$/g, '')
    return s
  }

  const createDish: Store['createDish'] = (dish) => {
    const base = typeof dish.id === 'string' ? normalizeDishId(dish.id) : normalizeDishId(dish.name)
    let id = base || `dish-${randomBytes(6).toString('hex')}`
    const exists = db.prepare(`SELECT 1 FROM dishes WHERE id = ? LIMIT 1`)
    if (exists.get(id)) id = `${id}-${randomBytes(3).toString('hex')}`

    const next: Dish = {
      id,
      name: dish.name,
      category: dish.category,
      timeText: dish.timeText,
      level: dish.level,
      tags: dish.tags,
      priceCent: dish.priceCent,
      story: dish.story,
      imageUrl: dish.imageUrl,
      badge: dish.badge,
      details: dish.details,
      createdBy: dish.createdBy
    }

    const now = Date.now()
    db.prepare(
      `
        INSERT INTO dishes (
          id, name, category, time_text, level, tags_json, price_cent, story, image_url, badge, details_json, created_by_user_id, created_by_name, created_at
        ) VALUES (
          ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
        )
      `
    ).run(
      next.id,
      next.name,
      next.category,
      next.timeText,
      next.level,
      JSON.stringify(next.tags),
      next.priceCent,
      next.story,
      next.imageUrl,
      next.badge,
      JSON.stringify(next.details),
      next.createdBy?.userId ?? null,
      next.createdBy?.name ?? null,
      now
    )

    return next
  }

  const deleteDish: Store['deleteDish'] = ({ dishId }) => {
    const res = db.prepare(`DELETE FROM dishes WHERE id = ?`).run(dishId)
    return Number((res as any)?.changes ?? 0) > 0
  }

  const createOrder: Store['createOrder'] = ({ userId, userName, items, note }) => {
    const now = Date.now()
    const orderId = createOrderId()

    db.exec('BEGIN')
    try {
      const lines: OrderItem[] = items.map((it) => {
        const dish = getDishById(it.dishId as any)
        if (!dish) {
          const err = new Error('DISH_NOT_FOUND')
          ;(err as any).code = 'DISH_NOT_FOUND'
          ;(err as any).details = { dishId: it.dishId }
          throw err
        }
        if (!Number.isInteger(it.qty) || it.qty <= 0) {
          const err = new Error('INVALID_QTY')
          ;(err as any).code = 'INVALID_QTY'
          ;(err as any).details = { dishId: it.dishId }
          throw err
        }
        return { dishId: dish.id, dishName: dish.name, qty: it.qty, priceCent: dish.priceCent }
      })

      const totalCent = lines.reduce((s, x) => s + x.priceCent * x.qty, 0)

      const me = getUserById(userId)
      if (!me) {
        const err = new Error('UNAUTHORIZED')
        ;(err as any).code = 'UNAUTHORIZED'
        throw err
      }
      if (me.loveMilli < totalCent) {
        const err = new Error('INSUFFICIENT_LOVE')
        ;(err as any).code = 'INSUFFICIENT_LOVE'
        ;(err as any).details = { loveMilli: me.loveMilli, required: totalCent }
        throw err
      }

      db.prepare(
        `INSERT INTO orders (id, created_at, updated_at, status, placed_by_user_id, placed_by_name, placed_note, total_cent) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
      ).run(orderId, now, now, 'placed', userId, userName, note ?? null, totalCent)

      const insertItem = db.prepare(
        `INSERT INTO order_items (order_id, dish_id, dish_name, qty, price_cent) VALUES (?, ?, ?, ?, ?)`
      )
      for (const line of lines) {
        insertItem.run(orderId, line.dishId, line.dishName, line.qty, line.priceCent)
      }

      db.prepare(`UPDATE users SET love_milli = love_milli - ? WHERE id = ?`).run(totalCent, userId)

      db.exec('COMMIT')

      return {
        id: orderId,
        createdAt: now,
        updatedAt: now,
        status: 'placed',
        placedBy: { userId, name: userName },
        placedNote: typeof note === 'string' ? note : undefined,
        items: lines,
        totalCent
      }
    } catch (e) {
      db.exec('ROLLBACK')
      throw e
    }
  }

  const getOrderById: Store['getOrderById'] = (orderId) => {
    const o = db
      .prepare(
        `SELECT id, created_at, updated_at, status, placed_by_user_id, placed_by_name, placed_note, accepted_by_user_id, accepted_by_name, finished_at, finish_images_json, finish_note, total_cent FROM orders WHERE id = ? LIMIT 1`
      )
      .get(orderId) as any
    if (!o) return

    const rows = db
      .prepare(`SELECT dish_id, dish_name, qty, price_cent FROM order_items WHERE order_id = ? ORDER BY id ASC`)
      .all(orderId) as any[]
    const items: OrderItem[] = rows.map((r) => ({
      dishId: String(r.dish_id),
      dishName: String(r.dish_name),
      qty: Number(r.qty),
      priceCent: Number(r.price_cent)
    }))

    const review = getReviewByOrderId(String(o.id))
    return {
      id: String(o.id),
      createdAt: Number(o.created_at),
      updatedAt: Number(o.updated_at),
      status: o.status as OrderStatus,
      placedBy: { userId: String(o.placed_by_user_id), name: String(o.placed_by_name) },
      placedNote: typeof o.placed_note === 'string' ? String(o.placed_note) : undefined,
      acceptedBy:
        typeof o.accepted_by_user_id === 'string' && typeof o.accepted_by_name === 'string'
          ? { userId: String(o.accepted_by_user_id), name: String(o.accepted_by_name) }
          : undefined,
      finishedAt: typeof o.finished_at === 'number' ? Number(o.finished_at) : undefined,
      finishImages: parseJson<string[]>(o.finish_images_json, []),
      finishNote: typeof o.finish_note === 'string' ? String(o.finish_note) : undefined,
      review,
      items,
      totalCent: Number(o.total_cent)
    }
  }

  const listOrders: Store['listOrders'] = ({ userId, scope, status, page, pageSize }) => {
    const where: string[] = []
    const params: any[] = []
    if (scope === 'mine') {
      where.push(`placed_by_user_id = ?`)
      params.push(userId)
    }
    if (status) {
      where.push(`status = ?`)
      params.push(status)
    }
    const whereSql = where.length ? `WHERE ${where.join(' AND ')}` : ''

    const totalRow = db.prepare(`SELECT COUNT(1) AS cnt FROM orders ${whereSql}`).get(...params) as any
    const total = Number(totalRow?.cnt ?? 0)

    const offset = (page - 1) * pageSize
    const orderRows = db
      .prepare(
        `
          SELECT id, created_at, updated_at, status, placed_by_user_id, placed_by_name, placed_note, accepted_by_user_id, accepted_by_name, finished_at, finish_images_json, finish_note, total_cent
          FROM orders
          ${whereSql}
          ORDER BY created_at DESC
          LIMIT ? OFFSET ?
        `
      )
      .all(...params, pageSize, offset) as any[]

    const items: Order[] = orderRows.map((o) => {
      const rows = db
        .prepare(`SELECT dish_id, dish_name, qty, price_cent FROM order_items WHERE order_id = ? ORDER BY id ASC`)
        .all(String(o.id)) as any[]
      const lines: OrderItem[] = rows.map((r) => ({
        dishId: String(r.dish_id),
        dishName: String(r.dish_name),
        qty: Number(r.qty),
        priceCent: Number(r.price_cent)
      }))

      const review = getReviewByOrderId(String(o.id))
      return {
        id: String(o.id),
        createdAt: Number(o.created_at),
        updatedAt: Number(o.updated_at),
        status: o.status as OrderStatus,
        placedBy: { userId: String(o.placed_by_user_id), name: String(o.placed_by_name) },
        placedNote: typeof o.placed_note === 'string' ? String(o.placed_note) : undefined,
        acceptedBy:
          typeof o.accepted_by_user_id === 'string' && typeof o.accepted_by_name === 'string'
            ? { userId: String(o.accepted_by_user_id), name: String(o.accepted_by_name) }
            : undefined,
        finishedAt: typeof o.finished_at === 'number' ? Number(o.finished_at) : undefined,
        finishImages: parseJson<string[]>(o.finish_images_json, []),
        finishNote: typeof o.finish_note === 'string' ? String(o.finish_note) : undefined,
        review,
        items: lines,
        totalCent: Number(o.total_cent)
      }
    })

    return { items, total }
  }

  const cancelOrder: Store['cancelOrder'] = ({ orderId, userId }) => {
    db.exec('BEGIN')
    try {
      const order = getOrderById(orderId)
      if (!order) {
        const err = new Error('ORDER_NOT_FOUND')
        ;(err as any).code = 'ORDER_NOT_FOUND'
        throw err
      }
      if (order.placedBy.userId !== userId) {
        const err = new Error('UNAUTHORIZED')
        ;(err as any).code = 'UNAUTHORIZED'
        throw err
      }
      if (order.status !== 'placed') {
        const err = new Error('ORDER_INVALID_STATUS')
        ;(err as any).code = 'ORDER_INVALID_STATUS'
        ;(err as any).details = { status: order.status }
        throw err
      }

      const nextUpdatedAt = Date.now()
      db.prepare(`UPDATE orders SET status = ?, updated_at = ? WHERE id = ?`).run('cancelled', nextUpdatedAt, orderId)
      db.prepare(`UPDATE users SET love_milli = love_milli + ? WHERE id = ?`).run(order.totalCent, userId)
      db.exec('COMMIT')

      const me = getUserById(userId)
      if (!me) {
        const err = new Error('UNAUTHORIZED')
        ;(err as any).code = 'UNAUTHORIZED'
        throw err
      }

      return { order: { ...order, status: 'cancelled', updatedAt: nextUpdatedAt }, me }
    } catch (e) {
      db.exec('ROLLBACK')
      throw e
    }
  }

  const acceptOrder: Store['acceptOrder'] = ({ orderId, acceptedBy }) => {
    db.exec('BEGIN')
    try {
      const order = getOrderById(orderId)
      if (!order) {
        const err = new Error('ORDER_NOT_FOUND')
        ;(err as any).code = 'ORDER_NOT_FOUND'
        throw err
      }
      if (order.status !== 'placed') {
        const err = new Error('ORDER_INVALID_STATUS')
        ;(err as any).code = 'ORDER_INVALID_STATUS'
        ;(err as any).details = { status: order.status }
        throw err
      }
      const nextUpdatedAt = Date.now()
      db.prepare(`UPDATE orders SET status = ?, updated_at = ?, accepted_by_user_id = ?, accepted_by_name = ? WHERE id = ?`).run(
        'accepted',
        nextUpdatedAt,
        acceptedBy.userId,
        acceptedBy.name,
        orderId
      )
      db.exec('COMMIT')
      return { ...order, status: 'accepted', updatedAt: nextUpdatedAt, acceptedBy }
    } catch (e) {
      db.exec('ROLLBACK')
      throw e
    }
  }

  const finishOrder: Store['finishOrder'] = ({ orderId, finishedByUserId, finishImages, note }) => {
    db.exec('BEGIN')
    try {
      const order = getOrderById(orderId)
      if (!order) {
        const err = new Error('ORDER_NOT_FOUND')
        ;(err as any).code = 'ORDER_NOT_FOUND'
        throw err
      }
      if (order.status !== 'accepted') {
        const err = new Error('ORDER_INVALID_STATUS')
        ;(err as any).code = 'ORDER_INVALID_STATUS'
        ;(err as any).details = { status: order.status }
        throw err
      }
      if (!order.acceptedBy?.userId) {
        const err = new Error('ORDER_INVALID_STATUS')
        ;(err as any).code = 'ORDER_INVALID_STATUS'
        ;(err as any).details = { status: order.status }
        throw err
      }
      if (order.acceptedBy.userId !== finishedByUserId) {
        const err = new Error('UNAUTHORIZED')
        ;(err as any).code = 'UNAUTHORIZED'
        throw err
      }
      const nextUpdatedAt = Date.now()
      const finishedAt = Date.now()
      db.prepare(`UPDATE orders SET status = ?, updated_at = ?, finished_at = ?, finish_images_json = ?, finish_note = ? WHERE id = ?`).run(
        'done',
        nextUpdatedAt,
        finishedAt,
        JSON.stringify(finishImages),
        note ?? null,
        orderId
      )
      db.prepare(`UPDATE users SET love_milli = love_milli + ? WHERE id = ?`).run(order.totalCent, order.acceptedBy.userId)
      db.exec('COMMIT')
      const me = getUserById(order.acceptedBy.userId)
      if (!me) {
        const err = new Error('UNAUTHORIZED')
        ;(err as any).code = 'UNAUTHORIZED'
        throw err
      }
      return {
        order: { ...order, status: 'done', updatedAt: nextUpdatedAt, finishedAt, finishImages, finishNote: note ?? undefined },
        me
      }
    } catch (e) {
      db.exec('ROLLBACK')
      throw e
    }
  }

  const createReview: Store['createReview'] = ({ orderId, userId, rating, content, images }) => {
    db.exec('BEGIN')
    try {
      const order = getOrderById(orderId)
      if (!order) {
        const err = new Error('ORDER_NOT_FOUND')
        ;(err as any).code = 'ORDER_NOT_FOUND'
        throw err
      }
      if (order.placedBy.userId !== userId) {
        const err = new Error('UNAUTHORIZED')
        ;(err as any).code = 'UNAUTHORIZED'
        throw err
      }
      if (order.status !== 'done') {
        const err = new Error('ORDER_INVALID_STATUS')
        ;(err as any).code = 'ORDER_INVALID_STATUS'
        ;(err as any).details = { status: order.status }
        throw err
      }
      const exists = db.prepare(`SELECT 1 FROM order_reviews WHERE order_id = ? LIMIT 1`).get(orderId)
      if (exists) {
        const err = new Error('ORDER_ALREADY_REVIEWED')
        ;(err as any).code = 'ORDER_ALREADY_REVIEWED'
        throw err
      }
      const createdAt = Date.now()
      db.prepare(
        `INSERT INTO order_reviews (order_id, rating, content, images_json, created_at, created_by_user_id) VALUES (?, ?, ?, ?, ?, ?)`
      ).run(orderId, rating, content, JSON.stringify(images), createdAt, userId)
      const nextUpdatedAt = Date.now()
      db.prepare(`UPDATE orders SET updated_at = ? WHERE id = ?`).run(nextUpdatedAt, orderId)
      db.exec('COMMIT')
      return { ...order, updatedAt: nextUpdatedAt, review: { rating, content, images, createdAt } }
    } catch (e) {
      db.exec('ROLLBACK')
      throw e
    }
  }

  const getDb: Store['getDb'] = () => {
    const users = db
      .prepare(`SELECT id, account, name, password_salt, password_hash, created_at, love_milli FROM users ORDER BY created_at ASC`)
      .all() as any[]
    const dishes = listDishes({ page: 1, pageSize: Number.MAX_SAFE_INTEGER }).items
    const orders = listOrders({ userId: '', scope: 'all', page: 1, pageSize: Number.MAX_SAFE_INTEGER }).items
    return {
      users: users.map((u) => ({
        id: String(u.id),
        account: String(u.account),
        name: String(u.name),
        password: { salt: String(u.password_salt), hash: String(u.password_hash) },
        createdAt: Number(u.created_at),
        loveMilli: Number(u.love_milli ?? 0)
      })),
      dishes,
      orders
    }
  }

  return {
    getDb,
    persist,
    register,
    login,
    getUserById,
    listUsers,
    listDishes,
    getDishById,
    createDish,
    deleteDish,
    createOrder,
    listOrders,
    getOrderById,
    cancelOrder,
    acceptOrder,
    finishOrder,
    createReview
  }
}

export const createOpaqueToken = (userId: string, secret: string) => {
  const h = createHash('sha256')
  h.update(secret)
  h.update(':')
  h.update(userId)
  h.update(':')
  h.update(randomBytes(16))
  return h.digest('hex')
}
