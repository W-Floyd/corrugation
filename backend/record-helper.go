package backend

import (
	"bytes"
	"context"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/components"
	"github.com/go-echarts/go-echarts/v2/opts"
	"github.com/nao1215/markdown/mermaid/flowchart"
	"gorm.io/gorm"
)

type RecordQuery struct {
	Query               string
	SearchImages        bool
	SearchTextEmbedded  bool
	SearchTextSubstring bool
	MinTextToImageScore float64
	MinTextScore        float64
	ChildrenDepth       int
	ParentDepth         int
}

func NewRecordQuery(query string) RecordQuery {
	return RecordQuery{
		Query:               query,
		MinTextToImageScore: minimumTextToImageSearchConfidence,
		MinTextScore:        minimumTextSearchConfidence,
	}
}

func GetRecords(ctx context.Context, ID *uint, childrenDepth *int, parentDepth *int, search *RecordQuery, preload []struct {
	q string
	h func(db gorm.PreloadBuilder) error
}, selects []string) (records []Record, partial bool, err error) {
	username := UsernameFromContext(ctx)
	authed := username != ""
	var user User
	if authed {
		user, err = loadUser(username)
		if err != nil {
			return nil, false, err
		}
	}
	if ID == nil {
		if childrenDepth != nil {
			err = errors.New("childrenDepth provided without an ID")
			return
		}

		q := gorm.G[Record](db)
		var v gorm.ChainInterface[Record]
		if len(selects) > 1 {
			v = q.Select(selects[0], selects[1:])
		} else if len(selects) == 1 {
			v = q.Select(selects[0])
		}
		for _, s := range preload {
			if v != nil {
				v = v.Preload(s.q, s.h)
			} else {
				v = q.Preload(s.q, s.h)
			}
		}
		if authed {
			if v != nil {
				v = v.Where("owner_id = ?", user.ID)
			} else {
				v = q.Where("owner_id = ?", user.ID)
			}
		}
		if v != nil {
			records, err = v.Find(dbCtx)
		} else {
			records, err = q.Find(dbCtx)
		}

		if err != nil {
			return
		}

	} else if *ID == 0 {
		// Top-level: records with no parent
		q := gorm.G[Record](db)
		var v gorm.ChainInterface[Record]
		if len(selects) > 1 {
			v = q.Select(selects[0], selects[1:])
		} else if len(selects) == 1 {
			v = q.Select(selects[0])
		}
		for _, s := range preload {
			if v != nil {
				v = v.Preload(s.q, s.h)
			} else {
				v = q.Preload(s.q, s.h)
			}
		}
		if authed {
			if v != nil {
				v = v.Where("owner_id = ?", user.ID)
			} else {
				v = q.Where("owner_id = ?", user.ID)
			}
		}
		if v != nil {
			records, err = v.Where("parent_id IS NULL").Find(dbCtx)
		} else {
			records, err = q.Where("parent_id IS NULL").Find(dbCtx)
		}
		if err != nil {
			return
		}
		if childrenDepth != nil {
			for _, r := range records {
				var sub []*Record
				sub, err = GetChildrenRecurse(r.ID, *childrenDepth, 1)
				if err != nil {
					return
				}
				for _, s := range sub {
					records = append(records, *s)
				}
			}
		}

	} else {

		var recordsSearched []Record // This should come back with one value...
		q := gorm.G[Record](db)
		var v gorm.ChainInterface[Record]
		if len(selects) > 1 {
			v = q.Select(selects[0], selects[1:])
		} else if len(selects) == 1 {
			v = q.Select(selects[0])
		}
		for _, s := range preload {
			if v != nil {
				v = v.Preload(s.q, s.h)
			} else {
				v = q.Preload(s.q, s.h)
			}
		}
		if authed {
			if v != nil {
				v = v.Where("owner_id = ?", user.ID)
			} else {
				v = q.Where("owner_id = ?", user.ID)
			}
		}
		if v != nil {
			recordsSearched, err = v.Where("id = ?", *ID).Find(dbCtx)
		} else {
			recordsSearched, err = q.Where("id = ?", *ID).Find(dbCtx)
		}
		if err != nil {
			return
		}
		if len(recordsSearched) == 0 {
			err = huma.Error404NotFound(errorRecordNotFound + " " + strconv.Itoa(int(*ID)))
		}
		records = append(records, recordsSearched...)

		if childrenDepth != nil {
			var recordPtrs []*Record
			recordPtrs, err = GetChildrenRecurse(*ID, *childrenDepth, 0)

			for _, record := range recordPtrs {
				records = append(records, *record)
			}
		}

		if parentDepth != nil {
			parentSearchCurrentDepth := 0
			searchID := recordsSearched[0].ParentID
			for {
				if searchID == nil {
					break
				}
				parentSearchCurrentDepth += 1
				if *parentDepth > 0 && parentSearchCurrentDepth > *parentDepth {
					break
				} else if parentSearchCurrentDepth > maxSearchDepth {
					err = errors.New("exceeded max search depth on parent")
					return
				}

				var recordsSearched []Record
				recordsSearched, err = gorm.G[Record](db).Where("id = ?", *searchID).Find(dbCtx)
				if err != nil {
					return
				}
				if len(recordsSearched) > 0 {
					records = append(records, recordsSearched...)
					searchID = recordsSearched[0].ParentID
				} else {
					err = errors.New("found no record for " + strconv.FormatUint(uint64(*searchID), 10))
					return
				}

			}
		}

	}

	if search != nil && search.Query != "" {
		scopedRecordIDs := make([]uint, 0, len(records))
		artifactRecordMap := map[uint]*uint{}
		for _, r := range records {
			scopedRecordIDs = append(scopedRecordIDs, r.ID)
			for _, a := range r.Artifacts {
				if a != nil {
					artifactRecordMap[a.ID] = a.RecordID
				}
			}
		}

		var artifactSearch, recordSearch []struct {
			id    uint
			score float64
		}
		var artifactErr, recordErr error
		var artifactPartial, recordPartial bool
		var wg sync.WaitGroup
		if search.SearchImages {
			wg.Go(func() {
				artifactSearch, artifactPartial, artifactErr = SearchByArtifact(ctx, search.Query, artifactRecordMap)
			})
		}
		if search.SearchTextEmbedded {
			wg.Go(func() {
				recordSearch, recordPartial, recordErr = SearchByRecord(ctx, search.Query, scopedRecordIDs)
			})
		}
		wg.Wait()
		if artifactErr != nil {
			err = artifactErr
			return
		}
		if recordErr != nil {
			err = recordErr
			return
		}
		partial = artifactPartial || recordPartial

		textScore := map[uint]float64{}
		bestImageScore := map[uint]float64{}
		bestScore := map[uint]float64{}

		for _, r := range artifactSearch {
			score, ok := bestImageScore[r.id]
			if !ok || r.score > score {
				bestImageScore[r.id] = r.score
				if bestImageScore[r.id] > bestScore[r.id] {
					bestScore[r.id] = bestImageScore[r.id]
				}
			}
		}

		for _, r := range recordSearch {
			textScore[r.id] = r.score
			if textScore[r.id] > bestScore[r.id] {
				bestScore[r.id] = textScore[r.id]
			}
		}

		searchLower := strings.ToLower(search.Query)
		for _, r := range records {
			if r.ReferenceNumber != nil && search.Query == *r.ReferenceNumber {
				textScore[r.ID] = 1.0
				bestScore[r.ID] = 1.0
				continue
			}
			if !search.SearchTextSubstring {
				continue
			}
			score := maxFieldScore(searchLower, r.Title, r.ReferenceNumber, r.Description)
			if score > textScore[r.ID] {
				textScore[r.ID] = score
			}
			if textScore[r.ID] > bestScore[r.ID] {
				bestScore[r.ID] = textScore[r.ID]
			}
		}

		var recordMap = map[uint]*Record{}
		for _, r := range records {
			recordMap[r.ID] = &r
		}

		recordIDs := []uint{}
		for id := range bestScore {
			if bestImageScore[id] >= search.MinTextToImageScore || textScore[id] >= search.MinTextScore {
				recordIDs = append(recordIDs, id)
			}
		}

		slices.Sort(recordIDs)
		recordIDs = slices.Compact(recordIDs)

		avgScore := func(id uint) float64 {
			img, txt := bestImageScore[id], textScore[id]
			switch {
			case img > 0 && txt > 0:
				return (img + txt) / 2.0
			case img > 0:
				return img
			default:
				return txt
			}
		}

		slices.SortFunc(recordIDs, func(a uint, b uint) int {
			sa, sb := avgScore(a), avgScore(b)
			if sa > sb {
				return -1
			} else if sa < sb {
				return 1
			}
			return 0
		})

		var filteredSortedRecords []Record

		for _, rid := range recordIDs {
			r, ok := recordMap[rid]
			if ok && r != nil {
				imageScore := bestImageScore[rid]
				textScore := textScore[rid]
				r.SearchConfidenceImage = &imageScore
				r.SearchConfidenceText = &textScore
				filteredSortedRecords = append(filteredSortedRecords, *r)
			}
		}

		records = filteredSortedRecords

	}

	return

}

func GetChildrenRecurse(parentID uint, searchDepth int, currentDepth int) (records []*Record, err error) {
	if currentDepth > maxSearchDepth {
		err = errors.New("exceeded max search depth on children")
		return
	} else if searchDepth > 0 && currentDepth >= searchDepth {
		return
	}

	maxDepth := searchDepth - currentDepth
	if maxDepth <= 0 {
		maxDepth = maxSearchDepth
	}
	cteSQL := `
WITH RECURSIVE children AS (
	SELECT r.*, 1 as depth
	FROM records r
	WHERE r.parent_id = ?
	UNION ALL
	SELECT r.*, c.depth + 1
	FROM records r
	INNER JOIN children c ON r.parent_id = c.id
	WHERE c.depth < ?
)
SELECT * FROM children ORDER BY depth, id
`
	err = db.Raw(cteSQL, parentID, maxDepth).Scan(&records).Error

	if err != nil {
		return
	}

	if len(records) > 0 {
		var recordPtrs []*Record
		for i := range records {
			recordPtrs = append(recordPtrs, records[i])
		}
		records = recordPtrs
	}

	return

}

func GetRecordsGraphFriendly(ctx context.Context, inputID uint, inputChildrenDepth int, inputParentDepth int) (graphOutput string, err error) {
	var records []Record
	var childrenDepth, parentDepth *int
	if inputChildrenDepth != 0 {
		childrenDepth = &inputChildrenDepth
	}
	if inputParentDepth != 0 {
		parentDepth = &inputParentDepth
	}
	records, _, err = GetRecords(ctx, &inputID, childrenDepth, parentDepth, nil, []struct {
		q string
		h func(db gorm.PreloadBuilder) error
	}{
		{q: "Artifacts", h: func(db gorm.PreloadBuilder) error { db.Select("id", "record_id"); return nil }},
	}, nil)

	recordMap := make(map[uint]*Record)

	fc := flowchart.NewFlowchart(
		io.Discard,
		flowchart.WithTitle("mermaid flowchart builder"),
		flowchart.WithOrientalTopToBottom(),
	)

	for _, record := range records {
		recordMap[record.ID] = &record
	}

	for _, record := range records {
		if record.ParentID != nil {
			if _, ok := recordMap[*record.ParentID]; ok {
				fc.LinkWithArrowHead(
					recordMap[*record.ParentID].PrettyString(),
					recordMap[record.ID].PrettyString(),
				)
			}
		}
	}

	graphOutput = fc.String()
	return
}

func GetRecordsGraphFriendlyNative(ctx context.Context, inputID uint, inputChildrenDepth int, inputParentDepth int) (graphOutput string, err error) {
	var records []Record
	var childrenDepth, parentDepth *int
	if inputChildrenDepth != 0 {
		childrenDepth = &inputChildrenDepth
	}
	if inputParentDepth != 0 {
		parentDepth = &inputParentDepth
	}
	records, _, err = GetRecords(ctx, &inputID, childrenDepth, parentDepth, nil, []struct {
		q string
		h func(db gorm.PreloadBuilder) error
	}{
		{q: "Artifacts", h: func(db gorm.PreloadBuilder) error { db.Select("id", "record_id"); return nil }},
	}, nil)
	if err != nil {
		return
	}
	recordMap := make(map[uint]*Record)
	childrenMap := make(map[uint][]uint)
	topLevel := []uint{}

	for _, record := range records {
		recordMap[record.ID] = &record

	}

	for _, record := range records {
		if record.ParentID != nil {
			if _, ok := recordMap[*record.ParentID]; ok {
				childrenMap[*record.ParentID] = append(childrenMap[*record.ParentID], record.ID)
			}
		}
	}

	for _, record := range records {
		if record.ParentID == nil {
			topLevel = append(topLevel, record.ID)
		}
	}

	children := []*opts.TreeData{}

	for _, tl := range topLevel {
		children = append(children, DescendTreeMap(tl, recordMap, childrenMap))
	}

	page := components.NewPage()
	page.AddCharts(
		treeBase(children),
	)

	b := bytes.NewBuffer([]byte{})

	err = page.Render(io.MultiWriter(b))
	if err != nil {
		return
	}
	graphOutput = b.String()
	return
}

func DescendTreeMap(
	rootID uint,
	recordMap map[uint]*Record,
	childrenMap map[uint][]uint,
) (output *opts.TreeData) {
	output = &opts.TreeData{
		Name: recordMap[rootID].PrettyString(),
		Children: func() (out []*opts.TreeData) {
			for _, child := range childrenMap[rootID] {
				out = append(out, DescendTreeMap(child, recordMap, childrenMap))
			}
			return
		}(),
	}
	return
}
func treeBase(treenodes []*opts.TreeData) *charts.Tree {
	graph := charts.NewTree()
	graph.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Width: "100%", Height: "95vh"}),
		//charts.WithTooltipOpts(opts.Tooltip{Show: false}),
	)
	var tree *charts.Tree

	directTreeNodes := []opts.TreeData{}

	for _, node := range treenodes {
		directTreeNodes = append(directTreeNodes, *node)
	}

	if len(directTreeNodes) == 1 {
		tree = graph.AddSeries("tree", directTreeNodes)
	} else {
		tree = graph.AddSeries("tree", []opts.TreeData{
			{
				Name:     topLevelName,
				Children: treenodes,
			},
		})
	}

	tree.
		SetSeriesOptions(
			charts.WithTreeOpts(
				opts.TreeChart{
					Layout:           "orthogonal",
					Orient:           "LR",
					InitialTreeDepth: -1,
					Leaves: &opts.TreeLeaves{
						Label: &opts.Label{Show: opts.Bool(true), Position: "right", Color: "Black"},
					},
				},
			),
			charts.WithLabelOpts(opts.Label{Show: opts.Bool(true), Position: "top", Color: "Black"}),
		)
	return graph
}

func fieldScore(field *string, searchLower string) float64 {
	if field == nil {
		return 0
	}
	fieldLower := strings.ToLower(*field)
	// Whole string match
	if fieldLower == searchLower {
		return 0.99
	}
	// Whole word match
	if slices.Contains(strings.Fields(fieldLower), searchLower) {
		return 0.98
	}
	// Substring match
	if strings.Contains(fieldLower, searchLower) {
		return 0.97
	}
	return 0
}

func maxFieldScore(searchLower string, fields ...*string) float64 {
	var best float64
	for _, f := range fields {
		if s := fieldScore(f, searchLower); s > best {
			best = s
		}
	}
	return best
}
