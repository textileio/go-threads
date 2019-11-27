package eventstore

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	dsquery "github.com/ipfs/go-datastore/query"
)

type JSONQuery struct {
	Ands []JSONCriterion
	Ors  []JSONQuery
	Sort JSONSort
}
type JSONCriterion struct {
	FieldPath string
	Operation JSONOperation
	Value     JSONValue
}

type JSONValue struct {
	String *string
	Bool   *bool
	Float  *float64
}

type JSONSort struct {
	FieldPath string
	Desc      bool
}

type JSONOperation int

const (
	Eq = JSONOperation(eq)
	Ne = JSONOperation(ne)
	Gt = JSONOperation(gt)
	Lt = JSONOperation(lt)
	Ge = JSONOperation(ge)
	Le = JSONOperation(le)
)

type marshaledValue struct {
	value   map[string]interface{}
	rawJson []byte
}

func (t *Txn) FindJSON(q JSONQuery) ([]string, error) {
	dsq := dsquery.Query{
		Prefix: t.model.dsKey.String(),
	}
	dsr, err := t.model.store.datastore.Query(dsq)
	if err != nil {
		return nil, fmt.Errorf("error when internal query: %v", err)
	}

	var values []marshaledValue
	for {
		res, ok := dsr.NextSync()
		if !ok {
			break
		}
		val := make(map[string]interface{})
		if err := json.Unmarshal(res.Value, &val); err != nil {
			return nil, fmt.Errorf("error when unmarshaling query result: %v", err)
		}
		ok, err = q.matchJSON(val)
		if err != nil {
			return nil, fmt.Errorf("error when matching entry with query: %v", err)
		}
		if ok {
			values = append(values, marshaledValue{value: val, rawJson: res.Value})
		}
	}

	if q.Sort.FieldPath != "" {
		var wrongField, cantCompare bool
		sort.Slice(values, func(i, j int) bool {
			fieldI, err := traverseFieldPathMap(values[i].value, q.Sort.FieldPath)
			if err != nil {
				wrongField = true
				return false
			}
			fieldJ, err := traverseFieldPathMap(values[j].value, q.Sort.FieldPath)
			if err != nil {
				wrongField = true
				return false
			}
			res, err := compare(fieldI.Interface(), fieldJ.Interface())
			if err != nil {
				cantCompare = true
				return false
			}
			if q.Sort.Desc {
				res *= -1
			}
			return res < 0
		})
		if wrongField {
			return nil, ErrInvalidSortingField
		}
		if cantCompare {
			panic("can't compare while sorting")
		}
	}

	res := make([]string, len(values))
	for i := range values {
		res[i] = string(values[i].rawJson)
	}

	return res, nil
}

func (q *JSONQuery) matchJSON(v map[string]interface{}) (bool, error) {
	if q == nil {
		panic("query can't be nil")
	}

	andOk := true
	for _, c := range q.Ands {
		fieldRes, err := traverseFieldPathMap(v, c.FieldPath)
		if err != nil {
			return false, err
		}
		ok, err := c.match(fieldRes)
		if err != nil {
			return false, err
		}
		andOk = andOk && ok
		if !andOk {
			break
		}
	}
	if andOk {
		return true, nil
	}

	for _, q := range q.Ors {
		ok, err := q.matchJSON(v)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
}

func compareJsonValue(value interface{}, critVal JSONValue) (int, error) {
	if critVal.String != nil {
		s, ok := value.(string)
		if !ok {
			return 0, &errTypeMismatch{value, critVal}
		}
		return strings.Compare(s, *critVal.String), nil
	}
	if critVal.Bool != nil {
		b, ok := value.(bool)
		if !ok {
			return 0, &errTypeMismatch{value, critVal}
		}
		if *critVal.Bool == b {
			return 0, nil
		}
		return -1, nil
	}
	if critVal.Float != nil {
		f, ok := value.(float64)
		if !ok {
			return 0, &errTypeMismatch{value, critVal}
		}
		if f == *critVal.Float {
			return 0, nil
		}
		if f < *critVal.Float {
			return -1, nil
		}
		return 1, nil
	}
	log.Fatalf("no underlying value for json criterion was provided")
	return 0, nil
}

func (c *JSONCriterion) match(value reflect.Value) (bool, error) {
	valueInterface := value.Interface()
	result, err := compareJsonValue(valueInterface, c.Value)
	if err != nil {
		return false, err
	}
	switch c.Operation {
	case Eq:
		return result == 0, nil
	case Ne:
		return result != 0, nil
	case Gt:
		return result > 0, nil
	case Lt:
		return result < 0, nil
	case Le:
		return result < 0 || result == 0, nil
	case Ge:
		return result > 0 || result == 0, nil
	default:
		panic("invalid operation")
	}

}

func traverseFieldPathMap(value map[string]interface{}, fieldPath string) (reflect.Value, error) {
	fields := strings.Split(fieldPath, ".")

	var curr interface{}
	curr = value
	for i := range fields {
		m, ok := curr.(map[string]interface{})
		if !ok {
			return reflect.Value{}, fmt.Errorf("instance field %s doesn't exist in type %s", fieldPath, value)
		}
		v, ok := m[fields[i]]
		if !ok {
			return reflect.Value{}, fmt.Errorf("instance field %s doesn't exist in type %s", fieldPath, value)
		}
		curr = v
	}
	return reflect.ValueOf(curr), nil
}
