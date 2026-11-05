package httpx

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strconv"
	"time"
)

type Cursor struct {
	CreatedAt string `json:"createdAt"`
	ID        string `json:"id"`
}
type Page struct {
	Limit int
	After *Cursor
}

func ParsePage(q url.Values) (Page, error) {
	p := Page{Limit: 20}
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > 100 {
			return p, Invalid("limit은 1~100이어야 합니다.")
		}
		p.Limit = n
	}
	if s := q.Get("cursor"); s != "" {
		if len(s) > 2048 {
			return p, Invalid("cursor가 올바르지 않습니다.")
		}
		data, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			return p, Invalid("cursor가 올바르지 않습니다.")
		}
		var c Cursor
		if json.Unmarshal(data, &c) != nil || c.ID == "" {
			return p, Invalid("cursor가 올바르지 않습니다.")
		}
		if _, err := time.Parse(time.RFC3339Nano, c.CreatedAt); err != nil {
			return p, Invalid("cursor 시각이 올바르지 않습니다.")
		}
		p.After = &c
	}
	return p, nil
}
func EncodeCursor(c Cursor) string {
	data, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(data)
}
func ParseDateRange(from, to string) (time.Time, time.Time, error) {
	zone := time.FixedZone("Asia/Seoul", 9*60*60)
	start, err := time.ParseInLocation("2006-01-02", from, zone)
	if err != nil {
		return time.Time{}, time.Time{}, Invalid("시작일이 올바르지 않습니다.")
	}
	end, err := time.ParseInLocation("2006-01-02", to, zone)
	if err != nil || end.Before(start) {
		return time.Time{}, time.Time{}, Invalid("종료일이 올바르지 않습니다.")
	}
	return start.UTC(), end.AddDate(0, 0, 1).UTC(), nil
}
