package messages

import (
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"strings"
	"testing"
	"time"
)

type listItem struct {
	ConversationID string `json:"conversationId"`
	ViewerRole     string `json:"viewerRole"`
	Product        struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"product"`
	Counterparty struct {
		Label    string    `json:"label"`
		Identity *identity `json:"identity"`
	} `json:"counterparty"`
	LastMessage *struct {
		Body      string `json:"body"`
		CreatedAt string `json:"createdAt"`
		FromSelf  bool   `json:"fromSelf"`
	} `json:"lastMessage"`
	HasUnread bool `json:"hasUnread"`
}

type listPage struct {
	Items      []listItem `json:"items"`
	NextCursor *string    `json:"nextCursor"`
}

func (h *harness) list(user, query string) listPage {
	h.t.Helper()
	var page listPage
	h.must(200, user, "GET", "/me/conversations"+query, nil, &page)
	return page
}

// post sends a message with a fresh idempotency key and returns its id.
func (h *harness) post(user, conversation, body string) string {
	h.t.Helper()
	var m struct {
		ID string `json:"id"`
	}
	status, code, _ := h.send(user, "POST", "/conversations/"+conversation+"/messages", map[string]string{"body": body}, map[string]string{"Idempotency-Key": httpx.NewID()}, &m)
	if status != 201 {
		h.t.Fatalf("post as %s: %d %s", user, status, code)
	}
	return m.ID
}

func TestListLabelsAndAliasSwitch(t *testing.T) {
	h := newHarness(t)
	product := h.f.Product(seller, "APPROVED", 1000, 5)
	id := h.startConversation(buyer, product)
	h.post(buyer, id, "문의드립니다")

	page := h.list(seller, "")
	if len(page.Items) != 1 || page.Items[0].Counterparty.Label != "문의자 A" || page.Items[0].ViewerRole != "SELLER" {
		t.Fatalf("seller view before application: %+v", page.Items)
	}
	h.f.Participant(product, buyer, "application")
	page = h.list(seller, "")
	if page.Items[0].Counterparty.Label != "익명 1" || page.Items[0].ConversationID != id {
		t.Fatalf("seller view after application: %+v", page.Items[0])
	}
	b := h.list(buyer, "").Items
	if len(b) != 1 || b[0].Counterparty.Label != "판매자" || b[0].ViewerRole != "BUYER" || b[0].Product.Title != "가상 상품" {
		t.Fatalf("buyer view: %+v", b)
	}
}

func TestListHidesEmptyConversationFromSeller(t *testing.T) {
	h := newHarness(t)
	product := h.f.Product(seller, "APPROVED", 1000, 5)
	h.startConversation(buyer, product)
	if n := len(h.list(seller, "").Items); n != 0 {
		t.Fatalf("seller sees %d empty conversations", n)
	}
	b := h.list(buyer, "").Items
	if len(b) != 1 || b[0].LastMessage != nil {
		t.Fatalf("buyer must see the empty conversation: %+v", b)
	}
}

func TestListMixesRolesWithCorrectLabels(t *testing.T) {
	h := newHarness(t)
	p1 := h.f.Product(seller, "APPROVED", 1000, 5)
	p2 := h.f.Product(buyer, "APPROVED", 1000, 5)
	h.post(buyer, h.startConversation(buyer, p1), "P1 문의")
	h.tick(time.Minute)
	h.post(seller, h.startConversation(seller, p2), "P2 문의")
	items := h.list(seller, "").Items
	if len(items) != 2 {
		t.Fatalf("want 2 conversations, got %+v", items)
	}
	byProduct := map[string]listItem{items[0].Product.ID: items[0], items[1].Product.ID: items[1]}
	if c := byProduct[p1]; c.ViewerRole != "SELLER" || c.Counterparty.Label != "문의자 A" {
		t.Fatalf("P1 as seller: %+v", c)
	}
	if c := byProduct[p2]; c.ViewerRole != "BUYER" || c.Counterparty.Label != "판매자" {
		t.Fatalf("P2 as buyer: %+v", c)
	}
}

func TestListUnreadOrderAndCursor(t *testing.T) {
	h := newHarness(t)
	var ids []string
	for i := 0; i < 3; i++ {
		product := h.f.Product(seller, "APPROVED", 1000, 5)
		id := h.startConversation(buyer, product)
		ids = append(ids, id)
		h.tick(time.Minute)
		h.post(buyer, id, strings.Repeat("가", 101))
	}
	h.tick(time.Minute)
	h.post(seller, ids[0], "답장") // the oldest conversation becomes the newest
	h.exec("UPDATE messages SET read_at=? WHERE participant_id=? AND recipient_id=?", "2026-11-05T03:10:00.000Z", ids[0], seller)
	page := h.list(seller, "?limit=2")
	if len(page.Items) != 2 || page.NextCursor == nil {
		t.Fatalf("first page: %+v", page)
	}
	if page.Items[0].ConversationID != ids[0] || page.Items[1].ConversationID != ids[2] {
		t.Fatalf("order: %s %s", page.Items[0].ConversationID, page.Items[1].ConversationID)
	}
	if page.Items[0].HasUnread || !page.Items[1].HasUnread {
		t.Fatalf("unread flags: %v %v", page.Items[0].HasUnread, page.Items[1].HasUnread)
	}
	if !page.Items[0].LastMessage.FromSelf || page.Items[0].LastMessage.Body != "답장" {
		t.Fatalf("last message: %+v", page.Items[0].LastMessage)
	}
	if n := len([]rune(page.Items[1].LastMessage.Body)); n != 100 {
		t.Fatalf("preview has %d characters, want 100", n)
	}
	rest := h.list(seller, "?limit=2&cursor="+*page.NextCursor)
	if len(rest.Items) != 1 || rest.Items[0].ConversationID != ids[1] || rest.NextCursor != nil {
		t.Fatalf("second page: %+v", rest)
	}
}

func TestIdentityOnlyWhenRevealed(t *testing.T) {
	h := newHarness(t)
	product := h.f.Product(seller, "APPROVED", 1000, 5)
	p := h.f.Participant(product, buyer, "application")
	h.post(buyer, p.ID, "신청했습니다")
	if got := h.list(seller, "").Items[0].Counterparty.Identity; got != nil {
		t.Fatalf("identity before acceptance: %+v", got)
	}
	h.f.Deal(p.ID, "ACCEPTED", 1, 1000)
	s := h.list(seller, "").Items[0].Counterparty.Identity
	b := h.list(buyer, "").Items[0].Counterparty.Identity
	if s == nil || *s != (identity{RealName: "테스트 직원 B", Department: "TEST_DEPT", JobLevel: "TEST_LEVEL"}) {
		t.Fatalf("seller sees %+v", s)
	}
	if b == nil || b.RealName != "테스트 직원 A" {
		t.Fatalf("buyer sees %+v", b)
	}
}
