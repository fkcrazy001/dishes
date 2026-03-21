export type DishCategory = 'home' | 'soup' | 'sweet' | 'quick'
export type DishLevel = 'easy' | 'medium' | 'hard'

export type DishDetails = {
  ingredients: string[]
  steps: string[]
}

export type Dish = {
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
  details: DishDetails
  createdBy?: { userId: string; name: string }
}

export const seedDishes = (): Dish[] => [
  {
    id: 'tomato-egg',
    name: '番茄炒蛋',
    category: 'home',
    timeText: '10 分钟',
    level: 'easy',
    tags: ['下饭', '孩子喜欢', '酸甜'],
    priceCent: 1800,
    story: '酸甜番茄遇上嫩滑鸡蛋，永远的家常顶流。',
    imageUrl: 'https://picsum.photos/seed/tomato-egg/1200/720',
    badge: '经典',
    details: {
      ingredients: ['番茄 2 个', '鸡蛋 3 个', '葱花 少许', '盐 适量', '糖 少许', '生抽 少许（可选）'],
      steps: [
        '番茄切块，鸡蛋加少许盐打散。',
        '热锅少油，鸡蛋滑炒至凝固盛出。',
        '少油下番茄炒出汁，按口味加盐/糖，喜欢可加一点生抽提鲜。',
        '倒回鸡蛋翻匀，收汁到喜欢的浓稠度，撒葱花出锅。'
      ]
    }
  },
  {
    id: 'braised-pork',
    name: '红烧肉',
    category: 'home',
    timeText: '60 分钟',
    level: 'medium',
    tags: ['软糯', '浓香', '周末限定'],
    priceCent: 4800,
    story: '慢火收汁，油亮不腻，配米饭能吃两碗。',
    imageUrl: 'https://picsum.photos/seed/braised-pork/1200/720',
    badge: '招牌',
    details: {
      ingredients: ['五花肉 500g', '冰糖 20g', '葱姜 适量', '料酒 适量', '生抽 2 勺', '老抽 1/2 勺', '热水 适量'],
      steps: [
        '五花肉切块焯水，冲净浮沫沥干。',
        '小火融化冰糖炒出糖色，倒入肉块上色。',
        '加葱姜和料酒，生抽/老抽调色调味。',
        '加热水没过肉，小火慢炖 40-60 分钟。',
        '开大火收汁，至油亮挂汁即可。'
      ]
    }
  },
  {
    id: 'chicken-soup',
    name: '玉米胡萝卜鸡汤',
    category: 'soup',
    timeText: '45 分钟',
    level: 'easy',
    tags: ['暖胃', '清甜', '加班回血'],
    priceCent: 3600,
    story: '清甜不寡淡，喝一口像回到家。',
    imageUrl: 'https://picsum.photos/seed/chicken-soup/1200/720',
    badge: '温暖',
    details: {
      ingredients: ['鸡腿/半只鸡 1 份', '玉米 1 根', '胡萝卜 1 根', '姜片 3 片', '盐 适量'],
      steps: ['鸡肉焯水去腥，玉米胡萝卜切段。', '锅中加足量水，放鸡肉与姜片大火煮开转小火。', '加入玉米胡萝卜，小火炖 30-45 分钟。', '出锅前加盐调味即可。']
    }
  },
  {
    id: 'mapo-tofu',
    name: '麻婆豆腐',
    category: 'quick',
    timeText: '15 分钟',
    level: 'easy',
    tags: ['香辣', '快手', '豆腐嫩'],
    priceCent: 2200,
    story: '麻辣鲜香但不呛，轻松做出馆子味。',
    imageUrl: 'https://picsum.photos/seed/mapo-tofu/1200/720',
    badge: '快手',
    details: {
      ingredients: ['嫩豆腐 1 盒', '牛肉末/猪肉末 80g', '豆瓣酱 1 勺', '花椒面 少许', '蒜末 葱花 适量', '淀粉水 少许'],
      steps: [
        '豆腐切块焯水 1 分钟（可加少许盐），捞出备用。',
        '炒香肉末，加入豆瓣酱炒出红油。',
        '加水/高汤，放豆腐小火煮 3-5 分钟入味。',
        '淀粉水勾薄芡，撒蒜末葱花花椒面即可。'
      ]
    }
  },
  {
    id: 'fried-rice',
    name: '金黄蛋炒饭',
    category: 'quick',
    timeText: '12 分钟',
    level: 'easy',
    tags: ['剩饭救星', '香气足', '一锅端'],
    priceCent: 1600,
    story: '粒粒分明，锅气一开，幸福感就来了。',
    imageUrl: 'https://picsum.photos/seed/fried-rice/1200/720',
    badge: '一锅',
    details: {
      ingredients: ['隔夜米饭 1 碗', '鸡蛋 1-2 个', '火腿/培根（可选）', '葱花 少许', '盐 适量'],
      steps: ['米饭提前打散，鸡蛋打匀。', '热锅少油，下蛋液，立即加入米饭翻炒裹匀。', '按需加入火腿丁等配料炒香。', '加盐调味，撒葱花出锅。']
    }
  },
  {
    id: 'milk-pudding',
    name: '桂花牛奶布丁',
    category: 'sweet',
    timeText: '20 分钟',
    level: 'easy',
    tags: ['软糯', '治愈', '饭后甜'],
    priceCent: 2000,
    story: '淡淡桂花香，像把温柔装进小碗里。',
    imageUrl: 'https://picsum.photos/seed/milk-pudding/1200/720',
    badge: '甜甜',
    details: {
      ingredients: ['牛奶 300ml', '淡奶油 100ml（可选）', '糖 20-30g', '吉利丁片 5g', '桂花蜜 少许'],
      steps: ['吉利丁片冷水泡软备用。', '牛奶加糖小火加热到温热不沸腾，离火加入吉利丁搅匀融化。', '倒入杯中放凉后冷藏 2-3 小时凝固。', '食用前淋桂花蜜即可。']
    }
  }
]
