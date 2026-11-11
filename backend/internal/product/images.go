package product

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Mrbaeksang/kch-relay/backend/internal/database"
	"github.com/Mrbaeksang/kch-relay/backend/internal/httpx"
	"github.com/go-chi/chi/v5"
)

var storageKeyPattern = regexp.MustCompile(`^[0-9a-f]{64}\.(jpg|png)$`)

type Image struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func imageURL(id string) string { return "/api/v1/images/" + id }

// AttachImagesTx replaces the photos of a product inside the caller's transaction.
// Every image must belong to sellerID and be unlinked or already linked to productID.
// It returns a VALIDATION_ERROR AppError for unusable input and never commits.
func AttachImagesTx(ctx context.Context, tx *sql.Tx, productID, sellerID string, imageIDs []string) error {
	if len(imageIDs) > MaxImagesPerProduct {
		return httpx.Invalid("사진은 최대 " + strconv.Itoa(MaxImagesPerProduct) + "장까지 등록할 수 있습니다.")
	}
	seen := map[string]bool{}
	for _, id := range imageIDs {
		if seen[id] {
			return httpx.Invalid("같은 사진을 두 번 등록할 수 없습니다.")
		}
		seen[id] = true
	}
	for _, id := range imageIDs {
		var ok bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM images WHERE id=? AND uploader_id=? AND (product_id IS NULL OR product_id=?))`, id, sellerID, productID).Scan(&ok)
		if err != nil {
			return err
		}
		if !ok {
			return httpx.Invalid("사용할 수 없는 사진이 포함되어 있습니다. 사진을 다시 올려 주세요.")
		}
	}
	if _, err := tx.ExecContext(ctx, `UPDATE images SET product_id=NULL,sort_order=0 WHERE product_id=?`, productID); err != nil {
		return err
	}
	for i, id := range imageIDs {
		if _, err := tx.ExecContext(ctx, `UPDATE images SET product_id=?,sort_order=? WHERE id=?`, productID, i, id); err != nil {
			return err
		}
	}
	return nil
}

func productImagesTx(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, productID string) ([]Image, error) {
	rows, err := q.QueryContext(ctx, `SELECT id FROM images WHERE product_id=? ORDER BY sort_order,id`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Image{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, Image{id, imageURL(id)})
	}
	return out, rows.Err()
}

// readUpload reads the single "file" part with a hard size limit.
func readUpload(r *http.Request) ([]byte, error) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "multipart/form-data" {
		return nil, httpx.Invalid("사진은 multipart/form-data로 올려 주세요.")
	}
	r.Body = http.MaxBytesReader(nil, r.Body, MaxImageBytes+64<<10)
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, httpx.Invalid("사진 요청을 확인해 주세요.")
	}
	tooBig := httpx.Invalid("사진은 " + strconv.FormatInt(MaxImageBytes>>20, 10) + "MB 이하만 올릴 수 있습니다.")
	for {
		part, err := reader.NextPart()
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				return nil, tooBig
			}
			return nil, httpx.Invalid("file 항목에 사진을 넣어 주세요.")
		}
		if part.FormName() != "file" {
			part.Close()
			continue
		}
		content, err := io.ReadAll(io.LimitReader(part, MaxImageBytes+1))
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) || int64(len(content)) > MaxImageBytes {
			return nil, tooBig
		}
		if err != nil {
			return nil, httpx.Invalid("사진을 읽지 못했습니다.")
		}
		return content, nil
	}
}

// sanitize decodes and re-encodes the picture. Re-encoding drops EXIF, comments
// and any non-image payload, so only plain pixels are stored.
func sanitize(content []byte) ([]byte, string, error) {
	unsupported := httpx.Invalid("JPEG 또는 PNG 사진만 올릴 수 있습니다.")
	cfg, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || (format != "jpeg" && format != "png") {
		return nil, "", unsupported
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > MaxImageSide || cfg.Height > MaxImageSide || cfg.Width*cfg.Height > MaxImagePixels {
		return nil, "", httpx.Invalid("사진 해상도가 너무 큽니다.")
	}
	img, _, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, "", unsupported
	}
	var out bytes.Buffer
	if format == "png" {
		err = png.Encode(&out, img)
		return out.Bytes(), "png", err
	}
	err = jpeg.Encode(&out, img, &jpeg.Options{Quality: 85})
	return out.Bytes(), "jpg", err
}

func (m *module) upload(r *http.Request) (httpx.Result, error) {
	// Serialize decoding to keep peak image memory bounded on the single small service.
	select {
	case m.uploadSlot <- struct{}{}:
		defer func() { <-m.uploadSlot }()
	case <-r.Context().Done():
		return httpx.Result{}, r.Context().Err()
	}
	actor, _ := httpx.CurrentUser(r.Context())
	content, err := readUpload(r)
	if err != nil {
		return httpx.Result{}, err
	}
	var pending int
	if err := m.db.QueryRowContext(r.Context(), `SELECT count(*) FROM images WHERE uploader_id=? AND product_id IS NULL`, actor.ID).Scan(&pending); err != nil {
		return httpx.Result{}, err
	}
	if pending >= MaxUnlinkedUploads {
		return httpx.Result{}, httpx.Error(409, "INVALID_STATE", "상품에 연결하지 않은 사진이 너무 많습니다. 기존 사진으로 상품을 등록해 주세요.")
	}
	clean, ext, err := sanitize(content)
	if err != nil {
		return httpx.Result{}, err
	}
	id, key := httpx.NewID(), httpx.NewID()+"."+ext
	final := filepath.Join(m.uploadDir, key)
	if err := writeFileAtomically(m.uploadDir, final, clean); err != nil {
		return httpx.Result{}, err
	}
	_, err = m.db.ExecContext(r.Context(), `INSERT INTO images(id,uploader_id,storage_key,created_at) VALUES(?,?,?,?)`, id, actor.ID, key, database.Timestamp(m.now()))
	if err != nil {
		// Compensate: the file must not outlive a failed DB insert.
		os.Remove(final)
		return httpx.Result{}, err
	}
	return httpx.Result{Status: 201, Data: Image{id, imageURL(id)}}, nil
}

func writeFileAtomically(dir, final string, content []byte) error {
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		return err
	}
	cleanup := func() { tmp.Close(); os.Remove(tmp.Name()) }
	if _, err := tmp.Write(content); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), final); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

// serveImage streams a stored photo after checking who may see it:
// unlinked uploads → uploader only; linked → anyone when the product is public,
// otherwise only the seller or HR_ADMIN. Everything else is 404.
func (m *module) serveImage(w http.ResponseWriter, r *http.Request) {
	actor, _ := httpx.CurrentUser(r.Context())
	notFound := httpx.Error(404, "NOT_FOUND", "사진을 찾을 수 없습니다.")
	var key, uploader string
	var productID, seller, status, deleted sql.NullString
	err := m.db.QueryRowContext(r.Context(), `SELECT i.storage_key,i.uploader_id,i.product_id,p.seller_id,p.approval_status,p.deleted_at FROM images i LEFT JOIN products p ON p.id=i.product_id WHERE i.id=?`, chi.URLParam(r, "imageID")).Scan(&key, &uploader, &productID, &seller, &status, &deleted)
	if errors.Is(err, sql.ErrNoRows) {
		httpx.WriteError(w, r, notFound)
		return
	}
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	allowed := false
	if !productID.Valid {
		allowed = uploader == actor.ID
	} else {
		allowed = (status.String == "APPROVED" && !deleted.Valid) || seller.String == actor.ID || actor.Role == "HR_ADMIN"
	}
	if !allowed || !storageKeyPattern.MatchString(key) {
		httpx.WriteError(w, r, notFound)
		return
	}
	file, err := os.Open(filepath.Join(m.uploadDir, key))
	if err != nil {
		httpx.WriteError(w, r, notFound)
		return
	}
	defer file.Close()
	contentType := "image/jpeg"
	if filepath.Ext(key) == ".png" {
		contentType = "image/png"
	}
	w.Header().Set("Content-Type", contentType)
	// no-cache: the browser must ask again every time, so a cached copy is never
	// shown to another account on the same browser. The ETag keeps repeats cheap (304).
	w.Header().Set("Cache-Control", "private, no-cache")
	w.Header().Set("ETag", `"`+strings.TrimSuffix(key, filepath.Ext(key))+`"`)
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Disposition", "inline")
	http.ServeContent(w, r, "", time.Time{}, file)
}
