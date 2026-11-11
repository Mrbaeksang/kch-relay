package transactions

import (
	"testing"
)

func TestEditingContentRequiresRejectingPendingApplications(t *testing.T) {
	h := newHarness(t, nil)
	product := h.f.Product(seller, "APPROVED", 1000, 10)
	first := h.apply(buyer, product, 1)
	second := h.apply(other, product, 2)

	status, code, details := h.callDetails(seller, "PATCH", "/products/"+product, map[string]any{"title": "새 제목"}, nil)
	if status != 409 || code != "PENDING_APPLICATIONS_EXIST" || details["pendingCount"] != float64(2) {
		t.Fatalf("got %d %s %v", status, code, details)
	}
	if h.text("SELECT title FROM products WHERE id=?", product) != "가상 상품" {
		t.Fatal("product changed although the edit was refused")
	}

	// Submitting the same values is not a change and rejects nobody.
	h.must(200, seller, "PATCH", "/products/"+product, map[string]any{"title": "가상 상품", "unitPrice": 1000, "rejectPendingApplications": true}, nil)
	if h.dealStatus(first.ID) != "REQUESTED" || h.notifications(first.ID) != 0 {
		t.Fatal("a no-op edit rejected applications")
	}

	var p productOut
	h.must(200, seller, "PATCH", "/products/"+product, map[string]any{"title": "새 제목", "description": "새 설명", "rejectPendingApplications": true}, &p)
	if p.Title != "새 제목" || p.Description != "새 설명" || p.PendingApplicationCount != 0 || p.ApprovalStatus != "APPROVED" {
		t.Fatalf("edited product: %+v", p)
	}
	for _, d := range []dealOut{first, second} {
		if h.dealStatus(d.ID) != "REJECTED" || h.notifications(d.ID) != 1 {
			t.Fatalf("deal %s: status=%s notifications=%d", d.ID, h.dealStatus(d.ID), h.notifications(d.ID))
		}
	}
	h.expect(400, "VALIDATION_ERROR", seller, "PATCH", "/products/"+product, map[string]any{"title": "   "})
	h.expect(400, "VALIDATION_ERROR", seller, "PATCH", "/products/"+product, map[string]any{"unitPrice": -1})
}

func TestEditingKeepsAcceptedDealsAndIsBlockedForOthers(t *testing.T) {
	h := newHarness(t, nil)
	product := h.f.Product(seller, "APPROVED", 1000, 10)
	accepted := h.apply(buyer, product, 2)
	h.must(200, seller, "POST", "/deals/"+accepted.ID+"/accept", nil, nil)

	h.expect(404, "NOT_FOUND", buyer, "PATCH", "/products/"+product, map[string]any{"title": "가로채기"})
	h.expect(404, "NOT_FOUND", buyer, "PATCH", "/products/"+product+"/quantity", map[string]int{"remainingQuantity": 99})
	h.expect(404, "NOT_FOUND", buyer, "POST", "/products/"+product+"/close", nil)
	h.expect(404, "NOT_FOUND", buyer, "DELETE", "/products/"+product, nil)

	h.must(200, seller, "PATCH", "/products/"+product, map[string]any{"title": "성사 후 수정", "unitPrice": 2000}, nil)
	if h.dealStatus(accepted.ID) != "ACCEPTED" || h.scalar("SELECT agreed_unit_price FROM deals WHERE id=?", accepted.ID) != 1000 {
		t.Fatal("editing touched an accepted deal")
	}
}

func TestFailedNotificationRollsBackRejectionAndEdit(t *testing.T) {
	h := newHarness(t, nil)
	product := h.f.Product(seller, "APPROVED", 1000, 10)
	d := h.apply(buyer, product, 1)
	if _, err := h.f.DB.Exec(`CREATE TRIGGER fail_notifications BEFORE INSERT ON notifications BEGIN SELECT RAISE(ABORT,'notification failure'); END`); err != nil {
		t.Fatal(err)
	}
	status, _ := h.call(seller, "PATCH", "/products/"+product, map[string]any{"title": "새 제목", "rejectPendingApplications": true}, nil)
	if status != 500 {
		t.Fatalf("status=%d, want 500", status)
	}
	status, _ = h.call(seller, "POST", "/deals/"+d.ID+"/reject", nil, nil)
	if status != 500 {
		t.Fatalf("status=%d, want 500", status)
	}
	if h.dealStatus(d.ID) != "REQUESTED" || h.text("SELECT title FROM products WHERE id=?", product) != "가상 상품" {
		t.Fatal("partial rejection or edit survived a notification failure")
	}
}

func TestHidingKeepsRecordsAndRejectsPendingApplications(t *testing.T) {
	h := newHarness(t, nil)
	product := h.f.Product(seller, "APPROVED", 1000, 10)
	pending := h.apply(buyer, product, 1)
	accepted := h.apply(other, product, 1)
	h.must(200, seller, "POST", "/deals/"+accepted.ID+"/accept", nil, nil)

	var result struct {
		Deleted bool `json:"deleted"`
	}
	h.must(200, seller, "DELETE", "/products/"+product, nil, &result)
	h.must(200, seller, "DELETE", "/products/"+product, nil, &result)
	if !result.Deleted || h.dealStatus(pending.ID) != "REJECTED" || h.notifications(pending.ID) != 1 {
		t.Fatalf("hide: deleted=%v pending=%s notifications=%d", result.Deleted, h.dealStatus(pending.ID), h.notifications(pending.ID))
	}
	h.expect(404, "NOT_FOUND", buyer, "POST", "/products/"+product+"/deals", map[string]int{"quantity": 1})
	h.expect(409, "INVALID_STATE", seller, "PATCH", "/products/"+product, map[string]any{"title": "숨긴 뒤 수정"})

	var seen dealOut
	h.must(200, buyer, "GET", "/deals/"+pending.ID, nil, &seen)
	if !seen.Product.Hidden || seen.Status != "REJECTED" {
		t.Fatalf("hidden product's record: %+v", seen)
	}
	h.must(200, seller, "POST", "/deals/"+accepted.ID+"/complete", nil, nil)
}

func TestPhotoReplacementUsesTheAttachContract(t *testing.T) {
	h := newHarness(t, testAttachImages)
	product := h.f.Product(seller, "APPROVED", 1000, 10)
	mine, others := h.image(seller), h.image(buyer)

	var p productOut
	h.must(200, seller, "PATCH", "/products/"+product, map[string]any{"imageIds": []string{mine}}, &p)
	if len(p.ImageIDs) != 1 || p.ImageIDs[0] != mine {
		t.Fatalf("images: %+v", p.ImageIDs)
	}
	d := h.apply(buyer, product, 1)
	h.must(200, seller, "PATCH", "/products/"+product, map[string]any{"imageIds": []string{mine}}, nil)
	if h.dealStatus(d.ID) != "REQUESTED" {
		t.Fatal("the same photos counted as a change")
	}
	h.expect(400, "VALIDATION_ERROR", seller, "PATCH", "/products/"+product, map[string]any{"imageIds": []string{others}, "rejectPendingApplications": true})
	if h.dealStatus(d.ID) != "REQUESTED" || h.text("SELECT product_id FROM images WHERE id=?", mine) != product {
		t.Fatal("a refused photo change was partially applied")
	}

	without := newHarness(t, nil)
	other := without.f.Product(seller, "APPROVED", 1000, 10)
	without.expect(400, "VALIDATION_ERROR", seller, "PATCH", "/products/"+other, map[string]any{"imageIds": []string{without.image(seller)}})
}
