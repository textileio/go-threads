package eventstore

import (
	"sort"
	"strings"
	"testing"
)

const (
	testQueryJsonModeSchema = `{
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
	query   JSONQuery
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
		jsonQueryTest{
			name:   "EqByTitle1",
			resIdx: []int{0},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Title",
						Operation: Eq,
						Value: JSONValue{
							String: &title0,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "NeByTitle1",
			resIdx: []int{1, 2, 3},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Title",
						Operation: Ne,
						Value: JSONValue{
							String: &title0,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "NeByTitle",
			resIdx: []int{0, 1, 2, 3},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Title",
						Operation: Ne,
						Value: JSONValue{
							String: &title,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "EqByTitle",
			resIdx: []int{},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Title",
						Operation: Eq,
						Value: JSONValue{
							String: &title,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "LtByTitle",
			resIdx: []int{},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Title",
						Operation: Lt,
						Value: JSONValue{
							String: &title,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "GtByTitle",
			resIdx: []int{0, 1, 2, 3},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Title",
						Operation: Gt,
						Value: JSONValue{
							String: &title,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "GtByTitleMax",
			resIdx: []int{},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Title",
						Operation: Gt,
						Value: JSONValue{
							String: &titleMax,
						},
					},
				},
			},
		},

		// Ands "int" (which query interpret as float)
		jsonQueryTest{
			name:   "EqByTotalReads1",
			resIdx: []int{0},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.TotalReads",
						Operation: Eq,
						Value: JSONValue{
							Float: &totreadEq1,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "LtByTotalReads1",
			resIdx: []int{},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.TotalReads",
						Operation: Lt,
						Value: JSONValue{
							Float: &totreadEq1,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "LeByTotalReadsBigger",
			resIdx: []int{0, 1, 2, 3},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.TotalReads",
						Operation: Le,
						Value: JSONValue{
							Float: &totreadMax,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "GeByTotalReadsMin",
			resIdx: []int{0, 1, 2, 3},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.TotalReads",
						Operation: Ge,
						Value: JSONValue{
							Float: &totreadMin,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "LtByTotalReadsMidpoint",
			resIdx: []int{0, 2},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.TotalReads",
						Operation: Lt,
						Value: JSONValue{
							Float: &totreadMid,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "GtByTotalReadsMidpoint",
			resIdx: []int{1, 3},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.TotalReads",
						Operation: Gt,
						Value: JSONValue{
							Float: &totreadMid,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "LtByTotalReads2",
			resIdx: []int{0, 2},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.TotalReads",
						Operation: Lt,
						Value: JSONValue{
							Float: &totreadEq2,
						},
					},
				},
			},
		},

		// Ands float
		jsonQueryTest{
			name:   "EqByRating1",
			resIdx: []int{0},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.Rating",
						Operation: Eq,
						Value: JSONValue{
							Float: &rating1,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "GtByRating1",
			resIdx: []int{1, 2},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.Rating",
						Operation: Gt,
						Value: JSONValue{
							Float: &rating1,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "LeByRatingMin",
			resIdx: []int{},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.Rating",
						Operation: Le,
						Value: JSONValue{
							Float: &ratingMin,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "GeByRatingMin",
			resIdx: []int{0, 1, 2, 3},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.Rating",
						Operation: Ge,
						Value: JSONValue{
							Float: &ratingMin,
						},
					},
				},
			},
		},

		jsonQueryTest{
			name:   "LeByRatingMid",
			resIdx: []int{0, 3},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.Rating",
						Operation: Le,
						Value: JSONValue{
							Float: &ratingMid,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "GeByRatingMid",
			resIdx: []int{1, 2},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.Rating",
						Operation: Ge,
						Value: JSONValue{
							Float: &ratingMid,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "LeByRatingMax",
			resIdx: []int{0, 1, 2, 3},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.Rating",
						Operation: Le,
						Value: JSONValue{
							Float: &ratingMax,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "GtByRatingMax",
			resIdx: []int{},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.Rating",
						Operation: Gt,
						Value: JSONValue{
							Float: &ratingMax,
						},
					},
				},
			},
		},
		// Ands bool
		jsonQueryTest{
			name:   "EqByBanned",
			resIdx: []int{0, 3},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Banned",
						Operation: Eq,
						Value: JSONValue{
							Bool: &boolTrue,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "NeByBanned",
			resIdx: []int{1, 2},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Banned",
						Operation: Ne,
						Value: JSONValue{
							Bool: &boolTrue,
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "EqByBannedFalse",
			resIdx: []int{1, 2},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Banned",
						Operation: Eq,
						Value: JSONValue{
							Bool: &boolFalse,
						},
					},
				},
			},
		},

		// Ors
		jsonQueryTest{
			name:   "EqTitle1OrTitle3",
			resIdx: []int{0, 2},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Title",
						Operation: Eq,
						Value: JSONValue{
							String: &title0,
						},
					},
				},
				Ors: []JSONQuery{
					JSONQuery{
						Ands: []JSONCriterion{
							JSONCriterion{
								FieldPath: "Title",
								Operation: Eq,
								Value: JSONValue{
									String: &title3,
								},
							},
						},
					},
				},
			},
		},
		jsonQueryTest{
			name:   "EqTitle2OrRating",
			resIdx: []int{0, 1},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Title",
						Operation: Eq,
						Value: JSONValue{
							String: &title1,
						},
					},
				},
				Ors: []JSONQuery{
					JSONQuery{
						Ands: []JSONCriterion{
							JSONCriterion{
								FieldPath: "Meta.Rating",
								Operation: Eq,
								Value: JSONValue{
									Float: &rating1,
								},
							},
						},
					},
				},
			},
		},

		// Ordering (string, int, float)
		jsonQueryTest{
			name:   "AllOrderedTitle",
			resIdx: []int{0, 1, 2, 3},
			query: JSONQuery{
				Sort: JSONSort{
					FieldPath: "Title",
				},
			},
			ordered: true,
		},
		jsonQueryTest{
			name:   "AllOrderedTitleDesc",
			resIdx: []int{3, 2, 1, 0},
			query: JSONQuery{
				Sort: JSONSort{
					FieldPath: "Title",
					Desc:      true,
				},
			},
			ordered: true,
		},
		jsonQueryTest{
			name:   "AllOrderedTotalReads",
			resIdx: []int{0, 2, 1, 3},
			query: JSONQuery{
				Sort: JSONSort{
					FieldPath: "Meta.TotalReads",
					Desc:      false,
				},
			},
			ordered: true,
		},
		jsonQueryTest{
			name:   "AllOrderedTotalReads",
			resIdx: []int{3, 1, 2, 0},
			query: JSONQuery{
				Sort: JSONSort{
					FieldPath: "Meta.TotalReads",
					Desc:      true,
				},
			},
			ordered: true,
		},
		jsonQueryTest{
			name:   "AllOrderedRatings",
			resIdx: []int{3, 0, 1, 2},
			query: JSONQuery{
				Sort: JSONSort{
					FieldPath: "Meta.Rating",
					Desc:      false,
				},
			},
			ordered: true,
		},
		jsonQueryTest{
			name:   "AllOrderedRatingsDesc",
			resIdx: []int{2, 1, 0, 3},
			query: JSONQuery{
				Sort: JSONSort{
					FieldPath: "Meta.Rating",
					Desc:      true,
				},
			},
			ordered: true,
		},
		jsonQueryTest{
			name:   "AllOrderedRatingsDescWithAnd",
			resIdx: []int{2, 1},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.Rating",
						Operation: Gt,
						Value: JSONValue{
							Float: &ratingMid,
						},
					},
				},
				Sort: JSONSort{
					FieldPath: "Meta.TotalReads",
					Desc:      false,
				},
			},
			ordered: true,
		},
		jsonQueryTest{
			name:   "AllOrderedRatingsDescWithAndDesc",
			resIdx: []int{1, 2},
			query: JSONQuery{
				Ands: []JSONCriterion{
					JSONCriterion{
						FieldPath: "Meta.Rating",
						Operation: Gt,
						Value: JSONValue{
							Float: &ratingMid,
						},
					},
				},
				Sort: JSONSort{
					FieldPath: "Meta.TotalReads",
					Desc:      true,
				},
			},
			ordered: true,
		},
	}
)

func TestQueryJsonMode(t *testing.T) {
	m, clean := createModelWithJsonData(t)
	defer clean()

	for _, q := range jsonQueries {
		q := q
		t.Run(q.name, func(t *testing.T) {
			t.Parallel()
			res, err := m.FindJSON(q.query)
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

func createModelWithJsonData(t *testing.T) (*Model, func()) {
	s, clean := createTestStore(t, WithJsonMode(true))
	m, err := s.RegisterSchema("Book", testQueryJsonModeSchema)
	checkErr(t, err)
	for i := range jsonSampleData {
		if err = m.Create(&jsonSampleData[i]); err != nil {
			t.Fatalf("failed to create sample data: %v", err)
		}
	}
	return m, clean
}
