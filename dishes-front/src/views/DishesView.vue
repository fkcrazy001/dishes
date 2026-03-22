<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { useAppStore, type CategoryId, type Dish, type Order } from '../store/appStore'

const store = useAppStore()
const router = useRouter()

const activeCategory = ref<CategoryId>('all')
const query = ref('')

const filteredDishes = computed(() => store.dishes.value)

const orderTotalCount = computed(() => store.orderItems.value.reduce((s, x) => s + x.qty, 0))
const hasCart = computed(() => orderTotalCount.value > 0)

const selectedDishId = ref<string | null>(null)
const selectedDish = ref<Dish | null>(null)
const selectedDishLoading = ref(false)

const myOrders = computed(() => store.ordersMine.value.slice().sort((a, b) => b.createdAt - a.createdAt))
const myAcceptedOrders = computed(() =>
  store.ordersAll.value
    .filter((o) => o.acceptedBy?.userId === store.auth.userId)
    .slice()
    .sort((a, b) => b.createdAt - a.createdAt)
)

const mineFeed = computed(() => {
  const mine = myOrders.value.map((o) => ({ kind: 'placed' as const, order: o }))
  const accepted = myAcceptedOrders.value.map((o) => ({ kind: 'accepted' as const, order: o }))
  const merged = [...mine, ...accepted]
  merged.sort((a, b) => b.order.createdAt - a.order.createdAt)

  const seen = new Set<string>()
  const out: typeof merged = []
  merged.forEach((x) => {
    if (seen.has(x.order.id)) return
    seen.add(x.order.id)
    out.push(x)
  })
  return out
})

const loveText = computed(() => store.formatLoveMilli(store.auth.loveMilli))
const cartLoveText = computed(() => store.formatLoveMilli(store.loveMilliFromCent(store.cartTotalCent.value)))
const cartMoneyText = computed(() => store.formatMoneyCent(store.cartTotalCent.value))

const loveRankShow = ref(false)
const loveRankLoading = ref(false)

const openLoveRank = async () => {
  loveRankShow.value = true
  loveRankLoading.value = true
  try {
    await store.fetchUsersLoveRank({ limit: 50 })
  } catch (e) {
    store.openErrorDialog('加载排行榜失败', e, '加载排行榜失败')
    closeLoveRank()
  } finally {
    loveRankLoading.value = false
  }
}

const closeLoveRank = () => {
  loveRankShow.value = false
  loveRankLoading.value = false
}

const cartSheetShow = ref(false)
const cartNote = ref('')

const openCartSheet = () => {
  cartSheetShow.value = true
}

const closeCartSheet = () => {
  cartSheetShow.value = false
}

const orderDetailId = ref<string | null>(null)
const orderDetail = ref<Order | null>(null)
const orderDetailLoading = ref(false)
const reviewRating = ref(5)
const reviewContent = ref('')
const reviewImages = ref<File[]>([])
const reviewUploading = ref(false)

const canCancelOrder = (o: Order) => o.status === 'placed' && o.placedBy?.userId === store.auth.userId
const canReviewOrder = (o: Order) => o.status === 'done' && o.placedBy?.userId === store.auth.userId

const openOrderDetail = async (orderId: string) => {
  orderDetailId.value = orderId
  orderDetail.value = null
  orderDetailLoading.value = true
  try {
    orderDetail.value = await store.fetchOrderDetail(orderId)
    reviewRating.value = orderDetail.value.review?.rating ?? 5
    reviewContent.value = orderDetail.value.review?.content ?? ''
    reviewImages.value = []
  } catch (e) {
    store.openErrorDialog('加载订单失败', e, '加载订单失败')
    closeOrderDetail()
  } finally {
    orderDetailLoading.value = false
  }
}

const closeOrderDetail = () => {
  orderDetailId.value = null
  orderDetail.value = null
  orderDetailLoading.value = false
  reviewUploading.value = false
  reviewImages.value = []
}

const onReviewFiles = (e: Event) => {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  reviewImages.value = files.slice(0, 3)
}

const submitReview = async () => {
  if (!orderDetail.value) return
  const content = reviewContent.value.trim()
  if (!content) {
    store.openDialog({ title: '评价还没写', message: '写一句感受再提交吧～', tone: 'error' })
    return
  }
  reviewUploading.value = true
  try {
    await store.submitOrderReview({
      orderId: orderDetail.value.id,
      rating: reviewRating.value,
      content,
      images: reviewImages.value,
    })
    store.showToast('评价已提交')
    closeOrderDetail()
  } catch (e) {
    store.openErrorDialog('提交评价失败', e, '提交评价失败')
  } finally {
    reviewUploading.value = false
  }
}

const cancelMyOrder = async (orderId: string) => {
  try {
    await store.cancelOrder(orderId)
    if (orderDetail.value?.id === orderId) orderDetail.value = await store.fetchOrderDetail(orderId)
  } catch (e) {
    store.openErrorDialog('取消失败', e, '取消失败')
  }
}

const finishShow = ref(false)
const finishOrderId = ref<string | null>(null)
const finishImages = ref<File[]>([])
const finishNote = ref('')
const finishLoading = ref(false)

const openFinishModal = (orderId: string) => {
  finishShow.value = true
  finishOrderId.value = orderId
  finishImages.value = []
  finishNote.value = ''
  finishLoading.value = false
}

const closeFinishModal = () => {
  finishShow.value = false
  finishOrderId.value = null
  finishImages.value = []
  finishNote.value = ''
  finishLoading.value = false
}

const onFinishFiles = (e: Event) => {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  finishImages.value = files.slice(0, 3)
}

const submitFinish = async () => {
  if (!finishOrderId.value) return
  finishLoading.value = true
  try {
    await store.finishOrder({ orderId: finishOrderId.value, images: finishImages.value, note: finishNote.value.trim() || undefined })
    closeFinishModal()
  } catch (e) {
    store.openErrorDialog('完成失败', e, '完成失败')
    if (!store.auth.loggedIn) await router.replace('/login')
  } finally {
    finishLoading.value = false
  }
}

const getOrderTotalMoney = (o: Order) => {
  return o.totalCent ?? o.items.reduce((s, it) => s + it.priceCent * it.qty, 0)
}

const closeDishDetail = () => {
  selectedDishId.value = null
  selectedDish.value = null
  selectedDishLoading.value = false
}

const openDishDetail = async (dishId: string) => {
  selectedDishId.value = dishId
  selectedDish.value = null
  selectedDishLoading.value = true
  try {
    selectedDish.value = await store.fetchDishDetail(dishId)
  } catch (e) {
    const err = e as { message?: string }
    store.showToast(err.message ?? '加载菜谱详情失败')
    closeDishDetail()
  } finally {
    selectedDishLoading.value = false
  }
}

const onKeydown = (e: KeyboardEvent) => {
  if (e.key !== 'Escape') return
  closeDishDetail()
  closeCartSheet()
  closeLoveRank()
  closeOrderDetail()
  closeFinishModal()
  closeDishCreate()
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))

const getOrderStatusLabel = (status: Order['status']) => {
  if (status === 'placed') return '已下单'
  if (status === 'accepted') return '制作中'
  if (status === 'done') return '已完成'
  return '已取消'
}

const getOrderStatusTone = (status: Order['status']) => {
  if (status === 'placed') return 'placed'
  if (status === 'accepted') return 'accepted'
  if (status === 'done') return 'done'
  return 'cancelled'
}

const getOrderSummary = (o: Order) => {
  const parts = o.items.map((it) => `${it.dishName ?? '未知菜品'}×${it.qty}`).join('，')
  return parts || '（空订单）'
}

const refreshDishes = async () => {
  const category = activeCategory.value === 'all' ? undefined : activeCategory.value
  const q = query.value.trim()
  try {
    await store.fetchDishes({ category, q })
  } catch (e) {
    const err = e as { message?: string }
    store.showToast(err.message ?? '加载菜谱失败')
  }
}

let searchTimer: number | undefined
watch([activeCategory, query], () => {
  if (searchTimer) window.clearTimeout(searchTimer)
  searchTimer = window.setTimeout(() => {
    refreshDishes()
  }, 250)
})

const refreshOrders = async () => {
  if (!store.auth.loggedIn) return
  try {
    if (store.mode.value === 'mine') {
      await store.fetchAllOrders('mine')
      await store.fetchAllOrders('all')
    } else {
      await store.fetchOrders({ scope: 'mine' })
    }
    if (store.mode.value === 'cook') await store.fetchOrders({ scope: 'all' })
  } catch (e) {
    const err = e as { message?: string }
    store.showToast(err.message ?? '加载订单失败')
    if (!store.auth.loggedIn) await router.replace('/login')
  }
}

onMounted(async () => {
  await refreshDishes()
  await refreshOrders()
  try {
    store.connectOrdersWs('mine')
    if (store.mode.value === 'cook' || store.mode.value === 'mine') store.connectOrdersWs('all')
  } catch (e) {
    store.openErrorDialog('连接失败', e, '订单状态实时连接失败')
  }
})

watch(
  () => store.mode.value,
  (m) => {
    if (m === 'cook' || m === 'mine') {
      try {
        store.connectOrdersWs('all')
      } catch (e) {
        store.openErrorDialog('连接失败', e, '订单状态实时连接失败')
      }
    } else {
      store.disconnectOrdersWs('all')
    }
    if (m === 'mine') refreshOrders()
    if (m === 'dishes') {
      store.fetchMyDishes().catch((e) => store.openErrorDialog('加载菜谱失败', e, '加载菜谱失败'))
    }
  }
)

onBeforeUnmount(() => {
  store.disconnectOrdersWs('mine')
  store.disconnectOrdersWs('all')
})

const onPlaceOrder = async () => {
  try {
    await store.placeOrder({ note: cartNote.value })
    await refreshOrders()
    closeCartSheet()
    cartNote.value = ''
  } catch (e) {
    const err = e as { message?: string }
    store.showToast(err.message ?? '下单失败')
    if (!store.auth.loggedIn) await router.replace('/login')
  }
}

const onAcceptOrder = async (orderId: string) => {
  try {
    await store.acceptOrder(orderId)
  } catch (e) {
    const err = e as { message?: string }
    store.showToast(err.message ?? '接单失败')
    if (!store.auth.loggedIn) await router.replace('/login')
  }
}

const onFinishOrder = async (orderId: string) => {
  openFinishModal(orderId)
}

const logout = async () => {
  store.logout()
  store.showToast('已退出登录')
  await router.replace('/login')
}

const dishCreateShow = ref(false)
const dishCreateLoading = ref(false)
const dishName = ref('')
const dishCategory = ref<Exclude<CategoryId, 'all'>>('home')
const dishTimeText = ref('15 分钟')
const dishLevel = ref<'easy' | 'medium' | 'hard'>('easy')
const dishTagsText = ref('')
const dishPriceYuan = ref('18')
const dishStory = ref('')
const dishBadge = ref('')
const dishImageUrl = ref('')
const dishIngredientsText = ref('')
const dishStepsText = ref('')

const openDishCreate = () => {
  dishCreateShow.value = true
  resetDishForm()
}

const closeDishCreate = () => {
  dishCreateShow.value = false
  dishCreateLoading.value = false
}

const resetDishForm = () => {
  dishName.value = ''
  dishCategory.value = 'home'
  dishTimeText.value = '15 分钟'
  dishLevel.value = 'easy'
  dishTagsText.value = ''
  dishPriceYuan.value = '18'
  dishStory.value = ''
  dishBadge.value = ''
  dishImageUrl.value = ''
  dishIngredientsText.value = ''
  dishStepsText.value = ''
}

const deleteMyDish = async (dishId: string) => {
  const ok = window.confirm('确认删除这道菜谱吗？删除后不可恢复。')
  if (!ok) return
  try {
    await store.deleteDish(dishId)
    store.showToast('已删除菜谱')
  } catch (e) {
    store.openErrorDialog('删除失败', e, '删除失败')
  }
}

const myCreatedDishes = computed(() =>
  store.dishesMine.value.filter((d) => d.createdBy?.userId && d.createdBy.userId === store.auth.userId)
)

const parseLines = (text: string) => {
  return text
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)
}

const parseTags = (text: string) => {
  return text
    .split(/[，,]/g)
    .map((s) => s.trim())
    .filter(Boolean)
}

const toPriceCent = (yuanText: string) => {
  const n = Number(yuanText)
  if (!Number.isFinite(n) || n <= 0) return null
  return Math.round(n * 100)
}

const dishLovePreview = computed(() => {
  const cent = toPriceCent(dishPriceYuan.value)
  if (cent === null) return null
  return store.formatLoveMilli(store.loveMilliFromCent(cent))
})

const submitDishCreate = async () => {
  const name = dishName.value.trim()
  const timeText = dishTimeText.value.trim()
  const story = dishStory.value.trim()
  const priceCent = toPriceCent(dishPriceYuan.value)
  const ingredients = parseLines(dishIngredientsText.value)
  const steps = parseLines(dishStepsText.value)
  const tags = parseTags(dishTagsText.value)

  if (!name || !timeText || !story || priceCent === null) {
    store.openDialog({ title: '新增菜谱失败', message: '把菜名、耗时、价格和简介填好再试试～', tone: 'error' })
    return
  }
  if (ingredients.length === 0 || steps.length === 0) {
    store.openDialog({ title: '新增菜谱失败', message: '食材和做法至少各写一条～', tone: 'error' })
    return
  }

  dishCreateLoading.value = true
  try {
    await store.createDish({
      name,
      category: dishCategory.value,
      timeText,
      level: dishLevel.value,
      tags,
      priceCent,
      story,
      badge: dishBadge.value.trim() || undefined,
      imageUrl: dishImageUrl.value.trim() || undefined,
      details: { ingredients, steps },
    })
    store.showToast('已新增菜谱')
    closeDishCreate()
    await store.fetchMyDishes()
  } catch (e) {
    store.openErrorDialog('新增菜谱失败', e, '新增菜谱失败')
  } finally {
    dishCreateLoading.value = false
  }
}
</script>

<template>
  <div class="wrap dishes-wrap">
    <header class="top">
      <div class="brand">
        <div class="badge">家</div>
        <div>
          <h1>家庭厨房</h1>
          <p>点菜下单，或者切到“做菜”去接单处理</p>
        </div>
      </div>

      <div v-if="store.mode.value === 'order'" class="controls">
        <div class="search" role="search">
          <span aria-hidden="true">🔎</span>
          <input v-model="query" placeholder="搜索：番茄 / 鸡 / 汤 / 甜…" autocomplete="off" />
        </div>
      </div>

      <div class="top-right">
        <div class="nav" aria-label="模式切换">
          <button type="button" class="chip" :data-active="String(store.mode.value === 'order')" @click="store.mode.value = 'order'">
            点菜
          </button>
          <button type="button" class="chip" :data-active="String(store.mode.value === 'cook')" @click="store.mode.value = 'cook'">
            做菜
          </button>
          <button type="button" class="chip" :data-active="String(store.mode.value === 'mine')" @click="store.mode.value = 'mine'">
            我的订单
          </button>
          <button type="button" class="chip" :data-active="String(store.mode.value === 'dishes')" @click="store.mode.value = 'dishes'">
            菜谱管理
          </button>
          <button type="button" class="chip" :data-active="String(loveRankShow)" @click="openLoveRank">爱心 {{ loveText }}</button>
          <button type="button" class="chip" @click="logout">退出</button>
        </div>

        <div v-if="store.mode.value === 'order'" class="chips" aria-label="分类筛选">
          <button
            v-for="c in store.categories"
            :key="c.id"
            type="button"
            class="chip"
            :data-active="String(activeCategory === c.id)"
            @click="activeCategory = c.id"
          >
            {{ c.name }}
          </button>
        </div>
      </div>
    </header>

    <section v-if="store.mode.value === 'order'" class="main">
      <div class="panel">
        <div class="panel-head">
          <h2>今日菜单</h2>
        </div>
        <div class="menu">
          <div class="grid">
            <article v-for="d in filteredDishes" :key="d.id" class="card">
              <div class="media">
                <button type="button" class="media-btn" :aria-label="`查看 ${d.name} 详情`" @click="openDishDetail(d.id)">
                  <img :alt="d.name" loading="lazy" :src="store.createDishImage(d)" />
                </button>
                <div class="pin"><b>★</b><span>{{ d.badge }}</span></div>
              </div>
              <div class="content">
                <div class="title">
                  <h3>{{ d.name }}</h3>
                  <div class="price">{{ store.formatMoneyCent(d.priceCent) }}</div>
                </div>
                <p class="desc">{{ d.story }}</p>
                <div class="row">
                  <span class="tag warm">⏱ {{ d.timeText }}</span>
                  <span class="tag good">🍳 {{ store.dishLevelText(d.level) }}</span>
                  <span v-for="t in d.tags.slice(0, 2)" :key="t" class="tag">{{ t }}</span>
                </div>
                <div class="cta">
                  <div class="hint">点单后可随时查看并调整</div>
                  <button type="button" class="btn primary" @click="store.addToCart(d.id)">加入点单</button>
                </div>
              </div>
            </article>
          </div>
        </div>
      </div>
    </section>

    <section v-else-if="store.mode.value === 'cook'" class="kitchen">
      <div class="panel">
        <div class="panel-head">
          <h2>做菜 · 订单看板</h2>
          <div class="meta">待接单 {{ store.placedOrders.value.length }} · 制作中 {{ store.acceptedOrders.value.length }}</div>
        </div>
        <div class="kitchen-body">
          <div class="kitchen-cols">
            <div class="kcol">
              <div class="kcol-head">待接单</div>
              <div v-if="store.placedOrders.value.length === 0" class="order-empty">暂无待接单</div>
              <div v-else class="klist">
                <div v-for="o in store.placedOrders.value" :key="o.id" class="order-card">
                  <div class="order-top">
                    <div class="order-id">{{ o.placedBy?.name ?? '匿名' }} 下的单</div>
                    <div class="order-meta">{{ store.formatOrderTime(o.createdAt) }}</div>
                  </div>
                  <div class="order-sub">{{ o.id }}</div>
                  <div class="order-items">
                    <div v-for="it in o.items" :key="it.dishId" class="order-item">
                      <span class="oi-name">{{ it.dishName ?? '未知菜品' }}</span>
                      <span class="oi-qty">×{{ it.qty }}</span>
                    </div>
                  </div>
                  <div class="order-actions">
                    <div class="order-money">{{ store.formatMoneyCent(getOrderTotalMoney(o)) }}</div>
                    <div class="order-reward">爱心 +{{ store.formatLoveMilli(store.loveMilliFromCent(getOrderTotalMoney(o))) }}</div>
                    <button type="button" class="btn primary" @click="onAcceptOrder(o.id)">接单</button>
                  </div>
                </div>
              </div>
            </div>

            <div class="kcol">
              <div class="kcol-head">制作中</div>
              <div v-if="store.acceptedOrders.value.length === 0" class="order-empty">暂无制作中订单</div>
              <div v-else class="klist">
                <div v-for="o in store.acceptedOrders.value" :key="o.id" class="order-card">
                  <div class="order-top">
                    <div class="order-id">{{ o.placedBy?.name ?? '匿名' }} 下的单</div>
                    <div class="order-meta">{{ store.formatOrderTime(o.createdAt) }}</div>
                  </div>
                  <div class="order-sub">{{ o.id }}</div>
                  <div class="order-items">
                    <div v-for="it in o.items" :key="it.dishId" class="order-item">
                      <span class="oi-name">{{ it.dishName ?? '未知菜品' }}</span>
                      <span class="oi-qty">×{{ it.qty }}</span>
                    </div>
                  </div>
                  <div class="order-actions">
                    <div class="order-money">{{ store.formatMoneyCent(getOrderTotalMoney(o)) }}</div>
                    <div class="order-reward">爱心 +{{ store.formatLoveMilli(store.loveMilliFromCent(getOrderTotalMoney(o))) }}</div>
                    <button type="button" class="btn primary" @click="onFinishOrder(o.id)">完成</button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section v-else-if="store.mode.value === 'dishes'" class="dishes-manage">
      <div class="panel">
        <div class="panel-head">
          <h2>菜谱管理</h2>
          <div class="meta">我的菜谱 {{ myCreatedDishes.length }} 道</div>
        </div>
        <div class="mine-body">
          <div class="actions end">
            <button type="button" class="btn primary" @click="openDishCreate">创建菜谱</button>
          </div>

          <div v-if="myCreatedDishes.length === 0" class="order-empty">还没有创建过菜谱</div>
          <ul v-else class="dish-list">
            <li v-for="d in myCreatedDishes" :key="d.id" class="dish-card">
              <div class="dish-top">
                <div>
                  <div class="dish-name">{{ d.name }}</div>
                  <div class="dish-sub">{{ d.id }}</div>
                </div>
                <button type="button" class="btn ghost" @click="deleteMyDish(d.id)">删除</button>
              </div>
              <div class="dish-meta">
                <span class="tag warm">⏱ {{ d.timeText }}</span>
                <span class="tag good">🍳 {{ store.dishLevelText(d.level) }}</span>
                <span class="tag">{{ store.formatMoneyCent(d.priceCent) }}</span>
              </div>
              <div class="dish-desc">{{ d.story }}</div>
            </li>
          </ul>
        </div>
      </div>
    </section>

    <section v-else class="mine-orders">
      <div class="panel">
        <div class="panel-head">
          <h2>我的订单</h2>
          <div class="meta">{{ mineFeed.length }} 条</div>
        </div>
        <div class="mine-body">
          <div v-if="mineFeed.length === 0" class="order-empty">还没有订单记录</div>
          <ul v-else class="history-list">
            <li v-for="x in mineFeed" :key="`${x.kind}-${x.order.id}`" class="history-card" :data-kind="x.kind">
              <button type="button" class="history-btn" @click="openOrderDetail(x.order.id)">
                <div class="history-top">
                  <div>
                    <div class="history-id">{{ x.order.placedBy?.name ?? '匿名' }} 下的单</div>
                    <div class="history-sub">
                      <span class="kind-badge" :data-kind="x.kind">{{ x.kind === 'placed' ? '我下的' : '我接的' }}</span>
                      <span>{{ x.order.id }}</span>
                    </div>
                  </div>
                  <div class="history-right">
                    <span class="status-badge" :class="getOrderStatusTone(x.order.status)">{{ getOrderStatusLabel(x.order.status) }}</span>
                    <span class="history-time">{{ store.formatOrderTime(x.order.createdAt) }}</span>
                  </div>
                </div>
                <div class="history-items">{{ getOrderSummary(x.order) }}</div>
                <div class="history-foot">
                  <div class="history-money">{{ store.formatMoneyCent(getOrderTotalMoney(x.order)) }}</div>
                  <div class="history-hint">
                    <span v-if="x.order.acceptedBy?.name">接单：{{ x.order.acceptedBy.name }}</span>
                    <span v-else-if="x.order.status === 'cancelled'">已取消</span>
                    <span v-else>等待接单中</span>
                  </div>
                </div>
              </button>

              <div v-if="x.kind === 'placed' && canCancelOrder(x.order)" class="history-actions">
                <button type="button" class="btn ghost history-cancel" @click.stop="cancelMyOrder(x.order.id)">取消订单</button>
              </div>
            </li>
          </ul>
        </div>
      </div>
    </section>
  </div>

  <div v-if="store.mode.value === 'order' && hasCart" class="cartbar" role="region" aria-label="点单汇总">
    <div class="cartbar-inner">
      <button type="button" class="cartbar-left" @click="openCartSheet">
        <div class="cartbar-title">已点 {{ orderTotalCount }} 份</div>
        <div class="cartbar-sub">爱心 {{ cartLoveText }} · {{ cartMoneyText }}</div>
      </button>
      <button type="button" class="btn primary cartbar-btn" @click="onPlaceOrder">下单</button>
    </div>
  </div>

  <div v-if="cartSheetShow" class="modal" role="dialog" aria-modal="true" @click.self="closeCartSheet">
    <div class="modal-card cartsheet">
      <div class="modal-head">
        <div>
          <div class="modal-title">我的点单</div>
          <div class="modal-sub">
            <span class="tag warm">爱心 {{ cartLoveText }}</span>
            <span class="tag">{{ cartMoneyText }}</span>
          </div>
        </div>
        <button type="button" class="btn ghost" @click="closeCartSheet">关闭</button>
      </div>

      <div class="modal-body cartsheet-body">
        <div v-if="store.orderItems.value.length === 0" class="order-empty">还没有点菜哦～</div>
        <ul v-else class="list">
          <li v-for="x in store.orderItems.value" :key="x.dish.id" class="li">
            <div>
              <strong>{{ x.dish.name }}</strong>
              <div class="sub">
                {{ store.formatMoneyCent(x.dish.priceCent) }} / 份 · {{ x.dish.timeText }} · {{ store.dishLevelText(x.dish.level) }}
              </div>
            </div>
            <div class="qty">
              <button type="button" :aria-label="`减少 ${x.dish.name}`" @click="store.updateCartQty(x.dish.id, -1)">−</button>
              <span>{{ x.qty }}</span>
              <button type="button" :aria-label="`增加 ${x.dish.name}`" @click="store.updateCartQty(x.dish.id, 1)">+</button>
            </div>
          </li>
        </ul>

        <div class="order-foot cartsheet-foot">
          <label class="field">
            <div class="field-label">订单备注（可选）</div>
            <textarea
              v-model="cartNote"
              class="field-input textarea"
              rows="3"
              placeholder="例如：少盐少油、不要香菜、家里有宝宝…"
            ></textarea>
          </label>

          <div class="sum">
            <div>
              <div class="label">将扣除爱心</div>
              <div class="hint">1 元 = 0.1 爱心</div>
            </div>
            <div class="money">{{ cartLoveText }}</div>
          </div>
          <div class="actions">
            <button type="button" class="btn ghost" @click="store.clearCart">清空</button>
            <button type="button" class="btn primary" @click="onPlaceOrder">确认下单</button>
          </div>
        </div>
      </div>
    </div>
  </div>

  <div v-if="loveRankShow" class="modal" role="dialog" aria-modal="true" @click.self="closeLoveRank">
    <div class="modal-card">
      <div class="modal-head">
        <div>
          <div class="modal-title">爱心排行榜</div>
          <div class="modal-sub">
            <span class="tag warm">按爱心值排序</span>
            <span class="tag">TOP {{ store.usersLoveRank.value.length }}</span>
          </div>
        </div>
        <button type="button" class="btn ghost" @click="closeLoveRank">关闭</button>
      </div>

      <div class="modal-body cartsheet-body">
        <div class="modal-content">
          <div class="modal-section">
            <div class="modal-h">榜单</div>
            <div v-if="loveRankLoading" class="modal-p">加载中…</div>
            <ul v-else class="history-list">
              <li v-for="(u, idx) in store.usersLoveRank.value" :key="u.id" class="history-card" :data-me="String(u.id === store.auth.userId)">
                <div class="history-top">
                  <div>
                    <div class="history-id">{{ u.name }}</div>
                    <div class="history-sub">
                      <span class="kind-badge" data-kind="placed">第 {{ idx + 1 }} 名</span>
                      <span v-if="u.id === store.auth.userId">当前账号</span>
                    </div>
                  </div>
                  <div class="history-right">
                    <span class="status-badge">爱心 {{ store.formatLoveMilli(u.loveMilli) }}</span>
                  </div>
                </div>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </div>
  </div>

  <div v-if="dishCreateShow" class="modal" role="dialog" aria-modal="true" @click.self="closeDishCreate">
    <div class="modal-card">
      <div class="modal-head">
        <div>
          <div class="modal-title">创建菜谱</div>
          <div class="modal-sub">
            <span class="tag warm">填写做法与价格</span>
            <span v-if="dishLovePreview" class="tag good">爱心 {{ dishLovePreview }}</span>
          </div>
        </div>
        <button type="button" class="btn ghost" :disabled="dishCreateLoading" @click="closeDishCreate">关闭</button>
      </div>

      <div class="modal-body">
        <div class="modal-content">
          <div class="modal-section">
            <div class="modal-h">基础信息</div>
            <div class="field-grid">
              <label class="field">
                <div class="field-label">菜名</div>
                <input v-model="dishName" class="field-input" placeholder="例如：葱油拌面" />
              </label>
              <label class="field">
                <div class="field-label">分类</div>
                <select v-model="dishCategory" class="field-input">
                  <option value="home">家常</option>
                  <option value="soup">汤羹</option>
                  <option value="sweet">甜点</option>
                  <option value="quick">快手</option>
                </select>
              </label>
            </div>

            <div class="field-grid">
              <label class="field">
                <div class="field-label">耗时</div>
                <input v-model="dishTimeText" class="field-input" placeholder="例如：20 分钟" />
              </label>
              <label class="field">
                <div class="field-label">难度</div>
                <select v-model="dishLevel" class="field-input">
                  <option value="easy">简单</option>
                  <option value="medium">中等</option>
                  <option value="hard">困难</option>
                </select>
              </label>
            </div>

            <div class="field-grid">
              <label class="field">
                <div class="field-label">价格（元）</div>
                <input v-model="dishPriceYuan" class="field-input" inputmode="decimal" placeholder="例如：18" />
              </label>
              <label class="field">
                <div class="field-label">标签（逗号分隔）</div>
                <input v-model="dishTagsText" class="field-input" placeholder="例如：下饭, 孩子喜欢" />
              </label>
            </div>

            <label class="field">
              <div class="field-label">一句话简介</div>
              <input v-model="dishStory" class="field-input" placeholder="例如：热气腾腾的一碗面，简单又满足" />
            </label>

            <div class="field-grid">
              <label class="field">
                <div class="field-label">徽标（可选）</div>
                <input v-model="dishBadge" class="field-input" placeholder="例如：招牌 / 快手" />
              </label>
              <label class="field">
                <div class="field-label">图片链接（可选）</div>
                <input v-model="dishImageUrl" class="field-input" placeholder="https://..." />
              </label>
            </div>
          </div>
        </div>

        <div class="modal-content">
          <div class="modal-section">
            <div class="modal-h">食材（每行一条）</div>
            <textarea v-model="dishIngredientsText" class="field-input textarea" rows="6" placeholder="鸡蛋 2 个&#10;葱花 少许"></textarea>
          </div>
          <div class="modal-section">
            <div class="modal-h">做法（每行一步）</div>
            <textarea v-model="dishStepsText" class="field-input textarea" rows="6" placeholder="热锅下油&#10;翻炒至金黄"></textarea>
          </div>

          <div class="actions">
            <button type="button" class="btn ghost" :disabled="dishCreateLoading" @click="resetDishForm">重置</button>
            <button type="button" class="btn primary" :disabled="dishCreateLoading" @click="submitDishCreate">
              {{ dishCreateLoading ? '提交中…' : '保存菜谱' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>

  <div v-if="finishShow" class="modal" role="dialog" aria-modal="true" @click.self="closeFinishModal">
    <div class="modal-card">
      <div class="modal-head">
        <div>
          <div class="modal-title">上传完成照片</div>
          <div class="modal-sub">
            <span class="tag warm">最多 3 张</span>
            <span class="tag">{{ finishOrderId }}</span>
          </div>
        </div>
        <button type="button" class="btn ghost" :disabled="finishLoading" @click="closeFinishModal">关闭</button>
      </div>

      <div class="modal-body cartsheet-body">
        <div class="modal-content">
          <div class="modal-section">
            <div class="modal-h">完成照片</div>
            <div class="modal-p">拍一张成品照，让下单的人看到你的手艺～</div>
            <input type="file" accept="image/*" multiple @change="onFinishFiles" />
            <div v-if="finishImages.length" class="modal-p">已选择 {{ finishImages.length }} 张</div>
          </div>

          <div class="modal-section">
            <div class="modal-h">备注（可选）</div>
            <textarea v-model="finishNote" class="field-input textarea" rows="4" placeholder="例如：少盐少油，已按要求做～"></textarea>
          </div>

          <div class="actions">
            <button type="button" class="btn ghost" :disabled="finishLoading" @click="closeFinishModal">取消</button>
            <button type="button" class="btn primary" :disabled="finishLoading" @click="submitFinish">
              {{ finishLoading ? '提交中…' : '确认完成' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>

  <div v-if="orderDetailId" class="modal" role="dialog" aria-modal="true" @click.self="closeOrderDetail">
    <div class="modal-card">
      <div class="modal-head">
        <div>
          <div class="modal-title">{{ orderDetail?.placedBy?.name ?? '匿名' }} 下的单</div>
          <div class="modal-sub">
            <span class="status-badge" :class="getOrderStatusTone(orderDetail?.status ?? 'placed')">
              {{ getOrderStatusLabel(orderDetail?.status ?? 'placed') }}
            </span>
            <span class="tag">{{ orderDetailId }}</span>
          </div>
        </div>
        <button type="button" class="btn ghost" @click="closeOrderDetail">关闭</button>
      </div>

      <div class="modal-body cartsheet-body">
        <div class="modal-content">
          <div v-if="orderDetailLoading" class="modal-section">
            <div class="modal-h">加载中</div>
            <div class="modal-p">正在获取订单详情…</div>
          </div>

          <template v-else-if="orderDetail">
            <div class="modal-section">
              <div class="modal-h">订单信息</div>
              <div class="modal-p">下单人：{{ orderDetail.placedBy?.name ?? '匿名' }}</div>
              <div class="modal-p">
                <span v-if="orderDetail.acceptedBy?.name">接单人：{{ orderDetail.acceptedBy.name }}</span>
                <span v-else>接单人：未接单</span>
              </div>
              <div v-if="orderDetail.placedNote" class="modal-p">备注：{{ orderDetail.placedNote }}</div>
              <div class="modal-p">金额：{{ store.formatMoneyCent(getOrderTotalMoney(orderDetail)) }}（爱心 {{ store.formatLoveMilli(store.loveMilliFromCent(getOrderTotalMoney(orderDetail))) }}）</div>
            </div>

            <div class="modal-section">
              <div class="modal-h">菜品清单</div>
              <div class="modal-p">{{ getOrderSummary(orderDetail) }}</div>
            </div>

            <div v-if="canCancelOrder(orderDetail)" class="modal-section">
              <div class="modal-h">取消订单</div>
              <div class="modal-p">仅在接单前可以取消，取消后会退回扣除的爱心。</div>
              <button type="button" class="btn ghost" @click="cancelMyOrder(orderDetail.id)">取消订单</button>
            </div>

            <div v-if="orderDetail.status === 'done'" class="modal-section">
              <div class="modal-h">完成情况</div>
              <div v-if="orderDetail.finishImages?.length" class="finish-images">
                <a v-for="u in orderDetail.finishImages" :key="u" :href="u" target="_blank" rel="noreferrer">
                  <img :src="u" alt="完成图片" />
                </a>
              </div>
              <div v-else class="modal-p">暂无完成图片</div>
            </div>

            <div v-if="canReviewOrder(orderDetail)" class="modal-section">
              <div class="modal-h">我的评价</div>
              <div class="field-grid">
                <label class="field">
                  <div class="field-label">评分</div>
                  <select v-model="reviewRating" class="field-input">
                    <option :value="5">5</option>
                    <option :value="4">4</option>
                    <option :value="3">3</option>
                    <option :value="2">2</option>
                    <option :value="1">1</option>
                  </select>
                </label>
                <label class="field">
                  <div class="field-label">评价图片（可选，最多 3 张）</div>
                  <input type="file" accept="image/*" multiple @change="onReviewFiles" />
                </label>
              </div>
              <textarea v-model="reviewContent" class="field-input textarea" rows="4" placeholder="写点感受吧～"></textarea>
              <div class="actions">
                <button type="button" class="btn primary" :disabled="reviewUploading" @click="submitReview">
                  {{ reviewUploading ? '提交中…' : '提交评价' }}
                </button>
              </div>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>

  <div v-if="selectedDishId" class="modal" role="dialog" aria-modal="true" @click.self="closeDishDetail">
    <div class="modal-card">
      <div class="modal-head">
        <div>
          <div class="modal-title">{{ selectedDish?.name ?? '菜谱详情' }}</div>
          <div v-if="selectedDish" class="modal-sub">
            <span class="tag warm">⏱ {{ selectedDish.timeText }}</span>
            <span class="tag good">🍳 {{ store.dishLevelText(selectedDish.level) }}</span>
            <span class="tag">{{ store.formatMoneyCent(selectedDish.priceCent) }}</span>
          </div>
        </div>
        <button type="button" class="btn ghost" @click="closeDishDetail">关闭</button>
      </div>

      <div class="modal-body">
        <div v-if="selectedDish" class="modal-media">
          <img :alt="selectedDish.name" :src="store.createDishImage(selectedDish)" />
        </div>
        <div v-else class="modal-media"></div>
        <div class="modal-content">
          <div v-if="selectedDishLoading" class="modal-section">
            <div class="modal-h">加载中</div>
            <div class="modal-p">正在获取菜谱详情…</div>
          </div>
          <template v-else-if="selectedDish">
            <div class="modal-section">
              <div class="modal-h">一句话</div>
              <div class="modal-p">{{ selectedDish.story }}</div>
            </div>

            <div class="modal-section">
              <div class="modal-h">食材</div>
              <div class="modal-p">
                <span v-for="x in selectedDish.details.ingredients" :key="x" class="pill">{{ x }}</span>
              </div>
            </div>

            <div class="modal-section">
              <div class="modal-h">做法</div>
              <ol class="steps">
                <li v-for="s in selectedDish.details.steps" :key="s">{{ s }}</li>
              </ol>
            </div>
          </template>
        </div>
      </div>
    </div>
  </div>
</template>
