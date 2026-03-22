package store

type OrderStatus string

const (
	OrderStatusPlaced    OrderStatus = "placed"
	OrderStatusAccepted  OrderStatus = "accepted"
	OrderStatusDone      OrderStatus = "done"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type User struct {
	ID       string `json:"id"`
	Account  string `json:"account"`
	Name     string `json:"name"`
	LoveMilli int64 `json:"loveMilli"`
}

type DishCategory string

const (
	DishCategoryHome  DishCategory = "home"
	DishCategorySoup  DishCategory = "soup"
	DishCategorySweet DishCategory = "sweet"
	DishCategoryQuick DishCategory = "quick"
)

type DishLevel string

const (
	DishLevelEasy   DishLevel = "easy"
	DishLevelMedium DishLevel = "medium"
	DishLevelHard   DishLevel = "hard"
)

type DishDetails struct {
	Ingredients []string `json:"ingredients"`
	Steps       []string `json:"steps"`
}

type DishCreatedBy struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
}

type Dish struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Category  DishCategory  `json:"category"`
	TimeText  string        `json:"timeText"`
	Level     DishLevel     `json:"level"`
	Tags      []string      `json:"tags"`
	PriceCent int64         `json:"priceCent"`
	Story     string        `json:"story"`
	ImageURL  string        `json:"imageUrl"`
	Badge     string        `json:"badge"`
	Details   DishDetails   `json:"details"`
	CreatedBy *DishCreatedBy `json:"createdBy,omitempty"`
}

type OrderItem struct {
	DishID    string `json:"dishId"`
	DishName  string `json:"dishName"`
	Qty       int64  `json:"qty"`
	PriceCent int64  `json:"priceCent"`
}

type OrderPerson struct {
	UserID string `json:"userId"`
	Name   string `json:"name"`
}

type OrderReview struct {
	Rating    int64    `json:"rating"`
	Content   string   `json:"content"`
	Images    []string `json:"images"`
	CreatedAt int64    `json:"createdAt"`
}

type Order struct {
	ID         string       `json:"id"`
	CreatedAt  int64        `json:"createdAt"`
	UpdatedAt  int64        `json:"updatedAt"`
	Status     OrderStatus  `json:"status"`
	PlacedBy   OrderPerson  `json:"placedBy"`
	PlacedNote *string      `json:"placedNote,omitempty"`
	AcceptedBy *OrderPerson `json:"acceptedBy,omitempty"`
	FinishedAt *int64       `json:"finishedAt,omitempty"`
	FinishImages []string   `json:"finishImages,omitempty"`
	FinishNote *string      `json:"finishNote,omitempty"`
	Review     *OrderReview `json:"review,omitempty"`
	Items      []OrderItem  `json:"items"`
	TotalCent  int64        `json:"totalCent"`
}

