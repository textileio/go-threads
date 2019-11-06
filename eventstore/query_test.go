package eventstore

import (
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/textileio/go-textile-threads/core"
)

type book struct {
	ID     core.EntityID
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
		book{Title: "Title1", Author: "Author1", Meta: bookStats{TotalReads: 10, Rating: 3.3}},
		book{Title: "Title2", Author: "Author1", Meta: bookStats{TotalReads: 20, Rating: 3.6}},
		book{Title: "Title3", Author: "Author1", Meta: bookStats{TotalReads: 30, Rating: 3.9}},
		book{Title: "Title4", Author: "Author2", Meta: bookStats{TotalReads: 114, Rating: 4.0}},
		book{Title: "Title5", Author: "Author3", Meta: bookStats{TotalReads: 500, Rating: 4.8}},
	}

	queries = []queryTest{
		queryTest{name: "AllNil", query: nil, resIdx: []int{0, 1, 2, 3, 4}},
		queryTest{name: "AllExplicit", query: &Query{}, resIdx: []int{0, 1, 2, 3, 4}},

		queryTest{name: "FromAuthor1", query: Where("Author").Eq("Author1"), resIdx: []int{0, 1, 2}},
		queryTest{name: "FromAuthor2", query: Where("Author").Eq("Author2"), resIdx: []int{3}},
		queryTest{name: "FromAuthor3", query: Where("Author").Eq("Author3"), resIdx: []int{4}},

		queryTest{name: "AndAuthor1Title2", query: Where("Author").Eq("Author1").And("Title").Eq("Title2"), resIdx: []int{1}},
		queryTest{name: "AndAuthorNestedTotalReads", query: Where("Author").Eq("Author1").And("Meta.TotalReads").Eq(10), resIdx: []int{0}},

		queryTest{name: "OrAuthor", query: Where("Author").Eq("Author1").Or(Where("Author").Eq("Author3")), resIdx: []int{0, 1, 2, 4}},

		queryTest{name: "NeAuthor", query: Where("Author").Ne("Author1"), resIdx: []int{3, 4}},
		queryTest{name: "NeTotalReads", query: Where("Meta.TotalReads").Ne(30), resIdx: []int{0, 1, 3, 4}},
		queryTest{name: "NeRating", query: Where("Meta.Rating").Ne(3.6), resIdx: []int{0, 2, 3, 4}},

		queryTest{name: "GtAuthor", query: Where("Author").Gt("Author2"), resIdx: []int{4}},
		queryTest{name: "GtTotalReads", query: Where("Meta.TotalReads").Gt(30), resIdx: []int{3, 4}},
		queryTest{name: "GtRating", query: Where("Meta.Rating").Gt(3.6), resIdx: []int{2, 3, 4}},

		queryTest{name: "GeAuthor", query: Where("Author").Ge("Author2"), resIdx: []int{3, 4}},
		queryTest{name: "GeTotalReads", query: Where("Meta.TotalReads").Ge(30), resIdx: []int{2, 3, 4}},
		queryTest{name: "GeRating", query: Where("Meta.Rating").Ge(3.6), resIdx: []int{1, 2, 3, 4}},

		queryTest{name: "LtAuthor", query: Where("Author").Lt("Author2"), resIdx: []int{0, 1, 2}},
		queryTest{name: "LtTotalReads", query: Where("Meta.TotalReads").Lt(30), resIdx: []int{0, 1}},
		queryTest{name: "LtRating", query: Where("Meta.Rating").Lt(3.6), resIdx: []int{0}},

		queryTest{name: "LeAuthor", query: Where("Author").Le("Author2"), resIdx: []int{0, 1, 2, 3}},
		queryTest{name: "LeTotalReads", query: Where("Meta.TotalReads").Le(30), resIdx: []int{0, 1, 2}},
		queryTest{name: "LeRating", query: Where("Meta.Rating").Le(3.6), resIdx: []int{0, 1}},

		queryTest{name: "FnAuthor", query: Where("Author").Fn(func(value interface{}) (bool, error) {
			v := value.(string)
			return v == "Author1" || v == "Author2", nil
		}), resIdx: []int{0, 1, 2, 3}},
		queryTest{name: "FnTotalReads", query: Where("Meta.TotalReads").Fn(func(value interface{}) (bool, error) {
			v := value.(int)
			return v >= 20 && v < 500, nil
		}), resIdx: []int{1, 2, 3}},
		queryTest{name: "FnRating", query: Where("Meta.Rating").Fn(func(value interface{}) (bool, error) {
			v := value.(float64)
			return v >= 3.6 && v <= 4.0 && v != 3.9, nil
		}), resIdx: []int{1, 3}},

		queryTest{name: "SortAscString", query: Where("Meta.TotalReads").Gt(20).OrderBy("Author"), resIdx: []int{2, 3, 4}, ordered: true},
		queryTest{name: "SortDescString", query: Where("Meta.TotalReads").Gt(20).OrderByDesc("Author"), resIdx: []int{4, 3, 2}, ordered: true},

		queryTest{name: "SortAscInt", query: Where("Meta.TotalReads").Gt(10).OrderBy("Meta.TotalReads"), resIdx: []int{1, 2, 3, 4}, ordered: true},
		queryTest{name: "SortDescInt", query: Where("Meta.TotalReads").Gt(10).OrderByDesc("Meta.TotalReads"), resIdx: []int{4, 3, 2, 1}, ordered: true},

		queryTest{name: "SortAscFloat", query: Where("Meta.TotalReads").Gt(10).OrderBy("Meta.Rating"), resIdx: []int{1, 2, 3, 4}, ordered: true},
		queryTest{name: "SortDescFloat", query: Where("Meta.TotalReads").Gt(10).OrderByDesc("Meta.Rating"), resIdx: []int{4, 3, 2, 1}, ordered: true},
	}
)

func TestModelQuery(t *testing.T) {
	t.Parallel()
	m := createModelWithData(t)
	for _, q := range queries {
		q := q
		t.Run(q.name, func(t *testing.T) {
			t.Parallel()
			var res []*book
			if err := m.Find(&res, q.query); err != nil {
				t.Fatal("error when executing query")
			}
			if len(q.resIdx) != len(res) {
				t.Fatalf("query results length doesn't match, expected: %d, got: %d", len(q.resIdx), len(res))
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
					return strings.Compare(sampleData[expectedIdx[i]].ID.String(), sampleData[expectedIdx[j]].ID.String()) == -1
				})
			}
			for i, idx := range expectedIdx {
				if !reflect.DeepEqual(sampleData[idx], *res[i]) {
					t.Fatalf("wrong query item result, expected: %v, got: %v", sampleData[idx], *res[i])
				}
			}
		})
	}
}

func TestInvalidSortField(t *testing.T) {
	t.Parallel()
	m := createModelWithData(t)
	var res []*book
	if err := m.Find(&res, (&Query{}).OrderBy("WrongFieldName")); !errors.Is(err, ErrInvalidSortingField) {
		t.Fatal("query should fail using an invalid field")
	}
}

func createModelWithData(t *testing.T) *Model {
	store := createTestStore()
	m, err := store.Register("Book", &book{})
	checkErr(t, err)
	for i := range sampleData {
		if err = m.Create(&sampleData[i]); err != nil {
			t.Fatalf("failed to create sample data: %v", err)
		}
	}
	return m
}
