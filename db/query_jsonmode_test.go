package db

import (
	"sort"
	"strings"
	"testing"
)

const (
	testQueryJSONModeSchema = `{
		"$schema": "http://json-schema.org/draft-04/schema#",
		"$ref": "#/definitions/book",
		"definitions": {
		   "book": {
			  "required": [
				 "ID",
				 "Title",
				 "Author",
				 "Meta"
			  ],
			  "properties": {
				 "Author": {
					"type": "string"
				 },
				 "Banned": {
					"type": "boolean"
				 },
				 "ID": {
					"type": "string"
				 },
				 "Meta": {
					"$schema": "http://json-schema.org/draft-04/schema#",
					"$ref": "#/definitions/bookStats"
				 },
				 "Title": {
					"type": "string"
				 }
			  },
			  "additionalProperties": false,
			  "type": "object"
		   },
		   "bookStats": {
			  "required": [
				 "TotalReads",
				 "Rating"
			  ],
			  "properties": {
				 "Rating": {
					"type": "number"
				 },
				 "TotalReads": {
					"type": "integer"
				 }
			  },
			  "additionalProperties": false,
			  "type": "object"
		   }
		}
	 }`
)

type jsonQueryTest struct {
	name    string
	query   *JSONQuery
	resIdx  []int
	ordered bool
}

var (
	jsonSampleData = []string{
		`{"ID": "", "Title": "Title1", "Banned": true,  "Author": "Author1", "Meta": { "TotalReads": 100, "Rating": 3.2 }}`,
		`{"ID": "", "Title": "Title2", "Banned": false, "Author": "Author1", "Meta": { "TotalReads": 150, "Rating": 4.1 }}`,
		`{"ID": "", "Title": "Title3", "Banned": false, "Author": "Author2", "Meta": { "TotalReads": 120, "Rating": 4.6 }}`,
		`{"ID": "", "Title": "Title4", "Banned": true,  "Author": "Author3", "Meta": { "TotalReads": 1000, "Rating": 2.6 }}`,
	}

	boolTrue   = true
	boolFalse  = false
	title      = "Title"
	title0     = "Title1"
	title1     = "Title2"
	title3     = "Title3"
	titleMax   = "Title5000"
	totreadEq1 = 100.0
	totreadMid = 124.36
	totreadEq2 = 150.0
	totreadMax = 1221.25
	totreadMin = 12.22
	ratingMin  = 1.0
	rating1    = 3.2
	ratingMid  = 3.6
	ratingMax  = 5.0

	jsonQueries = []jsonQueryTest{
		// Ands string
		{
			name:   "EqByTitle1",
			resIdx: []int{0},
			query:  JSONWhere("Title").Eq(&title0),
		},
		{
			name:   "NeByTitle1",
			resIdx: []int{1, 2, 3},
			query:  JSONWhere("Title").Ne(&title0),
		},
		{
			name:   "NeByTitle",
			resIdx: []int{0, 1, 2, 3},
			query:  JSONWhere("Title").Ne(&title),
		},
		{
			name:   "EqByTitle",
			resIdx: []int{},
			query:  JSONWhere("Title").Eq(&title),
		},
		{
			name:   "LtByTitle",
			resIdx: []int{},
			query:  JSONWhere("Title").Lt(&title),
		},
		{
			name:   "GtByTitle",
			resIdx: []int{0, 1, 2, 3},
			query:  JSONWhere("Title").Gt(&title),
		},
		{
			name:   "GtByTitleMax",
			resIdx: []int{},
			query:  JSONWhere("Title").Gt(&titleMax),
		},

		// Ands "int" (which query interpret as float)
		{
			name:   "EqByTotalReads1",
			resIdx: []int{0},
			query:  JSONWhere("Meta.TotalReads").Eq(&totreadEq1),
		},
		{
			name:   "LtByTotalReads1",
			resIdx: []int{},
			query:  JSONWhere("Meta.TotalReads").Lt(&totreadEq1),
		},
		{
			name:   "LeByTotalReadsBigger",
			resIdx: []int{0, 1, 2, 3},
			query:  JSONWhere("Meta.TotalReads").Le(&totreadMax),
		},
		{
			name:   "GeByTotalReadsMin",
			resIdx: []int{0, 1, 2, 3},
			query:  JSONWhere("Meta.TotalReads").Ge(&totreadMin),
		},
		{
			name:   "LtByTotalReadsMidpoint",
			resIdx: []int{0, 2},
			query:  JSONWhere("Meta.TotalReads").Lt(&totreadMid),
		},
		{
			name:   "GtByTotalReadsMidpoint",
			resIdx: []int{1, 3},
			query:  JSONWhere("Meta.TotalReads").Gt(&totreadMid),
		},
		{
			name:   "LtByTotalReads2",
			resIdx: []int{0, 2},
			query:  JSONWhere("Meta.TotalReads").Lt(&totreadEq2),
		},

		// Ands float
		{
			name:   "EqByRating1",
			resIdx: []int{0},
			query:  JSONWhere("Meta.Rating").Eq(&rating1),
		},
		{
			name:   "GtByRating1",
			resIdx: []int{1, 2},
			query:  JSONWhere("Meta.Rating").Gt(&rating1),
		},
		{
			name:   "LeByRatingMin",
			resIdx: []int{},
			query:  JSONWhere("Meta.Rating").Le(&ratingMin),
		},
		{
			name:   "GeByRatingMin",
			resIdx: []int{0, 1, 2, 3},
			query:  JSONWhere("Meta.Rating").Ge(&ratingMin),
		},
		{
			name:   "LeByRatingMid",
			resIdx: []int{0, 3},
			query:  JSONWhere("Meta.Rating").Le(&ratingMid),
		},
		{
			name:   "GeByRatingMid",
			resIdx: []int{1, 2},
			query:  JSONWhere("Meta.Rating").Ge(&ratingMid),
		},
		{
			name:   "LeByRatingMax",
			resIdx: []int{0, 1, 2, 3},
			query:  JSONWhere("Meta.Rating").Le(&ratingMax),
		},
		{
			name:   "GtByRatingMax",
			resIdx: []int{},
			query:  JSONWhere("Meta.Rating").Gt(&ratingMax),
		},
		// Ands bool
		{
			name:   "EqByBanned",
			resIdx: []int{0, 3},
			query:  JSONWhere("Banned").Eq(&boolTrue),
		},
		{
			name:   "NeByBanned",
			resIdx: []int{1, 2},
			query:  JSONWhere("Banned").Ne(&boolTrue),
		},
		{
			name:   "EqByBannedFalse",
			resIdx: []int{1, 2},
			query:  JSONWhere("Banned").Eq(&boolFalse),
		},

		// Ors
		{
			name:   "EqTitle1OrTitle3",
			resIdx: []int{0, 2},
			query:  JSONWhere("Title").Eq(&title0).JSONOr(JSONWhere("Title").Eq(&title3)),
		},
		{
			name:   "EqTitle2OrRating",
			resIdx: []int{0, 1},
			query:  JSONWhere("Title").Eq(&title1).JSONOr(JSONWhere("Meta.Rating").Eq(&rating1)),
		},

		// Ordering (string, int, float)
		{
			name:    "AllOrderedTitle",
			resIdx:  []int{0, 1, 2, 3},
			query:   JSONOrderBy("Title"),
			ordered: true,
		},
		{
			name:    "AllOrderedTitleDesc",
			resIdx:  []int{3, 2, 1, 0},
			query:   JSONOrderByDesc("Title"),
			ordered: true,
		},
		{
			name:    "AllOrderedTotalReads",
			resIdx:  []int{0, 2, 1, 3},
			query:   JSONOrderBy("Meta.TotalReads"),
			ordered: true,
		},
		{
			name:    "AllOrderedTotalReads",
			resIdx:  []int{3, 1, 2, 0},
			query:   JSONOrderByDesc("Meta.TotalReads"),
			ordered: true,
		},
		{
			name:    "AllOrderedRatings",
			resIdx:  []int{3, 0, 1, 2},
			query:   JSONOrderBy("Meta.Rating"),
			ordered: true,
		},
		{
			name:    "AllOrderedRatingsDesc",
			resIdx:  []int{2, 1, 0, 3},
			query:   JSONOrderByDesc("Meta.Rating"),
			ordered: true,
		},
		{
			name:    "AllOrderedRatingsDescWithAnd",
			resIdx:  []int{2, 1},
			query:   JSONWhere("Meta.Rating").Gt(&ratingMid).JSONOrderBy("Meta.TotalReads"),
			ordered: true,
		},
		{
			name:    "AllOrderedRatingsDescWithAndDesc",
			resIdx:  []int{1, 2},
			query:   JSONWhere("Meta.Rating").Gt(&ratingMid).JSONOrderByDesc("Meta.TotalReads"),
			ordered: true,
		},
		// Indexing
		{
			name:   "EqTitle1OrTitle3UseIndex",
			resIdx: []int{0, 2},
			query:  JSONWhere("Title").Eq(&title0).JSONOr(JSONWhere("Title").Eq(&title3)).UseIndex("Title"),
		},
		{
			name:   "GeByTotalReadsMinUseIndex",
			resIdx: []int{0, 1, 2, 3},
			query:  JSONWhere("Meta.TotalReads").Ge(&totreadMin).UseIndex("Meta.TotalReads"),
		},
		{
			name:   "InvalidIndex",
			resIdx: []int{},
			query:  JSONWhere("Meta.TotalReads").Ge(&totreadMin).UseIndex("Not.Valid.Path"),
		},
	}
)

func TestQueryJsonMode(t *testing.T) {
	c, clean := createCollectionWithJSONData(t)
	defer clean()

	for _, q := range jsonQueries {
		q := q
		t.Run(q.name, func(t *testing.T) {
			t.Parallel()
			res, err := c.FindJSON(q.query)
			checkErr(t, err)
			if len(q.resIdx) != len(res) {
				t.Fatalf("query results length doesn't match, expected: %d, got: %d", len(q.resIdx), len(res))
			}
			expectedIdx := make([]int, len(q.resIdx))
			for i := range q.resIdx {
				expectedIdx[i] = q.resIdx[i]
			}
			if !q.ordered {
				sort.Slice(res, func(i, j int) bool {
					return strings.Compare(res[i], res[j]) == -1
				})
				sort.Slice(expectedIdx, func(i, j int) bool {
					return strings.Compare(jsonSampleData[expectedIdx[i]], jsonSampleData[expectedIdx[j]]) == -1
				})
			}
			for i, idx := range expectedIdx {
				if jsonSampleData[idx] != res[i] {
					t.Fatalf("wrong query item result, expected: %v, got: %v", jsonSampleData[idx], res[i])
				}
			}
		})
	}
}

func createCollectionWithJSONData(t *testing.T) (*Collection, func()) {
	s, clean := createTestDB(t, WithJsonMode(true))
	c, err := s.NewCollection("Book", testQueryJSONModeSchema,
		&IndexConfig{
			Path: "Meta.TotalReads",
		},
		&IndexConfig{
			Path: "Title",
		})
	checkErr(t, err)
	for i := range jsonSampleData {
		if err = c.Create(&jsonSampleData[i]); err != nil {
			t.Fatalf("failed to create sample data: %v", err)
		}
	}
	return c, clean
}
