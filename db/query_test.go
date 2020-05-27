package db

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	logging "github.com/ipfs/go-log"
	"github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/util"
)

type Meta struct {
	TotalReads int
	Rating     float64
}

type Book struct {
	ID     db.InstanceID `json:"_id"`
	Title  string
	Banned bool
	Author string
	Meta   Meta
}

var (
	data = []Book{
		{Title: "Title1", Banned: true, Author: "Author1", Meta: Meta{TotalReads: 100, Rating: 3.2}},
		{Title: "Title2", Banned: false, Author: "Author1", Meta: Meta{TotalReads: 150, Rating: 4.1}},
		{Title: "Title3", Banned: false, Author: "Author2", Meta: Meta{TotalReads: 120, Rating: 4.6}},
		{Title: "Title4", Banned: true, Author: "Author3", Meta: Meta{TotalReads: 1000, Rating: 2.6}},
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

	jsonQueries = []queryTest{
		// Ands string
		{
			name:   "EqByTitle1",
			resIdx: []int{0},
			query:  Where("Title").Eq(&title0),
		},
		{
			name:   "NeByTitle1",
			resIdx: []int{1, 2, 3},
			query:  Where("Title").Ne(&title0),
		},
		{
			name:   "NeByTitle",
			resIdx: []int{0, 1, 2, 3},
			query:  Where("Title").Ne(&title),
		},
		{
			name:   "EqByTitle",
			resIdx: []int{},
			query:  Where("Title").Eq(&title),
		},
		{
			name:   "LtByTitle",
			resIdx: []int{},
			query:  Where("Title").Lt(&title),
		},
		{
			name:   "GtByTitle",
			resIdx: []int{0, 1, 2, 3},
			query:  Where("Title").Gt(&title),
		},
		{
			name:   "GtByTitleMax",
			resIdx: []int{},
			query:  Where("Title").Gt(&titleMax),
		},

		// Ands "int" (which query interpret as float)
		{
			name:   "EqByTotalReads1",
			resIdx: []int{0},
			query:  Where("Meta.TotalReads").Eq(&totreadEq1),
		},
		{
			name:   "LtByTotalReads1",
			resIdx: []int{},
			query:  Where("Meta.TotalReads").Lt(&totreadEq1),
		},
		{
			name:   "LeByTotalReadsBigger",
			resIdx: []int{0, 1, 2, 3},
			query:  Where("Meta.TotalReads").Le(&totreadMax),
		},
		{
			name:   "GeByTotalReadsMin",
			resIdx: []int{0, 1, 2, 3},
			query:  Where("Meta.TotalReads").Ge(&totreadMin),
		},
		{
			name:   "LtByTotalReadsMidpoint",
			resIdx: []int{0, 2},
			query:  Where("Meta.TotalReads").Lt(&totreadMid),
		},
		{
			name:   "GtByTotalReadsMidpoint",
			resIdx: []int{1, 3},
			query:  Where("Meta.TotalReads").Gt(&totreadMid),
		},
		{
			name:   "LtByTotalReads2",
			resIdx: []int{0, 2},
			query:  Where("Meta.TotalReads").Lt(&totreadEq2),
		},

		// Ands float
		{
			name:   "EqByRating1",
			resIdx: []int{0},
			query:  Where("Meta.Rating").Eq(&rating1),
		},
		{
			name:   "GtByRating1",
			resIdx: []int{1, 2},
			query:  Where("Meta.Rating").Gt(&rating1),
		},
		{
			name:   "LeByRatingMin",
			resIdx: []int{},
			query:  Where("Meta.Rating").Le(&ratingMin),
		},
		{
			name:   "GeByRatingMin",
			resIdx: []int{0, 1, 2, 3},
			query:  Where("Meta.Rating").Ge(&ratingMin),
		},
		{
			name:   "LeByRatingMid",
			resIdx: []int{0, 3},
			query:  Where("Meta.Rating").Le(&ratingMid),
		},
		{
			name:   "GeByRatingMid",
			resIdx: []int{1, 2},
			query:  Where("Meta.Rating").Ge(&ratingMid),
		},
		{
			name:   "LeByRatingMax",
			resIdx: []int{0, 1, 2, 3},
			query:  Where("Meta.Rating").Le(&ratingMax),
		},
		{
			name:   "GtByRatingMax",
			resIdx: []int{},
			query:  Where("Meta.Rating").Gt(&ratingMax),
		},
		// Ands bool
		{
			name:   "EqByBanned",
			resIdx: []int{0, 3},
			query:  Where("Banned").Eq(&boolTrue),
		},
		{
			name:   "NeByBanned",
			resIdx: []int{1, 2},
			query:  Where("Banned").Ne(&boolTrue),
		},
		{
			name:   "EqByBannedFalse",
			resIdx: []int{1, 2},
			query:  Where("Banned").Eq(&boolFalse),
		},

		// Ors
		{
			name:   "EqTitle1OrTitle3",
			resIdx: []int{0, 2},
			query:  Where("Title").Eq(&title0).Or(Where("Title").Eq(&title3)),
		},
		{
			name:   "EqTitle2OrRating",
			resIdx: []int{0, 1},
			query:  Where("Title").Eq(&title1).Or(Where("Meta.Rating").Eq(&rating1)),
		},

		// Ordering (string, int, float)
		{
			name:    "AllOrderedTitle",
			resIdx:  []int{0, 1, 2, 3},
			query:   OrderBy("Title"),
			ordered: true,
		},
		{
			name:    "AllOrderedTitleDesc",
			resIdx:  []int{3, 2, 1, 0},
			query:   OrderByDesc("Title"),
			ordered: true,
		},
		{
			name:    "AllOrderedTotalReads",
			resIdx:  []int{0, 2, 1, 3},
			query:   OrderBy("Meta.TotalReads"),
			ordered: true,
		},
		{
			name:    "AllOrderedTotalReads",
			resIdx:  []int{3, 1, 2, 0},
			query:   OrderByDesc("Meta.TotalReads"),
			ordered: true,
		},
		{
			name:    "AllOrderedRatings",
			resIdx:  []int{3, 0, 1, 2},
			query:   OrderBy("Meta.Rating"),
			ordered: true,
		},
		{
			name:    "AllOrderedRatingsDesc",
			resIdx:  []int{2, 1, 0, 3},
			query:   OrderByDesc("Meta.Rating"),
			ordered: true,
		},
		{
			name:    "AllOrderedRatingsDescWithAnd",
			resIdx:  []int{2, 1},
			query:   Where("Meta.Rating").Gt(&ratingMid).OrderBy("Meta.TotalReads"),
			ordered: true,
		},
		{
			name:    "AllOrderedRatingsDescWithAndDesc",
			resIdx:  []int{1, 2},
			query:   Where("Meta.Rating").Gt(&ratingMid).OrderByDesc("Meta.TotalReads"),
			ordered: true,
		},
		// Indexing
		{
			name:   "EqTitle1OrTitle3UseIndex",
			resIdx: []int{0, 2},
			query:  Where("Title").Eq(&title0).Or(Where("Title").Eq(&title3)).UseIndex("Title"),
		},
		{
			name:   "GeByTotalReadsMinUseIndex",
			resIdx: []int{0, 1, 2, 3},
			query:  Where("Meta.TotalReads").Ge(&totreadMin).UseIndex("Meta.TotalReads"),
		},
		{
			name:   "InvalidIndex",
			resIdx: []int{},
			query:  Where("Meta.TotalReads").Ge(&totreadMin).UseIndex("Not.Valid.Path"),
		},
	}
)

func TestQuery(t *testing.T) {
	logging.SetAllLoggers(logging.LevelError)
	c, d, clean := createCollectionWithJSONData(t)
	defer clean()

	for _, q := range jsonQueries {
		q := q
		t.Run(q.name, func(t *testing.T) {
			t.Parallel()
			res, err := c.Find(q.query)
			checkErr(t, err)
			if len(q.resIdx) != len(res) {
				t.Fatalf("query results length doesn't match, expected: %d, got: %d", len(q.resIdx), len(res))
			}

			foundBooks := make([]Book, len(res))
			for i := range res {
				book := Book{}
				util.InstanceFromJSON(res[i], &book)
				foundBooks[i] = book
			}

			expectedIdx := make([]int, len(q.resIdx))
			for i := range q.resIdx {
				expectedIdx[i] = q.resIdx[i]
			}

			if !q.ordered {
				sort.Slice(foundBooks, func(i, j int) bool {
					return strings.Compare(foundBooks[i].ID.String(), foundBooks[j].ID.String()) == -1
				})
				sort.Slice(expectedIdx, func(i, j int) bool {
					return strings.Compare(d[expectedIdx[i]].ID.String(), d[expectedIdx[j]].ID.String()) == -1
				})
			}
			for i, idx := range expectedIdx {
				if !reflect.DeepEqual(d[idx], foundBooks[i]) {
					t.Fatalf("wrong query item result, expected: %v, got: %v", d[idx], foundBooks[i])
				}
			}
		})
	}
}

func createCollectionWithJSONData(t *testing.T) (*Collection, []Book, func()) {
	s, clean := createTestDB(t)
	c, err := s.NewCollection(CollectionConfig{
		Name:   "Book",
		Schema: util.SchemaFromInstance(&Book{}, false),
		Indexes: []Index{
			{
				Path: "Meta.TotalReads",
			},
			{
				Path: "Title",
			}},
	})
	checkErr(t, err)
	dataCopy := make([]Book, len(data))
	for i := range data {
		id, err := c.Create(util.JSONFromInstance(data[i]))
		if err != nil {
			t.Fatalf("failed to create sample data: %v", err)
		}
		dataCopy[i] = data[i]
		dataCopy[i].ID = id
	}
	return c, dataCopy, clean
}
