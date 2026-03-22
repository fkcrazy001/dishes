package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

func SeedIfNeeded(ctx context.Context, db *sql.DB) error {
	var cnt int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM dishes`).Scan(&cnt); err != nil {
		return err
	}
	if cnt > 0 {
		return nil
	}

	now := time.Now().UnixMilli()
	dishes := seedDishes()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(
		ctx,
		`INSERT INTO dishes (id, name, category, time_text, level, tags_json, price_cent, story, image_url, badge, details_json, created_by_user_id, created_by_name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, d := range dishes {
		tagsJSON, _ := json.Marshal(d.Tags)
		detailsJSON, _ := json.Marshal(d.Details)
		var cUser any
		var cName any
		if d.CreatedBy != nil {
			cUser = d.CreatedBy.UserID
			cName = d.CreatedBy.Name
		} else {
			cUser = nil
			cName = nil
		}
		if _, err := stmt.ExecContext(
			ctx,
			d.ID,
			d.Name,
			string(d.Category),
			d.TimeText,
			string(d.Level),
			string(tagsJSON),
			d.PriceCent,
			d.Story,
			d.ImageURL,
			d.Badge,
			string(detailsJSON),
			cUser,
			cName,
			now,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func seedDishes() []Dish {
	return []Dish{
		{
			ID:        "tomato-egg",
			Name:      "番茄炒蛋",
			Category:  DishCategoryHome,
			TimeText:  "10 分钟",
			Level:     DishLevelEasy,
			Tags:      []string{"下饭", "孩子喜欢", "酸甜"},
			PriceCent: 1800,
			Story:     "酸甜番茄遇上嫩滑鸡蛋，永远的家常顶流。",
			ImageURL:  "https://picsum.photos/seed/tomato-egg/1200/720",
			Badge:     "经典",
			Details: DishDetails{
				Ingredients: []string{"番茄 2 个", "鸡蛋 3 个", "葱花 少许", "盐 适量", "糖 少许", "生抽 少许（可选）"},
				Steps: []string{
					"番茄切块，鸡蛋加少许盐打散。",
					"热锅少油，鸡蛋滑炒至凝固盛出。",
					"少油下番茄炒出汁，按口味加盐/糖，喜欢可加一点生抽提鲜。",
					"倒回鸡蛋翻匀，收汁到喜欢的浓稠度，撒葱花出锅。",
				},
			},
		},
		{
			ID:        "braised-pork",
			Name:      "红烧肉",
			Category:  DishCategoryHome,
			TimeText:  "60 分钟",
			Level:     DishLevelMedium,
			Tags:      []string{"软糯", "浓香", "周末限定"},
			PriceCent: 4800,
			Story:     "慢火收汁，油亮不腻，配米饭能吃两碗。",
			ImageURL:  "https://picsum.photos/seed/braised-pork/1200/720",
			Badge:     "招牌",
			Details: DishDetails{
				Ingredients: []string{"五花肉 500g", "冰糖 20g", "葱姜 适量", "料酒 适量", "生抽 2 勺", "老抽 1/2 勺", "热水 适量"},
				Steps: []string{
					"五花肉切块焯水，冲净浮沫沥干。",
					"小火融化冰糖炒出糖色，倒入肉块上色。",
					"加葱姜和料酒，生抽/老抽调色调味。",
					"加热水没过肉，小火慢炖 40-60 分钟。",
					"开大火收汁，至油亮挂汁即可。",
				},
			},
		},
		{
			ID:        "chicken-soup",
			Name:      "玉米胡萝卜鸡汤",
			Category:  DishCategorySoup,
			TimeText:  "45 分钟",
			Level:     DishLevelEasy,
			Tags:      []string{"暖胃", "清甜", "加班回血"},
			PriceCent: 3600,
			Story:     "清甜不寡淡，喝一口像回到家。",
			ImageURL:  "https://picsum.photos/seed/chicken-soup/1200/720",
			Badge:     "温暖",
			Details: DishDetails{
				Ingredients: []string{"鸡腿/半只鸡 1 份", "玉米 1 根", "胡萝卜 1 根", "姜片 3 片", "盐 适量"},
				Steps: []string{
					"鸡肉焯水去腥，玉米胡萝卜切段。",
					"锅中加足量水，放鸡肉与姜片大火煮开转小火。",
					"加入玉米胡萝卜，小火炖 30-45 分钟。",
					"出锅前加盐调味即可。",
				},
			},
		},
		{
			ID:        "mapo-tofu",
			Name:      "麻婆豆腐",
			Category:  DishCategoryQuick,
			TimeText:  "15 分钟",
			Level:     DishLevelEasy,
			Tags:      []string{"香辣", "快手", "豆腐嫩"},
			PriceCent: 2200,
			Story:     "麻辣鲜香但不呛，轻松做出馆子味。",
			ImageURL:  "https://picsum.photos/seed/mapo-tofu/1200/720",
			Badge:     "快手",
			Details: DishDetails{
				Ingredients: []string{"嫩豆腐 1 盒", "牛肉末/猪肉末 80g", "豆瓣酱 1 勺", "花椒面 少许", "蒜末 葱花 适量", "淀粉水 少许"},
				Steps: []string{
					"豆腐切块焯水 1 分钟（可加少许盐），捞出备用。",
					"炒香肉末，加入豆瓣酱炒出红油。",
					"加水/高汤，放豆腐小火煮 3-5 分钟入味。",
					"淀粉水勾薄芡，撒蒜末葱花花椒面即可。",
				},
			},
		},
		{
			ID:        "fried-rice",
			Name:      "金黄蛋炒饭",
			Category:  DishCategoryQuick,
			TimeText:  "12 分钟",
			Level:     DishLevelEasy,
			Tags:      []string{"剩饭救星", "香气足", "一锅端"},
			PriceCent: 1600,
			Story:     "粒粒分明，锅气一开，幸福感就来了。",
			ImageURL:  "https://picsum.photos/seed/fried-rice/1200/720",
			Badge:     "一锅",
			Details: DishDetails{
				Ingredients: []string{"隔夜米饭 1 碗", "鸡蛋 1-2 个", "火腿/培根（可选）", "葱花 少许", "盐 适量"},
				Steps: []string{
					"米饭提前打散，鸡蛋打匀。",
					"热锅少油，下蛋液，立即加入米饭翻炒裹匀。",
					"按需加入火腿丁等配料炒香。",
					"加盐调味，撒葱花出锅。",
				},
			},
		},
		{
			ID:        "milk-pudding",
			Name:      "桂花牛奶布丁",
			Category:  DishCategorySweet,
			TimeText:  "20 分钟",
			Level:     DishLevelEasy,
			Tags:      []string{"软糯", "治愈", "饭后甜"},
			PriceCent: 2000,
			Story:     "淡淡桂花香，像把温柔装进小碗里。",
			ImageURL:  "https://picsum.photos/seed/milk-pudding/1200/720",
			Badge:     "甜甜",
			Details: DishDetails{
				Ingredients: []string{"牛奶 300ml", "淡奶油 100ml（可选）", "糖 20-30g", "吉利丁片 5g", "桂花蜜 少许"},
				Steps: []string{
					"吉利丁片冷水泡软备用。",
					"牛奶加糖小火加热到温热不沸腾，离火加入吉利丁搅匀融化。",
					"倒入杯中放凉后冷藏 2-3 小时凝固。",
					"食用前淋桂花蜜即可。",
				},
			},
		},
	}
}

