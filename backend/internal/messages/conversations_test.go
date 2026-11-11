package messages

import "testing"

func TestStartConversationWithoutApplication(t *testing.T) {
	h := newHarness(t)
	product := h.f.Product(seller, "APPROVED", 1000, 5)
	var first, again struct {
		ConversationID string `json:"conversationId"`
	}
	h.must(201, buyer, "POST", "/products/"+product+"/conversations", nil, &first)
	if first.ConversationID == "" {
		t.Fatal("conversationId is empty")
	}
	h.must(200, buyer, "POST", "/products/"+product+"/conversations", nil, &again)
	if again.ConversationID != first.ConversationID {
		t.Fatalf("second start made a new conversation: %s vs %s", again.ConversationID, first.ConversationID)
	}
	var count, inquirer int
	if err := h.f.DB.QueryRow("SELECT COUNT(*),MAX(inquirer_number) FROM participants WHERE product_id=?", product).Scan(&count, &inquirer); err != nil {
		t.Fatal(err)
	}
	if count != 1 || inquirer != 1 {
		t.Fatalf("participants=%d inquirer=%d, want 1 and 1", count, inquirer)
	}
}

func TestStartConversationReusesApplicant(t *testing.T) {
	h := newHarness(t)
	product := h.f.Product(seller, "APPROVED", 1000, 5)
	p := h.f.Participant(product, buyer, "application")
	var out struct {
		ConversationID string `json:"conversationId"`
	}
	h.must(200, buyer, "POST", "/products/"+product+"/conversations", nil, &out)
	if out.ConversationID != p.ID {
		t.Fatalf("got %s, want the application participant %s", out.ConversationID, p.ID)
	}
}

func TestStartConversationRules(t *testing.T) {
	h := newHarness(t)
	approved := h.f.Product(seller, "APPROVED", 1000, 5)
	h.expect(400, "VALIDATION_ERROR", seller, "POST", "/products/"+approved+"/conversations", nil)

	pending := h.f.Product(seller, "PENDING", 1000, 5)
	h.expect(409, "INVALID_STATE", buyer, "POST", "/products/"+pending+"/conversations", nil)

	deleted := h.f.Product(seller, "APPROVED", 1000, 5)
	h.exec("UPDATE products SET deleted_at=? WHERE id=?", "2026-11-05T03:00:00.000Z", deleted)
	h.expect(409, "INVALID_STATE", buyer, "POST", "/products/"+deleted+"/conversations", nil)

	h.expect(404, "NOT_FOUND", buyer, "POST", "/products/missing/conversations", nil)

	closed := h.f.Product(seller, "APPROVED", 1000, 5)
	h.exec("UPDATE products SET recruitment_closed=1 WHERE id=?", closed)
	h.must(201, buyer, "POST", "/products/"+closed+"/conversations", nil, nil)

	id := h.startConversation(buyer, approved)
	h.exec("UPDATE products SET deleted_at=? WHERE id=?", "2026-11-05T03:00:00.000Z", approved)
	var out struct {
		ConversationID string `json:"conversationId"`
	}
	h.must(200, buyer, "POST", "/products/"+approved+"/conversations", nil, &out)
	if out.ConversationID != id {
		t.Fatal("an existing conversation must survive product deletion")
	}
}
