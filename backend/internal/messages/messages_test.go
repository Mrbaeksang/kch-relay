package messages

import (
	"strings"
	"testing"
	"time"
)

type messageItem struct {
	ID        string  `json:"id"`
	Body      string  `json:"body"`
	CreatedAt string  `json:"createdAt"`
	ReadAt    *string `json:"readAt"`
	Sender    struct {
		Kind     string    `json:"kind"`
		Label    string    `json:"label"`
		Identity *identity `json:"identity"`
	} `json:"sender"`
}

type messagePage struct {
	Items      []messageItem `json:"items"`
	NextCursor *string       `json:"nextCursor"`
}

func (h *harness) messages(user, conversation, query string) messagePage {
	h.t.Helper()
	var page messagePage
	h.must(200, user, "GET", "/conversations/"+conversation+"/messages"+query, nil, &page)
	return page
}

func (h *harness) sendWithKey(user, conversation, key string, body any) (int, string, messageItem) {
	h.t.Helper()
	var m messageItem
	headers := map[string]string{}
	if key != "" {
		headers["Idempotency-Key"] = key
	}
	status, code, _ := h.send(user, "POST", "/conversations/"+conversation+"/messages", body, headers, &m)
	return status, code, m
}

func newConversation(h *harness) string {
	return h.startConversation(buyer, h.f.Product(seller, "APPROVED", 1000, 5))
}

func TestSendAndListMessages(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	status, _, sent := h.sendWithKey(buyer, id, "k1", map[string]string{"body": "안녕하세요\n두 줄"})
	if status != 201 || sent.Sender.Kind != "SELF" || sent.Sender.Label != "나" || sent.ReadAt != nil {
		t.Fatalf("send: %d %+v", status, sent)
	}
	var page messagePage
	_, _, meta := h.send(seller, "GET", "/conversations/"+id+"/messages", nil, nil, &page)
	if len(page.Items) != 1 {
		t.Fatalf("seller sees %d messages", len(page.Items))
	}
	m := page.Items[0]
	if m.ID != sent.ID || m.Body != "안녕하세요\n두 줄" || m.Sender.Kind != "COUNTERPARTY" || m.Sender.Label != "문의자 A" || m.ReadAt != nil {
		t.Fatalf("seller view: %+v", m)
	}
	if meta == nil || !meta.HasUnreadMessages {
		t.Fatalf("seller meta must show unread messages: %+v", meta)
	}
}

func TestSendValidation(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	body := func(s string) map[string]string { return map[string]string{"body": s} }
	for name, c := range map[string]struct {
		key  string
		body any
	}{
		"missing key":       {"", body("안녕")},
		"key too long":      {strings.Repeat("k", 129), body("안녕")},
		"key with space":    {"a b", body("안녕")},
		"blank body":        {"k-blank", body("  \n\t")},
		"2001 characters":   {"k-long", body(strings.Repeat("가", 2001))},
		"spoofed recipient": {"k-spoof", map[string]string{"body": "안녕", "recipientId": seller, "senderId": buyer}},
	} {
		if status, code, _ := h.sendWithKey(buyer, id, c.key, c.body); status != 400 || code != "VALIDATION_ERROR" {
			t.Errorf("%s: %d %s, want 400 VALIDATION_ERROR", name, status, code)
		}
	}
	if status, code, _ := h.sendWithKey(buyer, id, "k-max", body(strings.Repeat("가", 2000))); status != 201 {
		t.Fatalf("exactly 2000 characters: %d %s", status, code)
	}
	if status, _, _ := h.sendWithKey(buyer, id, strings.Repeat("k", 128), body("최대 키")); status != 201 {
		t.Fatalf("128-character key must be accepted: %d", status)
	}
}

func TestSendIdempotent(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	s1, _, first := h.sendWithKey(buyer, id, "same", map[string]string{"body": "한 번만"})
	h.tick(time.Second)
	s2, _, again := h.sendWithKey(buyer, id, "same", map[string]string{"body": "한 번만"})
	if s1 != 201 || s2 != 200 || first.ID != again.ID {
		t.Fatalf("retry: %d %d %s %s", s1, s2, first.ID, again.ID)
	}
	if status, code, _ := h.sendWithKey(buyer, id, "same", map[string]string{"body": "다른 내용"}); status != 409 || code != "IDEMPOTENCY_KEY_REUSED" {
		t.Fatalf("reused key with another body: %d %s", status, code)
	}
	if status, _, _ := h.sendWithKey(seller, id, "same", map[string]string{"body": "한 번만"}); status != 201 {
		t.Fatalf("the other party's key is independent: %d", status)
	}
	var n int
	h.f.DB.QueryRow("SELECT COUNT(*) FROM messages WHERE participant_id=?", id).Scan(&n)
	if n != 2 {
		t.Fatalf("%d messages stored, want 2", n)
	}
}

func TestRecipientDecidedByServer(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	fromBuyer := h.post(buyer, id, "구매자")
	fromSeller := h.post(seller, id, "판매자")
	for msg, want := range map[string][2]string{fromBuyer: {buyer, seller}, fromSeller: {seller, buyer}} {
		var sender, recipient string
		h.f.DB.QueryRow("SELECT sender_id,recipient_id FROM messages WHERE id=?", msg).Scan(&sender, &recipient)
		if sender != want[0] || recipient != want[1] {
			t.Fatalf("message %s: %s→%s, want %s→%s", msg, sender, recipient, want[0], want[1])
		}
	}
}

func TestGetDoesNotMarkRead(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	msg := h.post(buyer, id, "읽지 않음")
	h.messages(seller, id, "")
	h.messages(seller, id, "")
	var readAt *string
	h.f.DB.QueryRow("SELECT read_at FROM messages WHERE id=?", msg).Scan(&readAt)
	if readAt != nil {
		t.Fatal("listing messages must not mark them read")
	}
}

func TestMessagesNewestFirstWithCursor(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	var last string
	for i := 0; i < 25; i++ {
		h.tick(time.Second)
		last = h.post(buyer, id, strings.Repeat("쪽", i+1))
	}
	page := h.messages(seller, id, "")
	if len(page.Items) != 20 || page.Items[0].ID != last || page.NextCursor == nil {
		t.Fatalf("first page: %d items, first %s", len(page.Items), page.Items[0].ID)
	}
	rest := h.messages(seller, id, "?cursor="+*page.NextCursor)
	if len(rest.Items) != 5 || rest.NextCursor != nil || rest.Items[4].Body != "쪽" {
		t.Fatalf("second page: %+v", rest)
	}
}

func TestSendAfterProductHidden(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	h.exec("UPDATE products SET deleted_at=?", "2026-11-05T03:00:00.000Z")
	h.post(buyer, id, "숨김 후에도")
	if n := len(h.messages(seller, id, "").Items); n != 1 {
		t.Fatalf("seller sees %d messages after hiding", n)
	}
}

func TestMonotonicCreatedAt(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	h.post(buyer, id, "하나")
	h.post(seller, id, "둘")
	h.post(buyer, id, "셋")
	items := h.messages(buyer, id, "").Items
	if !(items[0].CreatedAt > items[1].CreatedAt && items[1].CreatedAt > items[2].CreatedAt) {
		t.Fatalf("created_at must strictly increase within a conversation: %s %s %s", items[2].CreatedAt, items[1].CreatedAt, items[0].CreatedAt)
	}
}

func TestMessagesIncludeConversationHeader(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	var page struct {
		Conversation listItem `json:"conversation"`
	}
	h.must(200, buyer, "GET", "/conversations/"+id+"/messages", nil, &page)
	c := page.Conversation
	if c.ConversationID != id || c.ViewerRole != "BUYER" || c.Product.Title != "가상 상품" || c.Counterparty.Label != "판매자" {
		t.Fatalf("header for an empty conversation: %+v", c)
	}
}
