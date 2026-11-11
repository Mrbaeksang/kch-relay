// Package product implements product registration, photo upload and HR approval.
package product

// Input limits are published through GET /product-limits so screens never hardcode them.
// Price × quantity stays far below 2^53 so the browser can compute totals exactly.
const (
	MaxTitleLength       = 100
	MaxDescriptionLength = 5000
	MaxUnitPrice         = int64(10_000_000_000) // 100억 원; PRD requires at least 1억 원
	MaxQuantity          = int64(10_000)
	MaxImagesPerProduct  = 5
	MaxImageBytes        = int64(5 << 20)
	MaxImagePixels       = 16_000_000 // ~64MB while decoding; phone photos are resized in the browser first
	MaxImageSide         = 8_000
	MaxUnlinkedUploads   = 20 // per uploader, limits disk use by abandoned uploads
	MaxReasonLength      = 500
)

type Limits struct {
	MaxTitleLength       int      `json:"maxTitleLength"`
	MaxDescriptionLength int      `json:"maxDescriptionLength"`
	MinUnitPrice         int64    `json:"minUnitPrice"`
	MaxUnitPrice         int64    `json:"maxUnitPrice"`
	MinQuantity          int64    `json:"minQuantity"`
	MaxQuantity          int64    `json:"maxQuantity"`
	MaxImagesPerProduct  int      `json:"maxImagesPerProduct"`
	MaxImageBytes        int64    `json:"maxImageBytes"`
	ImageTypes           []string `json:"imageTypes"`
	MaxReasonLength      int      `json:"maxReasonLength"`
}

func CurrentLimits() Limits {
	return Limits{MaxTitleLength, MaxDescriptionLength, 0, MaxUnitPrice, 1, MaxQuantity, MaxImagesPerProduct, MaxImageBytes, []string{"image/jpeg", "image/png"}, MaxReasonLength}
}
