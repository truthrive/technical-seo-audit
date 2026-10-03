package sitecrawl

// The Visualization tab's data: the link graph, capped to what a canvas can
// draw and a human can read.

// GraphNode is one page in the picture.
type GraphNode struct {
	ID       int64  `json:"id"`
	URL      string `json:"url"`
	Status   int    `json:"status"`
	Inlinks  int    `json:"inlinks"`
	Depth    int    `json:"depth"`
	Kind     string `json:"kind"`
	Internal bool   `json:"internal"`
}

// GraphEdge is one deduplicated link between two included nodes.
type GraphEdge struct {
	Src int64 `json:"src"`
	Dst int64 `json:"dst"`
}

type GraphData struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
	// Total is the run's full page count, so the UI can say
	// "showing top N of M by inlinks" instead of silently truncating.
	Total int `json:"total"`
}

const (
	graphDefaultNodes = 3000
	graphMaxEdges     = 15000
)

// Graph returns the top-N pages by inlinks plus every edge between them.
//
// Edges are scanned narrow and filtered in Go: an IN-list of 3000 ids twice
// over would blow SQLite's bind limit, while one pass over even 5M two-int
// rows is cheap and closes the cursor before anything else runs.
func (s *Service) Graph(runID string, maxNodes int) (GraphData, error) {
	db, err := s.readDB()
	if err != nil {
		return GraphData{}, err
	}
	out := GraphData{Nodes: []GraphNode{}, Edges: []GraphEdge{}}
	if runID == "" {
		return out, nil
	}
	if maxNodes <= 0 || maxNodes > graphDefaultNodes {
		maxNodes = graphDefaultNodes
	}

	db.QueryRow(`SELECT COUNT(*) FROM sitecrawl_pages WHERE run_id = ?`, runID).Scan(&out.Total)

	rows, err := db.Query(`
		SELECT url_id, url, status, inlinks, depth, kind, is_internal
		  FROM sitecrawl_pages WHERE run_id = ?
		 ORDER BY inlinks DESC, url_id LIMIT ?`, runID, maxNodes)
	if err != nil {
		return GraphData{}, err
	}
	included := make(map[int64]bool, maxNodes)
	for rows.Next() {
		var n GraphNode
		var internal int
		if err := rows.Scan(&n.ID, &n.URL, &n.Status, &n.Inlinks, &n.Depth, &n.Kind, &internal); err != nil {
			continue
		}
		n.Internal = internal == 1
		out.Nodes = append(out.Nodes, n)
		included[n.ID] = true
	}
	rows.Close()

	erows, err := db.Query(`SELECT src_id, dst_id FROM sitecrawl_links WHERE run_id = ?`, runID)
	if err != nil {
		return out, nil // nodes alone still draw
	}
	type pair struct{ s, d int64 }
	seen := map[pair]bool{}
	for erows.Next() {
		var src, dst int64
		if err := erows.Scan(&src, &dst); err != nil {
			continue
		}
		if src == dst || !included[src] || !included[dst] {
			continue
		}
		p := pair{src, dst}
		if seen[p] {
			continue
		}
		seen[p] = true
		out.Edges = append(out.Edges, GraphEdge{Src: src, Dst: dst})
		if len(out.Edges) >= graphMaxEdges {
			break
		}
	}
	erows.Close()
	return out, nil
}
