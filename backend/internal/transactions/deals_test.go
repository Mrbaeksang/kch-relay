package transactions

import (
	"sync"
	"testing"
)

func TestBuyerAppliesAndSellerAcceptsRevealingIdentityOnlyToParties(t *testing.T) {
	h := newHarness(t, nil)
	product := h.f.Product(seller, "APPROVED", 10000, 5)

	d := h.apply(buyer, product, 2)
	if d.Status != "REQUESTED" || d.RequestedQuantity != 2 || d.AgreedUnitPrice != nil || d.TotalPrice != nil || d.AcceptedAt != nil {
		t.Fatalf("new application: %+v", d)
	}
	if d.Counterparty.Label != "판매자" || d.Counterparty.Identity != nil || d.ViewerRole != "BUYER" {
		t.Fatalf("buyer must see an anonymous seller: %+v", d.Counterparty)
	}
	if h.remaining(product) != 5 {
		t.Fatal("applying must not reserve stock")
	}

	var seen dealOut
	h.must(200, seller, "GET", "/deals/"+d.ID, nil, &seen)
	if seen.Counterparty.Label != "익명 1" || seen.Counterparty.Identity != nil || seen.ViewerRole != "SELLER" {
		t.Fatalf("seller must see an anonymous applicant before acceptance: %+v", seen.Counterparty)
	}
	h.expect(404, "NOT_FOUND", other, "GET", "/deals/"+d.ID, nil)

	var accepted dealOut
	h.must(200, seller, "POST", "/deals/"+d.ID+"/accept", nil, &accepted)
	if accepted.Status != "ACCEPTED" || *accepted.AgreedUnitPrice != 10000 || *accepted.AgreedQuantity != 2 || *accepted.TotalPrice != 20000 || accepted.AcceptedAt == nil {
		t.Fatalf("accepted deal: %+v", accepted)
	}
	if accepted.Counterparty.Identity == nil || accepted.Counterparty.Identity.RealName != "테스트 직원 B" {
		t.Fatalf("seller must see the buyer after acceptance: %+v", accepted.Counterparty)
	}
	if h.remaining(product) != 3 {
		t.Fatalf("remaining=%d, want 3", h.remaining(product))
	}
	h.must(200, buyer, "GET", "/deals/"+d.ID, nil, &seen)
	if seen.Counterparty.Identity == nil || seen.Counterparty.Identity.RealName != "테스트 직원 A" {
		t.Fatalf("buyer must see the seller after acceptance: %+v", seen.Counterparty)
	}
	h.expect(404, "NOT_FOUND", other, "GET", "/deals/"+d.ID, nil)

	// Editing the product later never changes what was agreed.
	h.must(200, seller, "PATCH", "/products/"+product, map[string]any{"unitPrice": 50000}, nil)
	h.must(200, buyer, "GET", "/deals/"+d.ID, nil, &seen)
	if *seen.AgreedUnitPrice != 10000 || *seen.TotalPrice != 20000 || seen.Product.UnitPrice != 50000 {
		t.Fatalf("agreed values changed with the product: %+v", seen)
	}

	var done dealOut
	h.must(200, seller, "POST", "/deals/"+d.ID+"/complete", nil, &done)
	if done.Status != "COMPLETED" || done.CompletedAt == nil || h.remaining(product) != 3 {
		t.Fatalf("completion must not touch stock: %+v remaining=%d", done, h.remaining(product))
	}
}

func TestConcurrentAcceptanceNeverOversellsAndRetriesAreHarmless(t *testing.T) {
	h := newHarness(t, nil)
	product := h.f.Product(seller, "APPROVED", 3000, 3)
	first := h.apply(buyer, product, 2)
	second := h.apply(other, product, 2)

	statuses := make([]int, 2)
	codes := make([]string, 2)
	var wg sync.WaitGroup
	for i, id := range []string{first.ID, second.ID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			statuses[i], codes[i] = h.call(seller, "POST", "/deals/"+id+"/accept", nil, nil)
		}()
	}
	wg.Wait()
	ok, rejected := 0, 0
	for i := range statuses {
		switch {
		case statuses[i] == 200:
			ok++
		case statuses[i] == 409 && codes[i] == "INSUFFICIENT_QUANTITY":
			rejected++
		}
	}
	if ok != 1 || rejected != 1 || h.remaining(product) != 1 {
		t.Fatalf("statuses=%v codes=%v remaining=%d", statuses, codes, h.remaining(product))
	}

	winner := first.ID
	if h.dealStatus(first.ID) != "ACCEPTED" {
		winner = second.ID
	}
	h.must(200, seller, "POST", "/deals/"+winner+"/accept", nil, nil)
	h.must(200, seller, "POST", "/deals/"+winner+"/complete", nil, nil)
	h.must(200, seller, "POST", "/deals/"+winner+"/complete", nil, nil)
	h.expect(409, "INVALID_STATE", seller, "POST", "/deals/"+winner+"/accept", nil)
	if h.remaining(product) != 1 {
		t.Fatalf("retries deducted stock again: remaining=%d", h.remaining(product))
	}
}

func TestOnlyTheRightPartyCanActOnADeal(t *testing.T) {
	h := newHarness(t, nil)
	product := h.f.Product(seller, "APPROVED", 1000, 5)
	d := h.apply(buyer, product, 1)

	h.expect(403, "FORBIDDEN", seller, "POST", "/products/"+product+"/deals", map[string]int{"quantity": 1})
	for _, path := range []string{"/accept", "/reject", "/complete"} {
		h.expect(403, "FORBIDDEN", buyer, "POST", "/deals/"+d.ID+path, nil)
		h.expect(404, "NOT_FOUND", other, "POST", "/deals/"+d.ID+path, nil)
	}
	h.expect(403, "FORBIDDEN", seller, "POST", "/deals/"+d.ID+"/withdraw", nil)
	h.expect(403, "FORBIDDEN", seller, "PATCH", "/deals/"+d.ID+"/quantity", map[string]int{"quantity": 2})
	h.expect(404, "NOT_FOUND", other, "POST", "/deals/"+d.ID+"/withdraw", nil)
	h.expect(404, "NOT_FOUND", other, "PATCH", "/deals/"+d.ID+"/quantity", map[string]int{"quantity": 2})
	h.expect(404, "NOT_FOUND", buyer, "GET", "/products/"+product+"/deals", nil)
	h.expect(404, "NOT_FOUND", other, "GET", "/products/"+product+"/deals", nil)

	var mine dealPage
	h.must(200, other, "GET", "/me/deals", nil, &mine)
	if len(mine.Items) != 0 {
		t.Fatal("another employee's deals leaked into my list")
	}
	if h.dealStatus(d.ID) != "REQUESTED" {
		t.Fatal("a blocked request changed the deal")
	}
}

func TestApplicationRules(t *testing.T) {
	h := newHarness(t, nil)
	pending := h.f.Product(seller, "PENDING", 1000, 5)
	h.expect(404, "NOT_FOUND", buyer, "POST", "/products/"+pending+"/deals", map[string]int{"quantity": 1})

	product := h.f.Product(seller, "APPROVED", 1000, 3)
	h.expect(400, "VALIDATION_ERROR", buyer, "POST", "/products/"+product+"/deals", map[string]int{"quantity": 0})
	h.expect(400, "VALIDATION_ERROR", buyer, "POST", "/products/"+product+"/deals", map[string]any{})
	h.expect(409, "INSUFFICIENT_QUANTITY", buyer, "POST", "/products/"+product+"/deals", map[string]int{"quantity": 4})

	d := h.apply(buyer, product, 1)
	h.expect(409, "ACTIVE_DEAL_EXISTS", buyer, "POST", "/products/"+product+"/deals", map[string]int{"quantity": 1})

	var changed dealOut
	h.must(200, buyer, "PATCH", "/deals/"+d.ID+"/quantity", map[string]int{"quantity": 3}, &changed)
	if changed.RequestedQuantity != 3 || changed.QuantityUpdatedAt == nil {
		t.Fatalf("quantity change must be recorded: %+v", changed)
	}
	h.expect(409, "INSUFFICIENT_QUANTITY", buyer, "PATCH", "/deals/"+d.ID+"/quantity", map[string]int{"quantity": 4})

	h.must(200, buyer, "POST", "/deals/"+d.ID+"/withdraw", nil, nil)
	h.must(200, buyer, "POST", "/deals/"+d.ID+"/withdraw", nil, nil)
	h.expect(409, "INVALID_STATE", buyer, "PATCH", "/deals/"+d.ID+"/quantity", map[string]int{"quantity": 2})

	again := h.apply(buyer, product, 1)
	if again.ID == d.ID || again.ConversationID != d.ConversationID {
		t.Fatal("re-application must be a new deal in the same conversation")
	}
	var seen dealOut
	h.must(200, seller, "GET", "/deals/"+again.ID, nil, &seen)
	if seen.Counterparty.Label != "익명 1" {
		t.Fatalf("re-application must keep the alias, got %q", seen.Counterparty.Label)
	}

	var list dealPage
	h.must(200, buyer, "GET", "/me/deals?limit=1", nil, &list)
	if len(list.Items) != 1 || list.NextCursor == nil || list.Items[0].ID != again.ID {
		t.Fatalf("first page: %+v", list)
	}
	h.must(200, buyer, "GET", "/me/deals?limit=1&cursor="+*list.NextCursor, nil, &list)
	if len(list.Items) != 1 || list.NextCursor != nil || list.Items[0].ID != d.ID {
		t.Fatalf("second page: %+v", list)
	}
	h.must(200, buyer, "GET", "/me/deals?status=WITHDRAWN", nil, &list)
	if len(list.Items) != 1 || list.Items[0].ID != d.ID {
		t.Fatalf("status filter: %+v", list)
	}
	h.expect(400, "INVALID_ENUM_VALUE", buyer, "GET", "/me/deals?status=UNKNOWN", nil)
	var sellerList struct {
		dealPage
		Product productOut `json:"product"`
	}
	h.must(200, seller, "GET", "/products/"+product+"/deals", nil, &sellerList)
	if len(sellerList.Items) != 2 || sellerList.Product.ID != product || sellerList.Product.PendingApplicationCount != 1 {
		t.Fatalf("seller list: %+v", sellerList)
	}
}

func TestRejectionNotifiesTheApplicantOncePerDeal(t *testing.T) {
	h := newHarness(t, nil)
	product := h.f.Product(seller, "APPROVED", 0, 2)
	d := h.apply(buyer, product, 1)
	h.must(200, seller, "POST", "/deals/"+d.ID+"/reject", nil, nil)
	h.must(200, seller, "POST", "/deals/"+d.ID+"/reject", nil, nil)
	h.expect(409, "INVALID_STATE", seller, "POST", "/deals/"+d.ID+"/accept", nil)
	if h.notifications(d.ID) != 1 {
		t.Fatalf("notifications=%d, want 1", h.notifications(d.ID))
	}
	if got := h.text("SELECT recipient_id FROM notifications WHERE deal_id=?", d.ID); got != buyer {
		t.Fatalf("notification went to %s", got)
	}
}

func TestReducedStockKeepsApplicationsButBlocksAcceptance(t *testing.T) {
	h := newHarness(t, nil)
	product := h.f.Product(seller, "APPROVED", 1000, 5)
	d := h.apply(buyer, product, 4)

	var p productOut
	h.must(200, seller, "PATCH", "/products/"+product+"/quantity", map[string]int{"remainingQuantity": 2}, &p)
	if p.RemainingQuantity != 2 || p.PendingApplicationCount != 1 || h.dealStatus(d.ID) != "REQUESTED" || h.notifications(d.ID) != 0 {
		t.Fatalf("stock change must keep applications: %+v", p)
	}
	h.expect(409, "INSUFFICIENT_QUANTITY", seller, "POST", "/deals/"+d.ID+"/accept", nil)
	h.expect(400, "VALIDATION_ERROR", seller, "PATCH", "/products/"+product+"/quantity", map[string]int{"remainingQuantity": -1})

	h.must(200, seller, "PATCH", "/products/"+product+"/quantity", map[string]int{"remainingQuantity": 0}, nil)
	h.expect(409, "INSUFFICIENT_QUANTITY", other, "POST", "/products/"+product+"/deals", map[string]int{"quantity": 1})

	h.must(200, seller, "PATCH", "/products/"+product+"/quantity", map[string]int{"remainingQuantity": 6}, nil)
	h.must(200, seller, "POST", "/products/"+product+"/close", nil, &p)
	if !p.RecruitmentClosed {
		t.Fatal("recruitment not closed")
	}
	h.expect(409, "RECRUITMENT_CLOSED", other, "POST", "/products/"+product+"/deals", map[string]int{"quantity": 1})
	h.must(200, seller, "POST", "/deals/"+d.ID+"/accept", nil, nil)
	if h.remaining(product) != 2 {
		t.Fatalf("remaining=%d, want 2", h.remaining(product))
	}
	h.must(200, seller, "POST", "/products/"+product+"/reopen", nil, &p)
	h.apply(other, product, 1)
}
