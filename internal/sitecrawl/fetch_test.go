package sitecrawl

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type errAfterBytesReader struct {
	data []byte
	pos  int
	err  error
}

func (r *errAfterBytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, r.err
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	if r.pos >= len(r.data) {
		return n, r.err
	}
	return n, nil
}

// delayedProbeErrorReader returns data up to limit cleanly with nil error,
// then returns 1 byte on the (limit+1) probe with nil error, and fails during
// the subsequent post-parse drain step.
type drainErrorReader struct {
	body     []byte
	pos      int
	drainErr error
}

func (r *drainErrorReader) Read(p []byte) (int, error) {
	if r.pos < len(r.body) {
		n := copy(p, r.body[r.pos:])
		r.pos += n
		return n, nil
	}
	return 0, r.drainErr
}

func newTestHTMLResponse(rawURL string, status int, contentLength int64, body io.Reader) *http.Response {
	u, _ := url.Parse(rawURL)
	return &http.Response{
		StatusCode:    status,
		Header:        http.Header{"Content-Type": []string{"text/html; charset=utf-8"}},
		ContentLength: contentLength,
		Body:          io.NopCloser(body),
		Request:       &http.Request{URL: u},
	}
}

func TestReadTerminal_ExtractionCompletenessContract(t *testing.T) {
	t.Run("1. clean EOF well below limit sets ExtractionStatus complete", func(t *testing.T) {
		f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
		htmlBody := `<html><head><link rel="canonical" href="https://example.com/a"></head><body>ok</body></html>`
		res := newTestHTMLResponse("https://example.com/a", 200, int64(len(htmlBody)), strings.NewReader(htmlBody))
		out := fetched{FinalURL: "https://example.com/a", Status: 200}

		f.readTerminal(res, &out, true)

		if out.ExtractionStatus != ExtractionStatusComplete {
			t.Fatalf("ExtractionStatus = %q, want %q", out.ExtractionStatus, ExtractionStatusComplete)
		}
		if out.Doc == nil || out.Doc.CanonicalTotalCount != 1 {
			t.Fatalf("expected Doc with CanonicalTotalCount=1, got %+v", out.Doc)
		}
	})

	t.Run("2. exact limit-byte boundary with clean EOF on (limit+1) probe sets ExtractionStatus complete", func(t *testing.T) {
		// maxFileSize() for MaxFileSizeMB=1 is 1<<20 (1048576 bytes) < maxHTMLBytes (4<<20).
		f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
		limit := int(f.opts.maxFileSize())
		prefix := `<html><head><link rel="canonical" href="/exact"></head><body>`
		suffix := `</body></html>`
		padLen := limit - len(prefix) - len(suffix)
		exactBody := prefix + strings.Repeat("a", padLen) + suffix
		if len(exactBody) != limit {
			t.Fatalf("constructed body len = %d, want %d", len(exactBody), limit)
		}

		res := newTestHTMLResponse("https://example.com/exact", 200, -1, strings.NewReader(exactBody))
		out := fetched{FinalURL: "https://example.com/exact", Status: 200}

		f.readTerminal(res, &out, true)

		if out.ExtractionStatus != ExtractionStatusComplete {
			t.Fatalf("ExtractionStatus = %q, want %q at exact limit boundary", out.ExtractionStatus, ExtractionStatusComplete)
		}
		if out.Size != int64(limit) {
			t.Errorf("Size = %d, want %d", out.Size, limit)
		}
	})

	t.Run("3. stream with limit+1 bytes sets ExtractionStatus truncated_size_limit", func(t *testing.T) {
		f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
		limit := int(f.opts.maxFileSize())
		prefix := `<html><head><link rel="canonical" href="/trunc"></head><body>`
		suffix := `</body></html>`
		// 1 byte over limit
		overBody := prefix + strings.Repeat("b", limit-len(prefix)-len(suffix)+1) + suffix
		if len(overBody) != limit+1 {
			t.Fatalf("constructed body len = %d, want %d", len(overBody), limit+1)
		}

		res := newTestHTMLResponse("https://example.com/trunc", 200, -1, strings.NewReader(overBody))
		out := fetched{FinalURL: "https://example.com/trunc", Status: 200}

		f.readTerminal(res, &out, true)

		if out.ExtractionStatus != ExtractionStatusTruncatedSizeLimit {
			t.Fatalf("ExtractionStatus = %q, want %q", out.ExtractionStatus, ExtractionStatusTruncatedSizeLimit)
		}
		if out.Size != int64(limit+1) {
			t.Errorf("Size = %d, want %d", out.Size, limit+1)
		}
	})

	t.Run("4. pre-parse ContentLength > cap sets ExtractionStatus truncated_size_limit and ErrTooLarge", func(t *testing.T) {
		f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
		capBytes := f.opts.maxFileSize()
		res := newTestHTMLResponse("https://example.com/huge", 200, capBytes+100, strings.NewReader("<html></html>"))
		out := fetched{FinalURL: "https://example.com/huge", Status: 200}

		f.readTerminal(res, &out, true)

		if out.ErrorType != ErrTooLarge {
			t.Errorf("ErrorType = %q, want %q", out.ErrorType, ErrTooLarge)
		}
		if out.ExtractionStatus != ExtractionStatusTruncatedSizeLimit {
			t.Errorf("ExtractionStatus = %q, want %q", out.ExtractionStatus, ExtractionStatusTruncatedSizeLimit)
		}
	})

	t.Run("5. mid-stream read error sets ExtractionStatus read_error", func(t *testing.T) {
		f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
		partialHTML := []byte(`<html><head><title>Partial`)
		bodyReader := &errAfterBytesReader{
			data: partialHTML,
			err:  errors.New("simulated TCP reset mid-stream"),
		}
		res := newTestHTMLResponse("https://example.com/miderr", 200, -1, bodyReader)
		out := fetched{FinalURL: "https://example.com/miderr", Status: 200}

		f.readTerminal(res, &out, true)

		if out.ExtractionStatus != ExtractionStatusReadError {
			t.Fatalf("ExtractionStatus = %q, want %q", out.ExtractionStatus, ExtractionStatusReadError)
		}
	})

	t.Run("6. exact limit-byte stream where (limit+1) probe encounters read error sets ExtractionStatus read_error", func(t *testing.T) {
		f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
		limit := int(f.opts.maxFileSize())
		exactBytes := bytes.Repeat([]byte("x"), limit)
		bodyReader := &drainErrorReader{
			body:     exactBytes,
			drainErr: errors.New("connection reset at exact limit boundary"),
		}
		res := newTestHTMLResponse("https://example.com/boundaryerr", 200, -1, bodyReader)
		out := fetched{FinalURL: "https://example.com/boundaryerr", Status: 200}

		f.readTerminal(res, &out, true)

		if out.ExtractionStatus != ExtractionStatusReadError {
			t.Fatalf("ExtractionStatus = %q, want %q", out.ExtractionStatus, ExtractionStatusReadError)
		}
	})

	t.Run("7. stream exceeding limit where post-parse drain encounters error sets ExtractionStatus read_error", func(t *testing.T) {
		f := &fetcher{opts: Options{MaxFileSizeMB: 1}.normalized()}
		limit := int(f.opts.maxFileSize())
		overBytes := bytes.Repeat([]byte("y"), limit+10)
		bodyReader := &drainErrorReader{
			body:     overBytes,
			drainErr: errors.New("connection reset during post-parse drain"),
		}
		res := newTestHTMLResponse("https://example.com/drainerr", 200, -1, bodyReader)
		out := fetched{FinalURL: "https://example.com/drainerr", Status: 200}

		f.readTerminal(res, &out, true)

		if out.ExtractionStatus != ExtractionStatusReadError {
			t.Fatalf("ExtractionStatus = %q, want %q when drainErr != nil", out.ExtractionStatus, ExtractionStatusReadError)
		}
	})
}
