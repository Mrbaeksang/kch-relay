package messages

import (
	"strings"
	"testing"
)

// assertBlocked checks that user cannot read, write or mark read the conversation.
func assertBlocked(t *testing.T, h *harness, user, conversation, message string) {
	t.Helper()
	h.expect(404, "NOT_FOUND", user, "GET", "/conversations/"+conversation+"/messages", nil)
	if status, code, _ := h.sendWithKey(user, conversation, "intruder", map[string]string{"body": "끼어들기"}); status != 404 || code != "NOT_FOUND" {
		t.Fatalf("send as %s: %d %s", user, status, code)
	}
	h.expect(404, "NOT_FOUND", user, "POST", "/conversations/"+conversation+"/read", map[string]string{"throughMessageId": message})
	for _, item := range h.list(user, "").Items {
		if item.ConversationID == conversation {
			t.Fatalf("%s lists a conversation that is not theirs", user)
		}
	}
	if h.readAt(message) != nil {
		t.Fatal("a blocked read must not change anything")
	}
}

func TestThirdPartyIncludingAdminIsBlocked(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	m := h.post(buyer, id, "둘만의 대화")
	assertBlocked(t, h, other, id, m) // dev-admin is an HR administrator
}

func TestOtherBuyerOfSameProductIsBlocked(t *testing.T) {
	h := newHarness(t)
	product := h.f.Product(seller, "APPROVED", 1000, 5)
	id := h.startConversation(buyer, product)
	m := h.post(buyer, id, "구매자 B의 문의")
	h.post(other, h.startConversation(other, product), "다른 구매자의 문의")
	assertBlocked(t, h, other, id, m)
}

func TestParticipantOfOtherProductIsBlocked(t *testing.T) {
	h := newHarness(t)
	id := newConversation(h)
	m := h.post(buyer, id, "이 상품 문의")
	h.post(other, h.startConversation(other, h.f.Product(buyer, "APPROVED", 1000, 5)), "다른 상품 문의")
	assertBlocked(t, h, other, id, m)
}

func TestResponsesNeverExposeUserIDs(t *testing.T) {
	h := newHarness(t)
	product := h.f.Product(seller, "APPROVED", 1000, 5)
	p := h.f.Participant(product, buyer, "application")
	m := h.post(buyer, p.ID, "신원 확인")
	h.f.Deal(p.ID, "ACCEPTED", 1, 1000)
	for _, user := range []string{seller, buyer} {
		bodies := [][]byte{}
		_, raw := h.callRaw(user, "GET", "/me/conversations", nil, nil)
		bodies = append(bodies, raw)
		_, raw = h.callRaw(user, "GET", "/conversations/"+p.ID+"/messages", nil, nil)
		bodies = append(bodies, raw)
		_, raw = h.callRaw(user, "POST", "/conversations/"+p.ID+"/messages", map[string]string{"body": "답장 " + user[len(user)-1:]}, map[string]string{"Idempotency-Key": "id-check-" + user})
		bodies = append(bodies, raw)
		_, raw = h.callRaw(user, "POST", "/conversations/"+p.ID+"/read", map[string]string{"throughMessageId": m}, nil)
		bodies = append(bodies, raw)
		for _, b := range bodies {
			for _, id := range []string{seller, buyer, other} {
				if strings.Contains(string(b), id) {
					t.Fatalf("response to %s exposes user id %s: %s", user, id, b)
				}
			}
		}
	}
}

// TestRealAcceptanceRevealsIdentity runs the #18 application and acceptance
// endpoints and checks that only the two matched people see each other.
func TestRealAcceptanceRevealsIdentity(t *testing.T) {
	h := newHarness(t)
	product := h.f.Product(seller, "APPROVED", 1000, 5)
	id := h.startConversation(buyer, product)
	h.post(buyer, id, "신청 전에 문의")
	otherID := h.startConversation(other, product)
	h.post(other, otherID, "저도 문의")

	var deal struct {
		ID             string `json:"id"`
		ConversationID string `json:"conversationId"`
	}
	h.must(201, buyer, "POST", "/products/"+product+"/deals", map[string]int{"quantity": 1}, &deal)
	if deal.ConversationID != id {
		t.Fatalf("applying must keep the conversation: %s vs %s", deal.ConversationID, id)
	}
	if got := h.messages(seller, id, "").Items[0].Sender; got.Label != "익명 1" || got.Identity != nil {
		t.Fatalf("after application, before acceptance: %+v", got)
	}
	h.must(200, seller, "POST", "/deals/"+deal.ID+"/accept", nil, nil)

	if got := h.messages(seller, id, "").Items[0].Sender; got.Identity == nil || got.Identity.RealName != "테스트 직원 B" {
		t.Fatalf("seller must see the matched buyer: %+v", got)
	}
	h.post(seller, id, "수락했습니다")
	if got := h.messages(buyer, id, "").Items[0].Sender; got.Identity == nil || got.Identity.RealName != "테스트 직원 A" {
		t.Fatalf("buyer must see the matched seller: %+v", got)
	}
	for _, item := range h.list(seller, "").Items {
		if item.ConversationID == otherID && item.Counterparty.Identity != nil {
			t.Fatal("an unmatched inquirer must stay anonymous")
		}
	}
	if got := h.messages(other, otherID, "").Items[0].Sender; got.Kind != "SELF" {
		t.Fatalf("other inquirer view: %+v", got)
	}
}
