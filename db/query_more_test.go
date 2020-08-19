package db

import (
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	core "github.com/textileio/go-threads/core/db"
	"github.com/textileio/go-threads/util"
)

type book struct {
	ID     core.InstanceID `json:"_id"`
	Title  string
	Author string
	Meta   bookStats
}

type bookStats struct {
	TotalReads int
	Rating     float64
}

type queryTest struct {
	name    string
	query   *Query
	resIdx  []int // expected idx results from sample data
	ordered bool
}

var (
	sampleData = []book{
		{Title: "Title1", Author: "Author1", Meta: bookStats{TotalReads: 10, Rating: 3.3}},
		{Title: "Title2", Author: "Author1", Meta: bookStats{TotalReads: 20, Rating: 3.6}},
		{Title: "Title3", Author: "Author1", Meta: bookStats{TotalReads: 30, Rating: 3.9}},
		{Title: "Title4", Author: "Author2", Meta: bookStats{TotalReads: 114, Rating: 4.0}},
		{Title: "Title5", Author: "Author3", Meta: bookStats{TotalReads: 500, Rating: 4.8}},
	}

	queries = []queryTest{
		{name: "AllNil", query: nil, resIdx: []int{0, 1, 2, 3, 4}},
		{name: "AllExplicit", query: &Query{}, resIdx: []int{0, 1, 2, 3, 4}},

		{name: "FromAuthor1", query: Where("Author").Eq("Author1"), resIdx: []int{0, 1, 2}},
		{name: "FromAuthor2", query: Where("Author").Eq("Author2"), resIdx: []int{3}},
		{name: "FromAuthor3", query: Where("Author").Eq("Author3"), resIdx: []int{4}},

		{name: "AndAuthor1Title2", query: Where("Author").Eq("Author1").And("Title").Eq("Title2"), resIdx: []int{1}},
		{name: "AndAuthorNestedTotalReads", query: Where("Author").Eq("Author1").And("Meta.TotalReads").Eq(float64(10)), resIdx: []int{0}},

		{name: "OrAuthor", query: Where("Author").Eq("Author1").Or(Where("Author").Eq("Author3")), resIdx: []int{0, 1, 2, 4}},

		{name: "NeAuthor", query: Where("Author").Ne("Author1"), resIdx: []int{3, 4}},
		{name: "NeTotalReads", query: Where("Meta.TotalReads").Ne(float64(30)), resIdx: []int{0, 1, 3, 4}},
		{name: "NeRating", query: Where("Meta.Rating").Ne(3.6), resIdx: []int{0, 2, 3, 4}},

		{name: "GtAuthor", query: Where("Author").Gt("Author2"), resIdx: []int{4}},
		{name: "GtTotalReads", query: Where("Meta.TotalReads").Gt(float64(30)), resIdx: []int{3, 4}},
		{name: "GtRating", query: Where("Meta.Rating").Gt(3.6), resIdx: []int{2, 3, 4}},

		{name: "GeAuthor", query: Where("Author").Ge("Author2"), resIdx: []int{3, 4}},
		{name: "GeTotalReads", query: Where("Meta.TotalReads").Ge(float64(30)), resIdx: []int{2, 3, 4}},
		{name: "GeRating", query: Where("Meta.Rating").Ge(3.6), resIdx: []int{1, 2, 3, 4}},

		{name: "LtAuthor", query: Where("Author").Lt("Author2"), resIdx: []int{0, 1, 2}},
		{name: "LtTotalReads", query: Where("Meta.TotalReads").Lt(float64(30)), resIdx: []int{0, 1}},
		{name: "LtRating", query: Where("Meta.Rating").Lt(3.6), resIdx: []int{0}},

		{name: "LeAuthor", query: Where("Author").Le("Author2"), resIdx: []int{0, 1, 2, 3}},
		{name: "LeTotalReads", query: Where("Meta.TotalReads").Le(float64(30)), resIdx: []int{0, 1, 2}},
		{name: "LeRating", query: Where("Meta.Rating").Le(3.6), resIdx: []int{0, 1}},

		{name: "SortAscString", query: Where("Meta.TotalReads").Gt(float64(20)).OrderBy("Author"), resIdx: []int{2, 3, 4}, ordered: true},
		{name: "SortDescString", query: Where("Meta.TotalReads").Gt(float64(20)).OrderByDesc("Author"), resIdx: []int{4, 3, 2}, ordered: true},

		{name: "SortAscInt", query: Where("Meta.TotalReads").Gt(float64(10)).OrderBy("Meta.TotalReads"), resIdx: []int{1, 2, 3, 4}, ordered: true},
		{name: "SortDescInt", query: Where("Meta.TotalReads").Gt(float64(10)).OrderByDesc("Meta.TotalReads"), resIdx: []int{4, 3, 2, 1}, ordered: true},

		{name: "SortAscFloat", query: Where("Meta.TotalReads").Gt(float64(10)).OrderBy("Meta.Rating"), resIdx: []int{1, 2, 3, 4}, ordered: true},
		{name: "SortDescFloat", query: Where("Meta.TotalReads").Gt(float64(10)).OrderByDesc("Meta.Rating"), resIdx: []int{4, 3, 2, 1}, ordered: true},

		{name: "SortAllAscString", query: OrderBy("Title"), resIdx: []int{0, 1, 2, 3, 4}, ordered: true},
		{name: "SortAllDescString", query: OrderByDesc("Title"), resIdx: []int{4, 3, 2, 1, 0}, ordered: true},

		{name: "SortAllAscInt", query: OrderBy("Meta.TotalReads"), resIdx: []int{0, 1, 2, 3, 4}, ordered: true},
		{name: "SortAllDescInt", query: OrderByDesc("Meta.TotalReads"), resIdx: []int{4, 3, 2, 1, 0}, ordered: true},

		{name: "SortAllAscFloat", query: OrderBy("Meta.Rating"), resIdx: []int{0, 1, 2, 3, 4}, ordered: true},
		{name: "SortAllDescFloat", query: OrderByDesc("Meta.Rating"), resIdx: []int{4, 3, 2, 1, 0}, ordered: true},
	}
)

func TestCollectionQuery(t *testing.T) {
	t.Parallel()
	c, data, clean := createCollectionWithData(t)
	defer clean()
	for _, q := range queries {
		q := q
		t.Run(q.name, func(t *testing.T) {
			ret, err := c.Find(q.query)
			if err != nil {
				t.Fatalf("error when executing query: %v", err)
			}
			if len(q.resIdx) != len(ret) {
				t.Fatalf("query results length doesn't match, expected: %d, got: %d", len(q.resIdx), len(ret))
			}
			res := make([]*book, len(ret))
			for i, bookJSON := range ret {
				var book = &book{}
				util.InstanceFromJSON(bookJSON, book)
				res[i] = book
			}

			expectedIdx := make([]int, len(q.resIdx))
			for i := range q.resIdx {
				expectedIdx[i] = q.resIdx[i]
			}
			if !q.ordered {
				sort.Slice(res, func(i, j int) bool {
					return strings.Compare(res[i].ID.String(), res[j].ID.String()) == -1
				})
				sort.Slice(expectedIdx, func(i, j int) bool {
					return strings.Compare(data[expectedIdx[i]].ID.String(), data[expectedIdx[j]].ID.String()) == -1
				})
			}
			for i, idx := range expectedIdx {
				if !reflect.DeepEqual(data[idx], *res[i]) {
					t.Fatalf("wrong query item result, expected: %v, got: %v", data[idx], *res[i])
				}
			}
		})
	}
}

func TestInvalidSortField(t *testing.T) {
	t.Parallel()

	c, _, clean := createCollectionWithData(t)
	defer clean()
	_, err := c.Find(OrderBy("WrongFieldName"))
	if !errors.Is(err, ErrInvalidSortingField) {
		t.Fatal("query should fail using an invalid field")
	}
}

func createCollectionWithData(t *testing.T) (*Collection, []book, func()) {
	db, clean := createTestDB(t)
	c, err := db.NewCollection(CollectionConfig{
		Name:   "Book",
		Schema: util.SchemaFromInstance(&book{}, false),
	})
	checkErr(t, err)
	sampleDataCopy := make([]book, len(sampleData))
	for i := range sampleData {
		sampleDataJSON := util.JSONFromInstance(sampleData[i])
		id, err := c.Create(sampleDataJSON)
		if err != nil {
			t.Fatalf("failed to create sample data: %v", err)
		}
		sampleDataJSON = util.SetJSONID(id, sampleDataJSON)
		updated := book{}
		util.InstanceFromJSON(sampleDataJSON, &updated)
		sampleDataCopy[i] = updated
	}
	return c, sampleDataCopy, clean
}
