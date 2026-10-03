package sitecrawl

// Opportunities: the "what do I actually fix?" half of PageSpeed.
//
// A score tells the user a page is slow; an opportunity tells them which image
// to compress and how many kilobytes it buys. Screaming Frog gives these their
// own tab because the useful view is site-wide — one audit failing on 62 pages
// is one afternoon's work, not 62 separate problems — so the list aggregates by
// audit and drills down to the URLs behind it.

// Opportunity is one audit rolled up across the whole run.
type Opportunity struct {
	AuditID string `json:"auditId"`
	Title   string `json:"title"`
	// Pages is how many URLs this audit fires on. Zero at the per-URL level,
	// filled only by the aggregate query.
	Pages        int   `json:"pages"`
	SavingsMs    int64 `json:"savingsMs"`
	SavingsBytes int64 `json:"savingsBytes"`
}

// OpportunityPage is one URL behind an audit.
type OpportunityPage struct {
	URL          string `json:"url"`
	SavingsMs    int64  `json:"savingsMs"`
	SavingsBytes int64  `json:"savingsBytes"`
}

// oppPagesLimit bounds the drill-down. A 50k crawl can have every page failing
// the same audit, and the list is for deciding what to fix, not for export.
const oppPagesLimit = 1000

// Opportunities lists every audit in the run, heaviest first.
func (s *Service) Opportunities(runID string) ([]Opportunity, error) {
	out := []Opportunity{}
	db, err := s.readDB()
	if err != nil {
		return nil, err
	}
	if runID == "" {
		return out, nil
	}
	// MAX(title) rather than a GROUP BY on it: Lighthouse occasionally words the
	// same audit differently between runs, and grouping on the text would split
	// one audit into two rows.
	rows, err := db.Query(`
		SELECT audit_id, MAX(title), COUNT(*), SUM(savings_ms), SUM(savings_bytes)
		  FROM sitecrawl_psi_opps WHERE run_id = ?
		 GROUP BY audit_id
		 ORDER BY SUM(savings_ms) DESC, COUNT(*) DESC`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var o Opportunity
		if err := rows.Scan(&o.AuditID, &o.Title, &o.Pages, &o.SavingsMs, &o.SavingsBytes); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// OpportunityPages lists the URLs one audit fires on, worst first.
func (s *Service) OpportunityPages(runID, auditID string) ([]OpportunityPage, error) {
	out := []OpportunityPage{}
	db, err := s.readDB()
	if err != nil {
		return nil, err
	}
	if runID == "" || auditID == "" {
		return out, nil
	}
	rows, err := db.Query(`
		SELECT url, savings_ms, savings_bytes FROM sitecrawl_psi_opps
		 WHERE run_id = ? AND audit_id = ?
		 ORDER BY savings_ms DESC, savings_bytes DESC
		 LIMIT ?`, runID, auditID, oppPagesLimit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p OpportunityPage
		if err := rows.Scan(&p.URL, &p.SavingsMs, &p.SavingsBytes); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
