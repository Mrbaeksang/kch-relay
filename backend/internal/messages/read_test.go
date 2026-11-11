package messages

import "testing"

type readResult struct {
	UpdatedCount int `json:"updatedCount"`
}

func (h *harness) read(user, conversation, through string) (readResult, bool) {
	h.t.Helper()
	var out readResult
	status, code, meta := h.send(user, "POST", "/conversations/"+conversation+"/read", map[string]string{"throughMessageId": through}, nil, &out)
	if status != 200 {
		h.t.Fatalf("read as %s: %d %s", user, status, code)
	}
	return out, meta.HasUnreadMessages
}

func (h *harness) readAt(message string) *string {
	h.t.Helper()
	var at *string
	if err := h.f.DB.QueryRow("SELECT read_at FROM messages WHERE id=?", message).Scan(&at); err != nil {
		h.t.Fatal(err)
	}
	return at
}

func TestReadThroughOnlyReceivedUpTo(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	m1 := h.post(buyer, id, "하나")
	m2 := h.post(buyer, id, "둘")
	m3 := h.post(seller, id, "셋")
	if got, _ := h.read(seller, id, m1); got.UpdatedCount != 1 || h.readAt(m1) == nil || h.readAt(m2) != nil {
		t.Fatalf("through m1: %+v", got)
	}
	if got, _ := h.read(seller, id, m3); got.UpdatedCount != 1 || h.readAt(m2) == nil || h.readAt(m3) != nil {
		t.Fatalf("through m3 must only cover messages the seller received: %+v", got)
	}
	var page messagePage
	h.must(200, buyer, "GET", "/conversations/"+id+"/messages", nil, &page)
	if page.Items[2].ReadAt == nil || page.Items[0].ReadAt != nil {
		t.Fatal("the buyer must see that the seller read the buyer's messages")
	}
}

func TestReadSameMillisecondArrivalStaysUnread(t *testing.T) {
	h := newHarness(t) // the clock does not move between these calls
	id := newConversation(h)
	m1 := h.post(buyer, id, "먼저")
	h.read(seller, id, m1)
	m2 := h.post(buyer, id, "같은 순간")
	got, unread := h.read(seller, id, m1)
	if got.UpdatedCount != 0 || h.readAt(m2) != nil || !unread {
		t.Fatalf("a later message must stay unread: %+v unread=%v", got, unread)
	}
}

func TestReadRejectsForeignMessage(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	h.post(buyer, id, "여기")
	otherConversation := newConversation(h)
	foreign := h.post(buyer, otherConversation, "저기")
	h.expect(400, "VALIDATION_ERROR", seller, "POST", "/conversations/"+id+"/read", map[string]string{"throughMessageId": foreign})
	h.expect(400, "VALIDATION_ERROR", seller, "POST", "/conversations/"+id+"/read", map[string]string{"throughMessageId": "missing"})
	if h.readAt(foreign) != nil {
		t.Fatal("a foreign message must not be marked read")
	}
}

func TestReadUpdatesMetaAndRepeats(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	m := h.post(buyer, id, "확인해 주세요")
	got, unread := h.read(seller, id, m)
	if got.UpdatedCount != 1 || unread {
		t.Fatalf("first read: %+v unread=%v", got, unread)
	}
	if again, _ := h.read(seller, id, m); again.UpdatedCount != 0 {
		t.Fatalf("repeated read: %+v", again)
	}
}
