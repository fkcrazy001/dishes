package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"
)

type Store struct {
	db *sql.DB
}

func New(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Register(ctx context.Context, input RegisterInput) (User, error) {
	account := strings.TrimSpace(input.Account)
	name := strings.TrimSpace(input.Name)
	if account == "" || name == "" {
		return User{}, Err("VALIDATION_ERROR", nil)
	}
	secret, err := normalizePasswordSecret(input.Password, input.PasswordHash)
	if err != nil {
		return User{}, err
	}

	var exists int
	_ = s.db.QueryRowContext(ctx, `SELECT 1 FROM users WHERE account = ? LIMIT 1`, account).Scan(&exists)
	if exists == 1 {
		return User{}, Err("ACCOUNT_EXISTS", nil)
	}

	userID := createUserID()
	salt := randomHex(16)
	hash := scryptHex(secret, salt)
	createdAt := time.Now().UnixMilli()
	loveMilli := int64(100000)

	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO users (id, account, name, password_salt, password_hash, password_kind, created_at, love_milli) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		userID,
		account,
		name,
		salt,
		hash,
		"sha256",
		createdAt,
		loveMilli,
	); err != nil {
		return User{}, err
	}

	return User{ID: userID, Account: account, Name: name, LoveMilli: loveMilli}, nil
}

func (s *Store) Login(ctx context.Context, input LoginInput) (User, error) {
	account := strings.TrimSpace(input.Account)
	if account == "" {
		return User{}, Err("VALIDATION_ERROR", nil)
	}
	secret, err := normalizePasswordSecret(input.Password, input.PasswordHash)
	if err != nil {
		return User{}, err
	}

	var row struct {
		ID        string
		Account   string
		Name      string
		Salt      string
		Hash      string
		Kind      string
		LoveMilli int64
	}
	err = s.db.QueryRowContext(
		ctx,
		`SELECT id, account, name, password_salt, password_hash, password_kind, love_milli FROM users WHERE account = ? LIMIT 1`,
		account,
	).Scan(&row.ID, &row.Account, &row.Name, &row.Salt, &row.Hash, &row.Kind, &row.LoveMilli)
	if err == sql.ErrNoRows {
		return User{}, Err("INVALID_CREDENTIALS", nil)
	}
	if err != nil {
		return User{}, err
	}

	if scryptHex(secret, row.Salt) != row.Hash {
		return User{}, Err("INVALID_CREDENTIALS", nil)
	}

	return User{ID: row.ID, Account: row.Account, Name: row.Name, LoveMilli: row.LoveMilli}, nil
}

func (s *Store) GetUserByID(ctx context.Context, userID string) (User, error) {
	var u User
	err := s.db.QueryRowContext(ctx, `SELECT id, account, name, love_milli FROM users WHERE id = ? LIMIT 1`, userID).Scan(
		&u.ID,
		&u.Account,
		&u.Name,
		&u.LoveMilli,
	)
	if err == sql.ErrNoRows {
		return User{}, Err("UNAUTHORIZED", nil)
	}
	return u, err
}

type UsersRankInput struct {
	Sort  string
	Limit int
}

type UserLoveRankItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	LoveMilli int64  `json:"loveMilli"`
}

func (s *Store) ListUsersRank(ctx context.Context, input UsersRankInput) ([]UserLoveRankItem, error) {
	if input.Sort != "" && input.Sort != "loveMilli_desc" {
		return nil, Err("VALIDATION_ERROR", map[string]any{"field": "sort", "allowed": []string{"loveMilli_desc"}})
	}
	limit := input.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	rows, err := s.db.QueryContext(ctx, `SELECT id, name, love_milli FROM users ORDER BY love_milli DESC, created_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []UserLoveRankItem
	for rows.Next() {
		var it UserLoveRankItem
		if err := rows.Scan(&it.ID, &it.Name, &it.LoveMilli); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

type ListDishesInput struct {
	Category        string
	Q               string
	Page            int
	PageSize        int
	CreatedByUserID string
}

func (s *Store) ListDishes(ctx context.Context, input ListDishesInput) ([]Dish, int64, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := (page - 1) * pageSize

	var where []string
	var params []any

	if input.CreatedByUserID != "" {
		where = append(where, `created_by_user_id = ?`)
		params = append(params, input.CreatedByUserID)
	}
	if input.Category != "" {
		where = append(where, `category = ?`)
		params = append(params, input.Category)
	}
	kw := strings.TrimSpace(input.Q)
	if kw != "" {
		where = append(where, `(name LIKE ? OR tags_json LIKE ?)`)
		like := "%" + kw + "%"
		params = append(params, like, like)
	}

	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM dishes `+whereSQL, params...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := `SELECT id, name, category, time_text, level, tags_json, price_cent, story, image_url, badge, details_json, created_by_user_id, created_by_name
FROM dishes ` + whereSQL + ` ORDER BY rowid ASC LIMIT ? OFFSET ?`
	params2 := append(params, pageSize, offset)

	rows, err := s.db.QueryContext(ctx, q, params2...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []Dish
	for rows.Next() {
		var (
			id, name, category, timeText, level, tagsJSON, story, imageURL, badge, detailsJSON sql.NullString
			priceCent                                                                          int64
			createdByUserID, createdByName                                                     sql.NullString
		)
		if err := rows.Scan(
			&id,
			&name,
			&category,
			&timeText,
			&level,
			&tagsJSON,
			&priceCent,
			&story,
			&imageURL,
			&badge,
			&detailsJSON,
			&createdByUserID,
			&createdByName,
		); err != nil {
			return nil, 0, err
		}
		var tags []string
		_ = json.Unmarshal([]byte(tagsJSON.String), &tags)
		var details DishDetails
		_ = json.Unmarshal([]byte(detailsJSON.String), &details)

		var createdBy *DishCreatedBy
		if createdByUserID.Valid && createdByName.Valid {
			createdBy = &DishCreatedBy{UserID: createdByUserID.String, Name: createdByName.String}
		}

		items = append(items, Dish{
			ID:        id.String,
			Name:      name.String,
			Category:  DishCategory(category.String),
			TimeText:  timeText.String,
			Level:     DishLevel(level.String),
			Tags:      tags,
			PriceCent: priceCent,
			Story:     story.String,
			ImageURL:  imageURL.String,
			Badge:     badge.String,
			Details:   details,
			CreatedBy: createdBy,
		})
	}
	return items, total, rows.Err()
}

func (s *Store) GetDishByID(ctx context.Context, dishID string) (Dish, error) {
	var (
		id, name, category, timeText, level, tagsJSON, story, imageURL, badge, detailsJSON sql.NullString
		priceCent                                                                          int64
		createdByUserID, createdByName                                                     sql.NullString
	)
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, name, category, time_text, level, tags_json, price_cent, story, image_url, badge, details_json, created_by_user_id, created_by_name
FROM dishes WHERE id = ? LIMIT 1`,
		dishID,
	).Scan(
		&id,
		&name,
		&category,
		&timeText,
		&level,
		&tagsJSON,
		&priceCent,
		&story,
		&imageURL,
		&badge,
		&detailsJSON,
		&createdByUserID,
		&createdByName,
	)
	if err == sql.ErrNoRows {
		return Dish{}, Err("DISH_NOT_FOUND", map[string]any{"dishId": dishID})
	}
	if err != nil {
		return Dish{}, err
	}
	var tags []string
	_ = json.Unmarshal([]byte(tagsJSON.String), &tags)
	var details DishDetails
	_ = json.Unmarshal([]byte(detailsJSON.String), &details)
	var createdBy *DishCreatedBy
	if createdByUserID.Valid && createdByName.Valid {
		createdBy = &DishCreatedBy{UserID: createdByUserID.String, Name: createdByName.String}
	}
	return Dish{
		ID:        id.String,
		Name:      name.String,
		Category:  DishCategory(category.String),
		TimeText:  timeText.String,
		Level:     DishLevel(level.String),
		Tags:      tags,
		PriceCent: priceCent,
		Story:     story.String,
		ImageURL:  imageURL.String,
		Badge:     badge.String,
		Details:   details,
		CreatedBy: createdBy,
	}, nil
}

type CreateDishInput struct {
	Name      string
	Category  string
	TimeText  string
	Level     string
	Tags      []string
	PriceCent int64
	Story     string
	ImageURL  string
	Badge     string
	Details   DishDetails
	CreatedBy DishCreatedBy
}

func (s *Store) CreateDish(ctx context.Context, input CreateDishInput) (Dish, error) {
	if strings.TrimSpace(input.Name) == "" ||
		strings.TrimSpace(input.Category) == "" ||
		strings.TrimSpace(input.TimeText) == "" ||
		strings.TrimSpace(input.Level) == "" ||
		input.PriceCent <= 0 ||
		strings.TrimSpace(input.Story) == "" ||
		len(input.Details.Ingredients) == 0 ||
		len(input.Details.Steps) == 0 {
		return Dish{}, Err("VALIDATION_ERROR", nil)
	}

	id := normalizeDishID(input.Name)
	now := time.Now().UnixMilli()
	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}
	imageURL := strings.TrimSpace(input.ImageURL)
	if imageURL == "" {
		imageURL = "https://picsum.photos/seed/" + id + "/1200/720"
	}
	badge := strings.TrimSpace(input.Badge)
	if badge == "" {
		badge = "自制"
	}

	tagsJSON, _ := json.Marshal(tags)
	detailsJSON, _ := json.Marshal(input.Details)
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO dishes (id, name, category, time_text, level, tags_json, price_cent, story, image_url, badge, details_json, created_by_user_id, created_by_name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		input.Name,
		input.Category,
		input.TimeText,
		input.Level,
		string(tagsJSON),
		input.PriceCent,
		input.Story,
		imageURL,
		badge,
		string(detailsJSON),
		input.CreatedBy.UserID,
		input.CreatedBy.Name,
		now,
	)
	if err != nil {
		return Dish{}, err
	}
	return Dish{
		ID:        id,
		Name:      input.Name,
		Category:  DishCategory(input.Category),
		TimeText:  input.TimeText,
		Level:     DishLevel(input.Level),
		Tags:      tags,
		PriceCent: input.PriceCent,
		Story:     input.Story,
		ImageURL:  imageURL,
		Badge:     badge,
		Details:   input.Details,
		CreatedBy: &DishCreatedBy{UserID: input.CreatedBy.UserID, Name: input.CreatedBy.Name},
	}, nil
}

func (s *Store) DeleteDish(ctx context.Context, dishID string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM dishes WHERE id = ?`, dishID)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

type CreateOrderInput struct {
	UserID   string
	UserName string
	Items    []CreateOrderItem
	Note     *string
}

type CreateOrderItem struct {
	DishID string
	Qty    int64
}

func (s *Store) CreateOrder(ctx context.Context, input CreateOrderInput) (Order, error) {
	if input.UserID == "" || input.UserName == "" || len(input.Items) == 0 {
		return Order{}, Err("VALIDATION_ERROR", nil)
	}

	now := time.Now().UnixMilli()
	orderID := createOrderID()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()

	var lines []OrderItem
	for _, it := range input.Items {
		if it.DishID == "" || it.Qty <= 0 {
			return Order{}, Err("INVALID_QTY", map[string]any{"dishId": it.DishID})
		}
		dish, err := s.getDishByIDTx(ctx, tx, it.DishID)
		if err != nil {
			return Order{}, err
		}
		lines = append(lines, OrderItem{DishID: dish.ID, DishName: dish.Name, Qty: it.Qty, PriceCent: dish.PriceCent})
	}
	var totalCent int64
	for _, l := range lines {
		totalCent += l.PriceCent * l.Qty
	}

	me, err := s.getUserByIDTx(ctx, tx, input.UserID)
	if err != nil {
		return Order{}, err
	}
	if me.LoveMilli < totalCent {
		return Order{}, Err("INSUFFICIENT_LOVE", map[string]any{"loveMilli": me.LoveMilli, "required": totalCent})
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO orders (id, created_at, updated_at, status, placed_by_user_id, placed_by_name, placed_note, total_cent) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		orderID,
		now,
		now,
		string(OrderStatusPlaced),
		input.UserID,
		input.UserName,
		nullString(input.Note),
		totalCent,
	)
	if err != nil {
		return Order{}, err
	}

	for _, l := range lines {
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_items (order_id, dish_id, dish_name, qty, price_cent) VALUES (?, ?, ?, ?, ?)`,
			orderID, l.DishID, l.DishName, l.Qty, l.PriceCent); err != nil {
			return Order{}, err
		}
	}

	if _, err := tx.ExecContext(ctx, `UPDATE users SET love_milli = love_milli - ? WHERE id = ?`, totalCent, input.UserID); err != nil {
		return Order{}, err
	}

	if err := tx.Commit(); err != nil {
		return Order{}, err
	}

	var placedNote *string
	if input.Note != nil && strings.TrimSpace(*input.Note) != "" {
		placedNote = input.Note
	}
	return Order{
		ID:         orderID,
		CreatedAt:  now,
		UpdatedAt:  now,
		Status:     OrderStatusPlaced,
		PlacedBy:   OrderPerson{UserID: input.UserID, Name: input.UserName},
		PlacedNote: placedNote,
		Items:      lines,
		TotalCent:  totalCent,
	}, nil
}

type ListOrdersInput struct {
	UserID   string
	Scope    string
	Status   string
	Page     int
	PageSize int
}

func (s *Store) ListOrders(ctx context.Context, input ListOrdersInput) ([]Order, int64, error) {
	page := input.Page
	if page <= 0 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	offset := (page - 1) * pageSize

	var where []string
	var params []any
	if input.Scope == "mine" {
		where = append(where, `placed_by_user_id = ?`)
		params = append(params, input.UserID)
	}
	if input.Status != "" {
		where = append(where, `status = ?`)
		params = append(params, input.Status)
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = "WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM orders `+whereSQL, params...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, created_at, updated_at, status, placed_by_user_id, placed_by_name, placed_note, accepted_by_user_id, accepted_by_name, finished_at, finish_images_json, finish_note, total_cent
FROM orders `+whereSQL+` ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		append(params, pageSize, offset)...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Order
	for rows.Next() {
		var (
			id, status, placedByUserID, placedByName string
			createdAt, updatedAt, totalCent          int64
			placedNote                               sql.NullString
			acceptedByUserID, acceptedByName         sql.NullString
			finishedAt                               sql.NullInt64
			finishImagesJSON                         sql.NullString
			finishNote                               sql.NullString
		)
		if err := rows.Scan(
			&id,
			&createdAt,
			&updatedAt,
			&status,
			&placedByUserID,
			&placedByName,
			&placedNote,
			&acceptedByUserID,
			&acceptedByName,
			&finishedAt,
			&finishImagesJSON,
			&finishNote,
			&totalCent,
		); err != nil {
			return nil, 0, err
		}
		items, err := s.getOrderItems(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		review, err := s.getReviewByOrderID(ctx, id)
		if err != nil {
			return nil, 0, err
		}
		var placedNotePtr *string
		if placedNote.Valid {
			v := placedNote.String
			placedNotePtr = &v
		}
		var acceptedBy *OrderPerson
		if acceptedByUserID.Valid && acceptedByName.Valid {
			acceptedBy = &OrderPerson{UserID: acceptedByUserID.String, Name: acceptedByName.String}
		}
		var finishedAtPtr *int64
		if finishedAt.Valid {
			v := finishedAt.Int64
			finishedAtPtr = &v
		}
		var finishImages []string
		if finishImagesJSON.Valid {
			_ = json.Unmarshal([]byte(finishImagesJSON.String), &finishImages)
		}
		var finishNotePtr *string
		if finishNote.Valid {
			v := finishNote.String
			finishNotePtr = &v
		}
		out = append(out, Order{
			ID:           id,
			CreatedAt:    createdAt,
			UpdatedAt:    updatedAt,
			Status:       OrderStatus(status),
			PlacedBy:     OrderPerson{UserID: placedByUserID, Name: placedByName},
			PlacedNote:   placedNotePtr,
			AcceptedBy:   acceptedBy,
			FinishedAt:   finishedAtPtr,
			FinishImages: finishImages,
			FinishNote:   finishNotePtr,
			Review:       review,
			Items:        items,
			TotalCent:    totalCent,
		})
	}
	return out, total, rows.Err()
}

func (s *Store) GetOrderByID(ctx context.Context, orderID string) (Order, error) {
	var (
		id, status, placedByUserID, placedByName string
		createdAt, updatedAt, totalCent          int64
		placedNote                               sql.NullString
		acceptedByUserID, acceptedByName         sql.NullString
		finishedAt                               sql.NullInt64
		finishImagesJSON                         sql.NullString
		finishNote                               sql.NullString
	)
	err := s.db.QueryRowContext(
		ctx,
		`SELECT id, created_at, updated_at, status, placed_by_user_id, placed_by_name, placed_note, accepted_by_user_id, accepted_by_name, finished_at, finish_images_json, finish_note, total_cent
FROM orders WHERE id = ? LIMIT 1`,
		orderID,
	).Scan(
		&id,
		&createdAt,
		&updatedAt,
		&status,
		&placedByUserID,
		&placedByName,
		&placedNote,
		&acceptedByUserID,
		&acceptedByName,
		&finishedAt,
		&finishImagesJSON,
		&finishNote,
		&totalCent,
	)
	if err == sql.ErrNoRows {
		return Order{}, Err("ORDER_NOT_FOUND", map[string]any{"orderId": orderID})
	}
	if err != nil {
		return Order{}, err
	}
	items, err := s.getOrderItems(ctx, id)
	if err != nil {
		return Order{}, err
	}
	review, err := s.getReviewByOrderID(ctx, id)
	if err != nil {
		return Order{}, err
	}
	var placedNotePtr *string
	if placedNote.Valid {
		v := placedNote.String
		placedNotePtr = &v
	}
	var acceptedBy *OrderPerson
	if acceptedByUserID.Valid && acceptedByName.Valid {
		acceptedBy = &OrderPerson{UserID: acceptedByUserID.String, Name: acceptedByName.String}
	}
	var finishedAtPtr *int64
	if finishedAt.Valid {
		v := finishedAt.Int64
		finishedAtPtr = &v
	}
	var finishImages []string
	if finishImagesJSON.Valid {
		_ = json.Unmarshal([]byte(finishImagesJSON.String), &finishImages)
	}
	var finishNotePtr *string
	if finishNote.Valid {
		v := finishNote.String
		finishNotePtr = &v
	}
	return Order{
		ID:           id,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		Status:       OrderStatus(status),
		PlacedBy:     OrderPerson{UserID: placedByUserID, Name: placedByName},
		PlacedNote:   placedNotePtr,
		AcceptedBy:   acceptedBy,
		FinishedAt:   finishedAtPtr,
		FinishImages: finishImages,
		FinishNote:   finishNotePtr,
		Review:       review,
		Items:        items,
		TotalCent:    totalCent,
	}, nil
}

type CancelOrderResult struct {
	Order Order
	Me    User
}

func (s *Store) CancelOrder(ctx context.Context, orderID, userID string) (CancelOrderResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return CancelOrderResult{}, err
	}
	defer tx.Rollback()

	order, err := s.getOrderByIDTx(ctx, tx, orderID)
	if err != nil {
		return CancelOrderResult{}, err
	}
	if order.PlacedBy.UserID != userID {
		return CancelOrderResult{}, Err("UNAUTHORIZED", nil)
	}
	if order.Status != OrderStatusPlaced {
		return CancelOrderResult{}, Err("ORDER_INVALID_STATUS", map[string]any{"status": order.Status})
	}
	nextUpdatedAt := time.Now().UnixMilli()
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET status = ?, updated_at = ? WHERE id = ?`, string(OrderStatusCancelled), nextUpdatedAt, orderID); err != nil {
		return CancelOrderResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET love_milli = love_milli + ? WHERE id = ?`, order.TotalCent, userID); err != nil {
		return CancelOrderResult{}, err
	}
	me, err := s.getUserByIDTx(ctx, tx, userID)
	if err != nil {
		return CancelOrderResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return CancelOrderResult{}, err
	}
	order.Status = OrderStatusCancelled
	order.UpdatedAt = nextUpdatedAt
	return CancelOrderResult{Order: order, Me: me}, nil
}

func (s *Store) AcceptOrder(ctx context.Context, orderID string, acceptedBy OrderPerson) (Order, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()

	order, err := s.getOrderByIDTx(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if order.Status != OrderStatusPlaced {
		return Order{}, Err("ORDER_INVALID_STATUS", map[string]any{"status": order.Status})
	}
	nextUpdatedAt := time.Now().UnixMilli()
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE orders SET status = ?, updated_at = ?, accepted_by_user_id = ?, accepted_by_name = ? WHERE id = ?`,
		string(OrderStatusAccepted),
		nextUpdatedAt,
		acceptedBy.UserID,
		acceptedBy.Name,
		orderID,
	); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	order.Status = OrderStatusAccepted
	order.UpdatedAt = nextUpdatedAt
	order.AcceptedBy = &acceptedBy
	return order, nil
}

type FinishOrderResult struct {
	Order Order
	Me    User
}

func (s *Store) FinishOrder(ctx context.Context, orderID, finishedByUserID string, images []string, note *string) (FinishOrderResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return FinishOrderResult{}, err
	}
	defer tx.Rollback()

	order, err := s.getOrderByIDTx(ctx, tx, orderID)
	if err != nil {
		return FinishOrderResult{}, err
	}
	if order.Status != OrderStatusAccepted {
		return FinishOrderResult{}, Err("ORDER_INVALID_STATUS", map[string]any{"status": order.Status})
	}
	if order.AcceptedBy == nil || order.AcceptedBy.UserID == "" {
		return FinishOrderResult{}, Err("ORDER_INVALID_STATUS", map[string]any{"status": order.Status})
	}
	if order.AcceptedBy.UserID != finishedByUserID {
		return FinishOrderResult{}, Err("UNAUTHORIZED", nil)
	}

	nextUpdatedAt := time.Now().UnixMilli()
	finishedAt := time.Now().UnixMilli()
	imagesJSON, _ := json.Marshal(images)
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE orders SET status = ?, updated_at = ?, finished_at = ?, finish_images_json = ?, finish_note = ? WHERE id = ?`,
		string(OrderStatusDone),
		nextUpdatedAt,
		finishedAt,
		string(imagesJSON),
		nullString(note),
		orderID,
	); err != nil {
		return FinishOrderResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE users SET love_milli = love_milli + ? WHERE id = ?`, order.TotalCent, order.AcceptedBy.UserID); err != nil {
		return FinishOrderResult{}, err
	}
	me, err := s.getUserByIDTx(ctx, tx, order.AcceptedBy.UserID)
	if err != nil {
		return FinishOrderResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return FinishOrderResult{}, err
	}

	order.Status = OrderStatusDone
	order.UpdatedAt = nextUpdatedAt
	order.FinishedAt = &finishedAt
	order.FinishImages = images
	order.FinishNote = note
	return FinishOrderResult{Order: order, Me: me}, nil
}

func (s *Store) CreateReview(ctx context.Context, orderID, userID string, rating int64, content string, images []string) (Order, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer tx.Rollback()

	order, err := s.getOrderByIDTx(ctx, tx, orderID)
	if err != nil {
		return Order{}, err
	}
	if order.PlacedBy.UserID != userID {
		return Order{}, Err("UNAUTHORIZED", nil)
	}
	if order.Status != OrderStatusDone {
		return Order{}, Err("ORDER_INVALID_STATUS", map[string]any{"status": order.Status})
	}
	var exists int
	_ = tx.QueryRowContext(ctx, `SELECT 1 FROM order_reviews WHERE order_id = ? LIMIT 1`, orderID).Scan(&exists)
	if exists == 1 {
		return Order{}, Err("ORDER_ALREADY_REVIEWED", nil)
	}
	createdAt := time.Now().UnixMilli()
	imagesJSON, _ := json.Marshal(images)
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO order_reviews (order_id, rating, content, images_json, created_at, created_by_user_id) VALUES (?, ?, ?, ?, ?, ?)`,
		orderID,
		rating,
		content,
		string(imagesJSON),
		createdAt,
		userID,
	); err != nil {
		return Order{}, err
	}
	nextUpdatedAt := time.Now().UnixMilli()
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET updated_at = ? WHERE id = ?`, nextUpdatedAt, orderID); err != nil {
		return Order{}, err
	}
	if err := tx.Commit(); err != nil {
		return Order{}, err
	}
	order.UpdatedAt = nextUpdatedAt
	order.Review = &OrderReview{Rating: rating, Content: content, Images: images, CreatedAt: createdAt}
	return order, nil
}

type RegisterInput struct {
	Account      string
	Name         string
	Password     *string
	PasswordHash *string
}

type LoginInput struct {
	Account      string
	Password     *string
	PasswordHash *string
}

func normalizePasswordSecret(password *string, passwordHash *string) (string, error) {
	if passwordHash != nil {
		v := strings.ToLower(strings.TrimSpace(*passwordHash))
		if v != "" && regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(v) {
			return v, nil
		}
	}
	if password != nil {
		p := strings.TrimSpace(*password)
		if len(p) < 6 {
			return "", Err("INVALID_PASSWORD", nil)
		}
		sum := sha256.Sum256([]byte(p))
		return hex.EncodeToString(sum[:]), nil
	}
	return "", Err("VALIDATION_ERROR", map[string]any{"field": "passwordHash", "reason": "missing"})
}

func scryptHex(secret string, saltHex string) string {
	salt, _ := hex.DecodeString(saltHex)
	key, _ := scrypt.Key([]byte(secret), salt, 16384, 8, 1, 32)
	return hex.EncodeToString(key)
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func createUserID() string { return "u_" + randomHex(6) }

func createOrderID() string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, 8)
	for i := 0; i < 8; i++ {
		out[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(out)
}

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func normalizeDishID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonWord.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "dish"
	}
	return s + "-" + randomHex(2)
}

func nullString(v *string) any {
	if v == nil {
		return nil
	}
	s := strings.TrimSpace(*v)
	if s == "" {
		return nil
	}
	return s
}

func (s *Store) getOrderItems(ctx context.Context, orderID string) ([]OrderItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT dish_id, dish_name, qty, price_cent FROM order_items WHERE order_id = ? ORDER BY id ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OrderItem
	for rows.Next() {
		var it OrderItem
		if err := rows.Scan(&it.DishID, &it.DishName, &it.Qty, &it.PriceCent); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) getReviewByOrderID(ctx context.Context, orderID string) (*OrderReview, error) {
	var rating int64
	var content string
	var imagesJSON string
	var createdAt int64
	err := s.db.QueryRowContext(ctx, `SELECT rating, content, images_json, created_at FROM order_reviews WHERE order_id = ? LIMIT 1`, orderID).Scan(
		&rating,
		&content,
		&imagesJSON,
		&createdAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var images []string
	_ = json.Unmarshal([]byte(imagesJSON), &images)
	return &OrderReview{Rating: rating, Content: content, Images: images, CreatedAt: createdAt}, nil
}

func (s *Store) getUserByIDTx(ctx context.Context, tx *sql.Tx, userID string) (User, error) {
	var u User
	err := tx.QueryRowContext(ctx, `SELECT id, account, name, love_milli FROM users WHERE id = ? LIMIT 1`, userID).Scan(
		&u.ID,
		&u.Account,
		&u.Name,
		&u.LoveMilli,
	)
	if err == sql.ErrNoRows {
		return User{}, Err("UNAUTHORIZED", nil)
	}
	return u, err
}

func (s *Store) getDishByIDTx(ctx context.Context, tx *sql.Tx, dishID string) (Dish, error) {
	var (
		id, name, category, timeText, level, tagsJSON, story, imageURL, badge, detailsJSON string
		priceCent                                                                          int64
	)
	err := tx.QueryRowContext(
		ctx,
		`SELECT id, name, category, time_text, level, tags_json, price_cent, story, image_url, badge, details_json FROM dishes WHERE id = ? LIMIT 1`,
		dishID,
	).Scan(
		&id,
		&name,
		&category,
		&timeText,
		&level,
		&tagsJSON,
		&priceCent,
		&story,
		&imageURL,
		&badge,
		&detailsJSON,
	)
	if err == sql.ErrNoRows {
		return Dish{}, Err("DISH_NOT_FOUND", map[string]any{"dishId": dishID})
	}
	if err != nil {
		return Dish{}, err
	}
	var tags []string
	_ = json.Unmarshal([]byte(tagsJSON), &tags)
	var details DishDetails
	_ = json.Unmarshal([]byte(detailsJSON), &details)
	return Dish{
		ID:        id,
		Name:      name,
		Category:  DishCategory(category),
		TimeText:  timeText,
		Level:     DishLevel(level),
		Tags:      tags,
		PriceCent: priceCent,
		Story:     story,
		ImageURL:  imageURL,
		Badge:     badge,
		Details:   details,
	}, nil
}

func (s *Store) getOrderByIDTx(ctx context.Context, tx *sql.Tx, orderID string) (Order, error) {
	var (
		id, status, placedByUserID, placedByName string
		createdAt, updatedAt, totalCent          int64
		placedNote                               sql.NullString
		acceptedByUserID, acceptedByName         sql.NullString
		finishedAt                               sql.NullInt64
		finishImagesJSON                         sql.NullString
		finishNote                               sql.NullString
	)
	err := tx.QueryRowContext(
		ctx,
		`SELECT id, created_at, updated_at, status, placed_by_user_id, placed_by_name, placed_note, accepted_by_user_id, accepted_by_name, finished_at, finish_images_json, finish_note, total_cent
FROM orders WHERE id = ? LIMIT 1`,
		orderID,
	).Scan(
		&id,
		&createdAt,
		&updatedAt,
		&status,
		&placedByUserID,
		&placedByName,
		&placedNote,
		&acceptedByUserID,
		&acceptedByName,
		&finishedAt,
		&finishImagesJSON,
		&finishNote,
		&totalCent,
	)
	if err == sql.ErrNoRows {
		return Order{}, Err("ORDER_NOT_FOUND", map[string]any{"orderId": orderID})
	}
	if err != nil {
		return Order{}, err
	}

	itemRows, err := tx.QueryContext(ctx, `SELECT dish_id, dish_name, qty, price_cent FROM order_items WHERE order_id = ? ORDER BY id ASC`, id)
	if err != nil {
		return Order{}, err
	}
	defer itemRows.Close()
	var items []OrderItem
	for itemRows.Next() {
		var it OrderItem
		if err := itemRows.Scan(&it.DishID, &it.DishName, &it.Qty, &it.PriceCent); err != nil {
			return Order{}, err
		}
		items = append(items, it)
	}
	if err := itemRows.Err(); err != nil {
		return Order{}, err
	}

	var placedNotePtr *string
	if placedNote.Valid {
		v := placedNote.String
		placedNotePtr = &v
	}
	var acceptedBy *OrderPerson
	if acceptedByUserID.Valid && acceptedByName.Valid {
		acceptedBy = &OrderPerson{UserID: acceptedByUserID.String, Name: acceptedByName.String}
	}
	var finishedAtPtr *int64
	if finishedAt.Valid {
		v := finishedAt.Int64
		finishedAtPtr = &v
	}
	var finishImages []string
	if finishImagesJSON.Valid {
		_ = json.Unmarshal([]byte(finishImagesJSON.String), &finishImages)
	}
	var finishNotePtr *string
	if finishNote.Valid {
		v := finishNote.String
		finishNotePtr = &v
	}

	return Order{
		ID:           id,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		Status:       OrderStatus(status),
		PlacedBy:     OrderPerson{UserID: placedByUserID, Name: placedByName},
		PlacedNote:   placedNotePtr,
		AcceptedBy:   acceptedBy,
		FinishedAt:   finishedAtPtr,
		FinishImages: finishImages,
		FinishNote:   finishNotePtr,
		Items:        items,
		TotalCent:    totalCent,
	}, nil
}
