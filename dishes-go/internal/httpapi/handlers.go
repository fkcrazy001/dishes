package httpapi

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fkcrazy001/dishes/dishes-go/internal/realtime"
	"github.com/fkcrazy001/dishes/dishes-go/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

func (a *API) handleRegister(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Account      string  `json:"account"`
		Password     *string `json:"password"`
		PasswordHash *string `json:"passwordHash"`
		Name         string  `json:"name"`
	}
	if err := a.readJSON(r, &body); err != nil {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}

	u, err := a.deps.Store.Register(r.Context(), store.RegisterInput{
		Account:      body.Account,
		Name:         body.Name,
		Password:     body.Password,
		PasswordHash: body.PasswordHash,
	})
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	a.writeOK(w, map[string]any{"user": u})
}

func (a *API) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Account      string  `json:"account"`
		Password     *string `json:"password"`
		PasswordHash *string `json:"passwordHash"`
	}
	if err := a.readJSON(r, &body); err != nil {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}

	u, err := a.deps.Store.Login(r.Context(), store.LoginInput{
		Account:      body.Account,
		Password:     body.Password,
		PasswordHash: body.PasswordHash,
	})
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	token, err := a.signToken(u.ID)
	if err != nil {
		a.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "服务内部错误", nil)
		return
	}
	a.writeOK(w, map[string]any{"accessToken": token, "user": u})
}

func (a *API) handleMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	u, err := a.deps.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	a.writeOK(w, map[string]any{"user": u})
}

func (a *API) handleUsersRank(w http.ResponseWriter, r *http.Request) {
	sort := r.URL.Query().Get("sort")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	items, err := a.deps.Store.ListUsersRank(r.Context(), store.UsersRankInput{Sort: sort, Limit: limit})
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	a.writeOK(w, map[string]any{"items": items})
}

func (a *API) handleListDishes(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	category := r.URL.Query().Get("category")
	scope := r.URL.Query().Get("scope")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	createdByUserID := ""
	if scope == "mine" {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
			return
		}
		userID, err := a.verifyToken(auth)
		if err != nil {
			a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
			return
		}
		createdByUserID = userID
	}

	items, total, err := a.deps.Store.ListDishes(r.Context(), store.ListDishesInput{
		Category:        category,
		Q:               q,
		Page:            page,
		PageSize:        pageSize,
		CreatedByUserID: createdByUserID,
	})
	if err != nil {
		a.writeStoreError(w, err)
		return
	}

	listItems := make([]any, 0, len(items))
	for _, d := range items {
		listItems = append(listItems, map[string]any{
			"id":        d.ID,
			"name":      d.Name,
			"category":  d.Category,
			"timeText":  d.TimeText,
			"level":     d.Level,
			"tags":      d.Tags,
			"priceCent": d.PriceCent,
			"story":     d.Story,
			"imageUrl":  d.ImageURL,
			"badge":     d.Badge,
			"createdBy": d.CreatedBy,
		})
	}

	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	a.writeOK(w, map[string]any{"items": listItems, "page": page, "pageSize": pageSize, "total": total})
}

func (a *API) handleGetDish(w http.ResponseWriter, r *http.Request) {
	dishID := chi.URLParam(r, "dishId")
	if dishID == "" {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}
	d, err := a.deps.Store.GetDishByID(r.Context(), dishID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	a.writeOK(w, map[string]any{"dish": d})
}

func (a *API) handleCreateDish(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	me, err := a.deps.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}

	var body struct {
		Name      string            `json:"name"`
		Category  string            `json:"category"`
		TimeText  string            `json:"timeText"`
		Level     string            `json:"level"`
		Tags      []string          `json:"tags"`
		PriceCent int64             `json:"priceCent"`
		Story     string            `json:"story"`
		ImageURL  string            `json:"imageUrl"`
		Badge     string            `json:"badge"`
		Details   store.DishDetails `json:"details"`
	}
	if err := a.readJSON(r, &body); err != nil {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", err)
		return
	}

	d, err := a.deps.Store.CreateDish(r.Context(), store.CreateDishInput{
		Name:      body.Name,
		Category:  body.Category,
		TimeText:  body.TimeText,
		Level:     body.Level,
		Tags:      body.Tags,
		PriceCent: body.PriceCent,
		Story:     body.Story,
		ImageURL:  body.ImageURL,
		Badge:     body.Badge,
		Details:   body.Details,
		CreatedBy: store.DishCreatedBy{UserID: me.ID, Name: me.Name},
	})
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	a.writeOK(w, map[string]any{"dish": d})
}

func (a *API) handleDeleteDish(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	dishID := chi.URLParam(r, "dishId")
	if dishID == "" {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}
	d, err := a.deps.Store.GetDishByID(r.Context(), dishID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	if d.CreatedBy == nil || d.CreatedBy.UserID != userID {
		a.writeError(w, http.StatusForbidden, "UNAUTHORIZED", "无权限删除该菜谱", nil)
		return
	}
	okDeleted, err := a.deps.Store.DeleteDish(r.Context(), dishID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	if !okDeleted {
		a.writeError(w, http.StatusNotFound, "DISH_NOT_FOUND", "菜谱不存在", map[string]any{"dishId": dishID})
		return
	}
	a.writeOK(w, map[string]any{"dish": map[string]any{"id": dishID}})
}

func (a *API) handleCreateOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	me, err := a.deps.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}

	var body struct {
		Items []struct {
			DishID string `json:"dishId"`
			Qty    int64  `json:"qty"`
		} `json:"items"`
		Note *string `json:"note"`
	}
	if err := a.readJSON(r, &body); err != nil || len(body.Items) == 0 {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}

	items := make([]store.CreateOrderItem, 0, len(body.Items))
	for _, it := range body.Items {
		items = append(items, store.CreateOrderItem{DishID: it.DishID, Qty: it.Qty})
	}

	order, err := a.deps.Store.CreateOrder(r.Context(), store.CreateOrderInput{
		UserID:   userID,
		UserName: me.Name,
		Items:    items,
		Note:     body.Note,
	})
	if err != nil {
		a.writeStoreError(w, err)
		return
	}

	nextMe, _ := a.deps.Store.GetUserByID(r.Context(), userID)
	a.writeOK(w, map[string]any{"order": order, "me": nextMe})

	a.deps.Hub.Publish(realtime.Event{Type: "order.updated", Data: map[string]any{"orderId": order.ID, "status": order.Status, "updatedAt": order.UpdatedAt}})
	a.deps.Hub.Publish(realtime.Event{Type: "order.snapshot", Data: map[string]any{"order": order}})
}

func (a *API) handleListOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "mine"
	}
	status := r.URL.Query().Get("status")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))

	items, total, err := a.deps.Store.ListOrders(r.Context(), store.ListOrdersInput{
		UserID:   userID,
		Scope:    scope,
		Status:   status,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	a.writeOK(w, map[string]any{"items": items, "page": page, "pageSize": pageSize, "total": total})
}

func (a *API) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	orderID := chi.URLParam(r, "orderId")
	if orderID == "" {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}
	order, err := a.deps.Store.GetOrderByID(r.Context(), orderID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	canRead := order.PlacedBy.UserID == userID || (order.AcceptedBy != nil && order.AcceptedBy.UserID == userID)
	if !canRead {
		a.writeError(w, http.StatusForbidden, "UNAUTHORIZED", "无权限查看该订单", nil)
		return
	}
	a.writeOK(w, map[string]any{"order": order})
}

func (a *API) handleAcceptOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	me, err := a.deps.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	orderID := chi.URLParam(r, "orderId")
	if orderID == "" {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}
	order, err := a.deps.Store.AcceptOrder(r.Context(), orderID, store.OrderPerson{UserID: me.ID, Name: me.Name})
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	a.writeOK(w, map[string]any{"order": order})
	a.emitOrderUpdated(order)
}

func (a *API) handleCancelOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	orderID := chi.URLParam(r, "orderId")
	if orderID == "" {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}
	res, err := a.deps.Store.CancelOrder(r.Context(), orderID, userID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	a.writeOK(w, map[string]any{"order": res.Order, "me": res.Me})
	a.emitOrderUpdated(res.Order)
}

func (a *API) handleFinishOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	orderID := chi.URLParam(r, "orderId")
	if orderID == "" {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}

	images, note, err := a.saveMultipartImages(r, "order", orderID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}

	res, err := a.deps.Store.FinishOrder(r.Context(), orderID, userID, images, note)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	a.writeOK(w, map[string]any{"order": res.Order, "me": res.Me})
	a.emitOrderUpdated(res.Order)
}

func (a *API) handleReviewOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	orderID := chi.URLParam(r, "orderId")
	if orderID == "" {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}

	images, note, rating, content, err := a.saveMultipartReview(r, orderID)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	if rating < 1 || rating > 5 || strings.TrimSpace(content) == "" {
		a.writeError(w, http.StatusBadRequest, "VALIDATION_ERROR", "参数错误", nil)
		return
	}

	_ = note
	order, err := a.deps.Store.CreateReview(r.Context(), orderID, userID, rating, content, images)
	if err != nil {
		a.writeStoreError(w, err)
		return
	}
	a.writeOK(w, map[string]any{"order": order})
	a.emitOrderUpdated(order)
}

func (a *API) emitOrderUpdated(order store.Order) {
	a.deps.Hub.Publish(realtime.Event{Type: "order.updated", Data: map[string]any{"orderId": order.ID, "status": order.Status, "updatedAt": order.UpdatedAt}})
	a.deps.Hub.Publish(realtime.Event{Type: "order.snapshot", Data: map[string]any{"order": order}})
}

func (a *API) writeStoreError(w http.ResponseWriter, err error) {
	if ae, ok := store.AsAppError(err); ok {
		code := ae.Code
		details := ae.Details
		msg := mapErrorMessage(code)
		status := http.StatusBadRequest
		if code == "UNAUTHORIZED" {
			status = http.StatusUnauthorized
		}
		if code == "DISH_NOT_FOUND" || code == "ORDER_NOT_FOUND" {
			status = http.StatusNotFound
		}
		a.writeError(w, status, code, msg, details)
		return
	}
	a.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "服务内部错误", nil)
}

func mapErrorMessage(code string) string {
	switch code {
	case "ACCOUNT_EXISTS":
		return "账号已存在"
	case "INVALID_PASSWORD":
		return "密码至少 6 位"
	case "INVALID_CREDENTIALS":
		return "账号或密码错误"
	case "UNAUTHORIZED":
		return "请先登录"
	case "VALIDATION_ERROR":
		return "参数错误"
	case "DISH_NOT_FOUND":
		return "菜谱不存在"
	case "INVALID_QTY":
		return "数量不合法"
	case "INSUFFICIENT_LOVE":
		return "爱心值不足"
	case "ORDER_NOT_FOUND":
		return "订单不存在"
	case "ORDER_INVALID_STATUS":
		return "订单状态不允许该操作"
	case "ORDER_ALREADY_REVIEWED":
		return "订单已评价"
	default:
		return code
	}
}

func (a *API) saveMultipartImages(r *http.Request, kind, orderID string) ([]string, *string, error) {
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		if err == http.ErrNotMultipart {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	note := strings.TrimSpace(r.FormValue("note"))
	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	files := r.MultipartForm.File["images"]
	if len(files) > 3 {
		files = files[:3]
	}
	dir := filepath.Join(a.deps.UploadDir, kind, orderID)
	if err := ensureDir(dir); err != nil {
		return nil, nil, err
	}
	var urls []string
	for _, fh := range files {
		u, err := a.saveOneFile(fh, dir, kind, orderID)
		if err != nil {
			return nil, nil, err
		}
		urls = append(urls, u)
	}
	return urls, notePtr, nil
}

func (a *API) saveMultipartReview(r *http.Request, orderID string) ([]string, *string, int64, string, error) {
	if err := r.ParseMultipartForm(16 << 20); err != nil {
		return nil, nil, 0, "", err
	}
	content := strings.TrimSpace(r.FormValue("content"))
	ratingStr := strings.TrimSpace(r.FormValue("rating"))
	note := strings.TrimSpace(r.FormValue("note"))
	var notePtr *string
	if note != "" {
		notePtr = &note
	}
	rating, _ := strconv.ParseInt(ratingStr, 10, 64)

	files := r.MultipartForm.File["images"]
	if len(files) > 3 {
		files = files[:3]
	}
	dir := filepath.Join(a.deps.UploadDir, "review", orderID)
	if err := ensureDir(dir); err != nil {
		return nil, nil, 0, "", err
	}
	var urls []string
	for _, fh := range files {
		u, err := a.saveOneFile(fh, dir, "review", orderID)
		if err != nil {
			return nil, nil, 0, "", err
		}
		urls = append(urls, u)
	}
	return urls, notePtr, rating, content, nil
}

func (a *API) saveOneFile(fh *multipart.FileHeader, absDir, kind, orderID string) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()
	ext := filepath.Ext(fh.Filename)
	if ext == "" {
		ext = ".jpg"
	}
	name := randomName() + ext
	abs := filepath.Join(absDir, name)
	out, err := openFileCreate(abs)
	if err != nil {
		return "", err
	}
	defer out.Close()
	if _, err := io.Copy(out, io.LimitReader(f, 12<<20)); err != nil {
		return "", err
	}
	return strings.TrimSuffix(a.deps.UploadsURL, "/") + "/" + kind + "/" + orderID + "/" + name, nil
}

func randomName() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return strconv.FormatInt(time.Now().UnixMilli(), 10) + "-" + strings.ToLower(hexEncode(b))
}

func hexEncode(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}

func openFileCreate(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
}

func (a *API) handleOrdersStream(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromContext(r.Context())
	if !ok {
		a.writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "请先登录", nil)
		return
	}
	scope := r.URL.Query().Get("scope")
	if scope == "" {
		scope = "mine"
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		a.writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "服务内部错误", nil)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-transform")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch, cancel := a.deps.Hub.Subscribe(32)
	defer cancel()

	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			_, _ = io.WriteString(w, "event: ping\ndata: "+strconv.FormatInt(time.Now().UnixMilli(), 10)+"\n\n")
			flusher.Flush()
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if evt.Type != "order.updated" {
				continue
			}
			if scope == "mine" {
				m, ok := evt.Data.(map[string]any)
				if !ok {
					continue
				}
				orderID, _ := m["orderId"].(string)
				order, err := a.deps.Store.GetOrderByID(r.Context(), orderID)
				if err != nil {
					continue
				}
				if order.PlacedBy.UserID != userID {
					continue
				}
			}
			payload, _ := json.Marshal(evt)
			_, _ = io.WriteString(w, "data: "+string(payload)+"\n\n")
			flusher.Flush()
		}
	}
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (a *API) handleOrdersWS(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimSpace(r.URL.Query().Get("token"))
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID, err := a.verifyToken(token)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	scope := r.URL.Query().Get("scope")
	if scope != "all" {
		scope = "mine"
	}

	ws, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer ws.Close()

	closed := make(chan struct{})
	go func() {
		defer close(closed)
		for {
			if _, _, err := ws.ReadMessage(); err != nil {
				return
			}
		}
	}()

	page := 1
	pageSize := 50
	items, _, err := a.deps.Store.ListOrders(r.Context(), store.ListOrdersInput{UserID: userID, Scope: scope, Page: page, PageSize: pageSize})
	if err == nil {
		for _, o := range items {
			_ = ws.WriteJSON(realtime.Event{Type: "order.snapshot", Data: map[string]any{"order": o}})
		}
	}

	ch, cancel := a.deps.Hub.Subscribe(128)
	defer cancel()

	for {
		select {
		case <-closed:
			return
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if evt.Type != "order.updated" {
				continue
			}
			m, ok := evt.Data.(map[string]any)
			if !ok {
				continue
			}
			orderID, _ := m["orderId"].(string)
			if orderID == "" {
				continue
			}
			order, err := a.deps.Store.GetOrderByID(r.Context(), orderID)
			if err != nil {
				continue
			}
			if scope == "mine" && order.PlacedBy.UserID != userID {
				continue
			}
			_ = ws.WriteJSON(evt)
			_ = ws.WriteJSON(realtime.Event{Type: "order.snapshot", Data: map[string]any{"order": order}})
		}
	}
}
