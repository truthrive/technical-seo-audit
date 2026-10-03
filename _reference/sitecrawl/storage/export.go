package sitecrawl

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"os"
	"strings"

	"onescout/desktop/internal/core/runs"
)

// Export formats.
const (
	FormatCSV  = "csv"
	FormatJSON = "json"
	FormatXML  = "xml"
)

// exportWindow is how many rows are read per query. Streaming from one live
// cursor would hold the connection for the whole export and freeze the crawl;
// paging in windows lets an export run while a crawl is still writing.
const exportWindow = 2000

// runs.BOMUTF8 makes Excel read Vietnamese correctly.

// Export writes the current view to a file the user picks.
//
// The query is the grid's own, so what lands in the file is exactly what is on
// screen — same tab, filter, search and sort.
func (s *Service) Export(format string, q RowQuery) (string, error) {
	if s.Files == nil {
		return "", errors.New("sitecrawl: no file picker")
	}
	format = strings.ToLower(strings.TrimSpace(format))
	var filterName, pattern string
	switch format {
	case FormatJSON:
		filterName, pattern = "JSON file", "*.json"
	case FormatXML:
		filterName, pattern = "XML file", "*.xml"
	default:
		format, filterName, pattern = FormatCSV, "CSV file", "*.csv"
	}

	name := "site-crawl-" + q.Tab + "." + format
	path, err := s.Files.SaveFile(name, filterName, pattern)
	if err != nil {
		return "", err
	}
	// An empty path means the dialog was cancelled — not an error.
	if path == "" {
		return "", nil
	}
	if !strings.HasSuffix(strings.ToLower(path), "."+format) {
		path += "." + format
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	w := bufio.NewWriterSize(f, 64<<10)

	switch format {
	case FormatJSON:
		err = s.exportJSON(w, q)
	case FormatXML:
		err = s.exportXML(w, q)
	default:
		err = s.exportCSV(w, q)
	}
	// Flush and Close are reported, not deferred: with a 64KB buffer the last
	// rows only exist in memory until Flush, so a full disk or a dropped network
	// drive silently truncated the file while the UI said "Exported".
	if flushErr := w.Flush(); err == nil {
		err = flushErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(path) // a truncated export is worse than none
		return "", err
	}
	return path, nil
}

// eachWindow walks the query in pages, closing the cursor between them.
func (s *Service) eachWindow(q RowQuery, fn func(rows []Row) error) error {
	q.WantTotal = false
	q.Limit = exportWindow
	for offset := 0; ; offset += exportWindow {
		q.Offset = offset
		page, err := s.Rows(q)
		if err != nil {
			return err
		}
		if len(page.Rows) == 0 {
			return nil
		}
		if err := fn(page.Rows); err != nil {
			return err
		}
		if len(page.Rows) < exportWindow {
			return nil
		}
	}
}

func (s *Service) exportCSV(w *bufio.Writer, q RowQuery) error {
	if _, err := w.Write(runs.BOMUTF8); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	cw.UseCRLF = true
	defer cw.Flush()

	if err := cw.Write(q.Cols); err != nil {
		return err
	}
	err := s.eachWindow(q, func(rows []Row) error {
		for _, r := range rows {
			out := make([]string, len(r.Cells))
			for i, c := range r.Cells {
				out[i] = runs.CSVGuard(c)
			}
			if err := cw.Write(out); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	})
	if err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

func (s *Service) exportJSON(w *bufio.Writer, q RowQuery) error {
	// Written by hand rather than marshalling one big slice: a 50k-row export
	// materialised in memory is exactly what the windowing exists to avoid.
	if _, err := w.WriteString(`{"columns":`); err != nil {
		return err
	}
	cols, err := json.Marshal(q.Cols)
	if err != nil {
		return err
	}
	w.Write(cols)
	w.WriteString(`,"rows":[`)

	firstRow := true
	err = s.eachWindow(q, func(rows []Row) error {
		for _, r := range rows {
			if !firstRow {
				w.WriteString(",")
			}
			firstRow = false
			obj := make(map[string]string, len(q.Cols))
			for i, c := range q.Cols {
				if i < len(r.Cells) {
					obj[c] = r.Cells[i]
				}
			}
			blob, err := json.Marshal(obj)
			if err != nil {
				return err
			}
			if _, err := w.Write(blob); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	_, err = w.WriteString("]}")
	return err
}

func (s *Service) exportXML(w *bufio.Writer, q RowQuery) error {
	if _, err := w.WriteString(xml.Header + "<sitecrawl>\n"); err != nil {
		return err
	}
	err := s.eachWindow(q, func(rows []Row) error {
		for _, r := range rows {
			w.WriteString("  <row>\n")
			for i, c := range q.Cols {
				if i >= len(r.Cells) {
					break
				}
				w.WriteString("    <" + xmlName(c) + ">")
				if err := xml.EscapeText(w, []byte(r.Cells[i])); err != nil {
					return err
				}
				w.WriteString("</" + xmlName(c) + ">\n")
			}
			w.WriteString("  </row>\n")
		}
		return nil
	})
	if err != nil {
		return err
	}
	_, err = w.WriteString("</sitecrawl>\n")
	return err
}

// xmlName keeps a column id usable as an element name.
func xmlName(col string) string {
	if col == "" {
		return "value"
	}
	var b strings.Builder
	for i, r := range col {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '_':
			b.WriteRune(r)
		case (r >= '0' && r <= '9') && i > 0:
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// csvGuard neutralises spreadsheet formula injection.
//
// A local copy of the other tools' helper: the tool packages stay independent
// of each other by design, so shared plumbing is duplicated rather than
// extracted. A crawled page title genuinely can start with "=" — it is user
// content from a third-party site, which is exactly the untrusted case.
