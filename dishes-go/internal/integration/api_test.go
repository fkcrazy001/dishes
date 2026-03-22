package integration

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fkcrazy001/dishes/dishes-go/internal/app"
	"github.com/gorilla/websocket"
)

type apiClient struct {
	baseURL string
	c       *http.Client
}

func newTestServer(t *testing.T) (*apiClient, func()) {
	t.Helper()

	tmp := t.TempDir()
	cfg := app.Config{
		Host:      "127.0.0.1",
		Port:      "0",
		JWTSecret: "test-secret",
		DBFile:    filepath.Join(tmp, "data", "db.sqlite"),
		UploadDir: filepath.Join(tmp, "data", "uploads"),
	}
	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}

	s := httptest.NewServer(a.Router())
	client := &apiClient{
		baseURL: s.URL,
		c: &http.Client{
			Timeout: 3 * time.Second,
		},
	}
	return client, func() {
		s.Close()
	}
}

func (c *apiClient) doJSON(t *testing.T, method, path string, token string, body any) (int, map[string]any) {
	t.Helper()

	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, rdr)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := c.c.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()

	raw, _ := io.ReadAll(res.Body)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal(%s): %v", string(raw), err)
	}
	return res.StatusCode, out
}

func (c *apiClient) doMultipart(t *testing.T, method, path, token string, fields map[string]string, files map[string][]byte) (int, map[string]any) {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	for name, content := range files {
		fw, err := w.CreateFormFile("images", name)
		if err != nil {
			t.Fatalf("CreateFormFile: %v", err)
		}
		if _, err := fw.Write(content); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}
	_ = w.Close()

	req, err := http.NewRequest(method, c.baseURL+path, &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	res, err := c.c.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal(%s): %v", string(raw), err)
	}
	return res.StatusCode, out
}

func mustData(t *testing.T, res map[string]any) map[string]any {
	t.Helper()
	if s, ok := res["success"].(bool); !ok || !s {
		t.Fatalf("expected success true, got %v", res)
	}
	data, ok := res["data"].(map[string]any)
	if !ok {
		t.Fatalf("missing data: %v", res)
	}
	return data
}

func TestAuthAndRank(t *testing.T) {
	c, closeFn := newTestServer(t)
	defer closeFn()

	account := "138" + strconv.FormatInt(time.Now().UnixNano()%100000000, 10)
	password := "123456"
	name := "小熊"

	st, reg := c.doJSON(t, http.MethodPost, "/api/auth/register", "", map[string]any{
		"account":  account,
		"password": password,
		"name":     name,
	})
	if st != 200 {
		t.Fatalf("register status=%d body=%v", st, reg)
	}
	user := mustData(t, reg)["user"].(map[string]any)
	if user["loveMilli"].(float64) != 100000 {
		t.Fatalf("unexpected loveMilli: %v", user)
	}

	st, login := c.doJSON(t, http.MethodPost, "/api/auth/login", "", map[string]any{
		"account":  account,
		"password": password,
	})
	if st != 200 {
		t.Fatalf("login status=%d body=%v", st, login)
	}
	token := mustData(t, login)["accessToken"].(string)

	st, me := c.doJSON(t, http.MethodGet, "/api/me", token, nil)
	if st != 200 {
		t.Fatalf("me status=%d body=%v", st, me)
	}

	st, rank := c.doJSON(t, http.MethodGet, "/api/users?sort=loveMilli_desc&limit=50", token, nil)
	if st != 200 {
		t.Fatalf("rank status=%d body=%v", st, rank)
	}
	items := mustData(t, rank)["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("rank items empty")
	}
}

func TestDishesCRUD(t *testing.T) {
	c, closeFn := newTestServer(t)
	defer closeFn()

	account := "138" + strconv.FormatInt(time.Now().UnixNano()%100000000, 10)
	password := "123456"
	name := "小熊"
	_, reg := c.doJSON(t, http.MethodPost, "/api/auth/register", "", map[string]any{
		"account":  account,
		"password": password,
		"name":     name,
	})
	_, login := c.doJSON(t, http.MethodPost, "/api/auth/login", "", map[string]any{
		"account":  account,
		"password": password,
	})
	token := mustData(t, login)["accessToken"].(string)
	userID := mustData(t, reg)["user"].(map[string]any)["id"].(string)

	st, list := c.doJSON(t, http.MethodGet, "/api/dishes?page=1&pageSize=2", "", nil)
	if st != 200 {
		t.Fatalf("dishes status=%d body=%v", st, list)
	}
	items := mustData(t, list)["items"].([]any)
	if len(items) == 0 {
		t.Fatalf("seed dishes missing")
	}
	firstID := items[0].(map[string]any)["id"].(string)

	st, detail := c.doJSON(t, http.MethodGet, "/api/dishes/"+url.PathEscape(firstID), "", nil)
	if st != 200 {
		t.Fatalf("dish detail status=%d body=%v", st, detail)
	}

	st, created := c.doJSON(t, http.MethodPost, "/api/dishes", token, map[string]any{
		"name":      "葱油拌面",
		"category":  "home",
		"timeText":  "15 分钟",
		"level":     "easy",
		"tags":      []string{"快手", "下饭"},
		"priceCent": 1800,
		"story":     "热气腾腾的一碗面，简单又满足",
		"imageUrl":  "",
		"badge":     "",
		"details": map[string]any{
			"ingredients": []string{"面 1 份"},
			"steps":       []string{"煮面", "拌匀"},
		},
	})
	if st != 200 {
		t.Fatalf("create dish status=%d body=%v", st, created)
	}
	dish := mustData(t, created)["dish"].(map[string]any)
	if dish["createdBy"] == nil {
		t.Fatalf("createdBy missing: %v", dish)
	}
	if strings.TrimSpace(dish["imageUrl"].(string)) == "" {
		t.Fatalf("imageUrl should be defaulted: %v", dish)
	}
	if strings.TrimSpace(dish["badge"].(string)) == "" {
		t.Fatalf("badge should be defaulted: %v", dish)
	}
	cb := dish["createdBy"].(map[string]any)
	if cb["userId"].(string) != userID {
		t.Fatalf("createdBy mismatch: %v", cb)
	}
	dishID := dish["id"].(string)

	st, mine := c.doJSON(t, http.MethodGet, "/api/dishes?scope=mine&page=1&pageSize=50", token, nil)
	if st != 200 {
		t.Fatalf("mine dishes status=%d body=%v", st, mine)
	}

	st, del := c.doJSON(t, http.MethodDelete, "/api/dishes/"+url.PathEscape(dishID), token, nil)
	if st != 200 {
		t.Fatalf("delete dish status=%d body=%v", st, del)
	}
}

func TestOrdersLifecycleAndReview_SSE_WS(t *testing.T) {
	c, closeFn := newTestServer(t)
	defer closeFn()

	createUser := func(name string) (account string, token string, userID string) {
		account = "138" + strconv.FormatInt(time.Now().UnixNano()%100000000, 10)
		password := "123456"
		_, reg := c.doJSON(t, http.MethodPost, "/api/auth/register", "", map[string]any{
			"account":  account,
			"password": password,
			"name":     name,
		})
		userID = mustData(t, reg)["user"].(map[string]any)["id"].(string)
		_, login := c.doJSON(t, http.MethodPost, "/api/auth/login", "", map[string]any{
			"account":  account,
			"password": password,
		})
		token = mustData(t, login)["accessToken"].(string)
		return
	}

	_, placerToken, placerID := createUser("小熊")
	_, cookToken, _ := createUser("妈妈")

	_, dishes := c.doJSON(t, http.MethodGet, "/api/dishes?page=1&pageSize=2", "", nil)
	dishID := mustData(t, dishes)["items"].([]any)[0].(map[string]any)["id"].(string)

	sseCtx, sseCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer sseCancel()

	sseReq, _ := http.NewRequestWithContext(sseCtx, http.MethodGet, c.baseURL+"/api/orders/stream?scope=mine", nil)
	sseReq.Header.Set("Authorization", "Bearer "+placerToken)
	sseRes, err := c.c.Do(sseReq)
	if err != nil {
		t.Fatalf("sse connect: %v", err)
	}
	defer sseRes.Body.Close()

	reader := bufio.NewReader(sseRes.Body)
	gotEvent := make(chan struct{})
	go func() {
		defer close(gotEvent)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "data: ") {
				return
			}
		}
	}()

	_, orderRes := c.doJSON(t, http.MethodPost, "/api/orders", placerToken, map[string]any{
		"items": []any{map[string]any{"dishId": dishID, "qty": 1}},
		"note":  "少盐",
	})
	order := mustData(t, orderRes)["order"].(map[string]any)
	orderID := order["id"].(string)
	meAfterPlace := mustData(t, orderRes)["me"].(map[string]any)
	if meAfterPlace["id"].(string) != placerID {
		t.Fatalf("me mismatch")
	}

	select {
	case <-gotEvent:
	case <-sseCtx.Done():
		t.Fatalf("sse did not receive data event")
	}

	u := url.URL{Scheme: "ws", Host: strings.TrimPrefix(c.baseURL, "http://"), Path: "/api/ws/orders"}
	q := u.Query()
	q.Set("scope", "mine")
	q.Set("token", placerToken)
	u.RawQuery = q.Encode()
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}

	_, acc := c.doJSON(t, http.MethodPost, "/api/orders/"+url.PathEscape(orderID)+"/accept", cookToken, nil)
	if _, ok := mustData(t, acc)["order"]; !ok {
		t.Fatalf("accept missing order: %v", acc)
	}

	_ = ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	gotUpdated := false
	for i := 0; i < 8; i++ {
		var msg map[string]any
		if err := ws.ReadJSON(&msg); err != nil {
			t.Fatalf("ws read: %v", err)
		}
		if msg["type"] != "order.updated" {
			continue
		}
		data, ok := msg["data"].(map[string]any)
		if !ok {
			continue
		}
		if data["orderId"] == orderID {
			gotUpdated = true
			break
		}
	}
	_ = ws.Close()
	if !gotUpdated {
		t.Fatalf("ws did not receive order.updated for %s", orderID)
	}

	st, fin := c.doMultipart(t, http.MethodPost, "/api/orders/"+url.PathEscape(orderID)+"/finish", cookToken,
		map[string]string{"note": "已完成"},
		map[string][]byte{"a.jpg": []byte("fakeimg")},
	)
	if st != 200 {
		t.Fatalf("finish status=%d body=%v", st, fin)
	}
	finData := mustData(t, fin)
	finOrder := finData["order"].(map[string]any)
	if finOrder["status"].(string) != "done" {
		t.Fatalf("finish status mismatch: %v", finOrder)
	}

	st, rev := c.doMultipart(t, http.MethodPost, "/api/orders/"+url.PathEscape(orderID)+"/review", placerToken,
		map[string]string{"rating": "5", "content": "好吃"},
		map[string][]byte{"b.jpg": []byte("fakeimg")},
	)
	if st != 200 {
		t.Fatalf("review status=%d body=%v", st, rev)
	}
	revOrder := mustData(t, rev)["order"].(map[string]any)
	if revOrder["review"] == nil {
		t.Fatalf("review missing: %v", revOrder)
	}
}

func TestRankEndpointDoesNotHang(t *testing.T) {
	c, closeFn := newTestServer(t)
	defer closeFn()

	account := "138" + strconv.FormatInt(time.Now().UnixNano()%100000000, 10)
	password := "123456"
	name := "小熊"

	_, _ = c.doJSON(t, http.MethodPost, "/api/auth/register", "", map[string]any{
		"account":  account,
		"password": password,
		"name":     name,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/users?sort=loveMilli_desc&limit=50", nil)
	res, err := c.c.Do(req)
	if err != nil {
		t.Fatalf("rank request: %v", err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if len(b) == 0 {
		t.Fatalf("empty response")
	}
}
