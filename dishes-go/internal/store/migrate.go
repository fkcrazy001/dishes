package store

import (
	"context"
	"database/sql"
)

func Migrate(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(
		ctx,
		`
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;

CREATE TABLE IF NOT EXISTS users (
  id TEXT PRIMARY KEY,
  account TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  password_salt TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  password_kind TEXT NOT NULL DEFAULT 'sha256',
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

CREATE TABLE IF NOT EXISTS order_reviews (
  order_id TEXT PRIMARY KEY,
  rating INTEGER NOT NULL,
  content TEXT NOT NULL,
  images_json TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  created_by_user_id TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_dishes_category ON dishes(category);
CREATE INDEX IF NOT EXISTS idx_orders_created_at ON orders(created_at);
CREATE INDEX IF NOT EXISTS idx_orders_placed_by_user_id ON orders(placed_by_user_id);
CREATE INDEX IF NOT EXISTS idx_order_items_order_id ON order_items(order_id);
CREATE INDEX IF NOT EXISTS idx_order_reviews_created_at ON order_reviews(created_at);
`,
	)
	return err
}

