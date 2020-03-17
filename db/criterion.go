package db

/*	MIT License

	Copyright (c) 2019 Tim Shannon

	Permission is hereby granted, free of charge, to any person obtaining a copy
	of this software and associated documentation files (the "Software"), to deal
	in the Software without restriction, including without limitation the rights
	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
	copies of the Software, and to permit persons to whom the Software is
	furnished to do so, subject to the following conditions:

	The above copyright notice and this permission notice shall be included in all
	copies or substantial portions of the Software.

	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
	SOFTWARE.
*/
// Code has multiple changes compared to original, but still merits proper mentioning.

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"
	"time"
)

// Criterion is a partial condition that can specify comparison
// operator for a field.
type Criterion struct {
	fieldPath string
	operation operation
	value     interface{}
	query     *Query
}

type operation int

const (
	eq operation = iota
	ne           // !=
	gt           // >
	lt           // <
	ge           // >=
	le           // <=
	fn           // func
)

// Eq is an equality operator against a field
func (c *Criterion) Eq(value interface{}) *Query {
	return c.createcriterion(eq, value)
}

// Ne is a not equal operator against a field
func (c *Criterion) Ne(value interface{}) *Query {
	return c.createcriterion(ne, value)
}

// Gt is a greater operator against a field
func (c *Criterion) Gt(value interface{}) *Query {
	return c.createcriterion(gt, value)
}

// Lt is a less operation against a field
func (c *Criterion) Lt(value interface{}) *Query {
	return c.createcriterion(lt, value)
}

// Ge is a greater or equal operator against a field
func (c *Criterion) Ge(value interface{}) *Query {
	return c.createcriterion(ge, value)
}

// Le is a less or equal operator against a field
func (c *Criterion) Le(value interface{}) *Query {
	return c.createcriterion(le, value)
}

// Fn is a custom evaluation function against a field
func (c *Criterion) Fn(mf MatchFunc) *Query {
	return c.createcriterion(fn, mf)
}

func (c *Criterion) createcriterion(op operation, value interface{}) *Query {
	c.operation = op
	c.value = value
	if c.query == nil {
		c.query = &Query{}
	}
	c.query.ands = append(c.query.ands, c)
	return c.query
}

func (c *Criterion) compare(testedValue, criterionValue interface{}) (int, error) {
	if testedValue == nil || criterionValue == nil {
		if testedValue == criterionValue {
			return 0, nil
		}
		return 0, &errTypeMismatch{testedValue, criterionValue}
	}

	for reflect.TypeOf(testedValue).Kind() == reflect.Ptr {
		testedValue = reflect.ValueOf(testedValue).Elem().Interface()
	}

	for reflect.TypeOf(criterionValue).Kind() == reflect.Ptr {
		criterionValue = reflect.ValueOf(criterionValue).Elem().Interface()
	}

	return compare(testedValue, criterionValue)
}

func (c *Criterion) match(value reflect.Value) (bool, error) {
	valueInterface := value.Interface()
	switch c.operation {
	case fn:
		return c.value.(MatchFunc)(valueInterface)
	default:
		result, err := c.compare(valueInterface, c.value)
		if err != nil {
			return false, err
		}
		switch c.operation {
		case eq:
			return result == 0, nil
		case ne:
			return result != 0, nil
		case gt:
			return result > 0, nil
		case lt:
			return result < 0, nil
		case le:
			return result < 0 || result == 0, nil
		case ge:
			return result > 0 || result == 0, nil
		default:
			panic("invalid operation")
		}
	}
}

func traverseFieldPath(value reflect.Value, fieldPath string) (reflect.Value, error) {
	fields := strings.Split(fieldPath, ".")
	for i := range fields {
		if value.Kind() == reflect.Ptr {
			value = value.Elem()
		}
		value = value.FieldByName(fields[i])

		if !value.IsValid() {
			return reflect.Value{}, fmt.Errorf("instance field %s doesn't exist in type %s", fieldPath, value)
		}
	}
	return value, nil
}

type errTypeMismatch struct {
	Value interface{}
	Other interface{}
}

func (e *errTypeMismatch) Error() string {
	return fmt.Sprintf("%v (%T) cannot be compared with %v (%T)", e.Value, e.Value, e.Other, e.Other)
}

//Comparer compares a type against the encoded value in the db. The result should be 0 if current==other,
// -1 if current < other, and +1 if current > other.
// If a field in a struct doesn't specify a comparer, then the default comparison is used (convert to string and compare)
// this interface is already handled for standard Go Types as well as more complex ones such as those in time and big
// an error is returned if the type cannot be compared
// The concrete type will always be passedin, not a pointer
type Comparer interface {
	Compare(other interface{}) (int, error)
}

// MatchFunc is a function used to test an arbitrary matching value in a query
type MatchFunc func(value interface{}) (bool, error)

func compare(value, other interface{}) (int, error) {
	switch t := value.(type) {
	case time.Time:
		tother, ok := other.(time.Time)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(time.Time).Equal(tother) {
			return 0, nil
		}

		if value.(time.Time).Before(tother) {
			return -1, nil
		}
		return 1, nil
	case big.Float:
		o, ok := other.(big.Float)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		v := value.(big.Float)

		return v.Cmp(&o), nil
	case big.Int:
		o, ok := other.(big.Int)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		v := value.(big.Int)

		return v.Cmp(&o), nil
	case big.Rat:
		o, ok := other.(big.Rat)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		v := value.(big.Rat)

		return v.Cmp(&o), nil
	case int:
		tother, ok := other.(int)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(int) == tother {
			return 0, nil
		}

		if value.(int) < tother {
			return -1, nil
		}
		return 1, nil
	case int8:
		tother, ok := other.(int8)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(int8) == tother {
			return 0, nil
		}

		if value.(int8) < tother {
			return -1, nil
		}
		return 1, nil

	case int16:
		tother, ok := other.(int16)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(int16) == tother {
			return 0, nil
		}

		if value.(int16) < tother {
			return -1, nil
		}
		return 1, nil
	case int32:
		tother, ok := other.(int32)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(int32) == tother {
			return 0, nil
		}

		if value.(int32) < tother {
			return -1, nil
		}
		return 1, nil

	case int64:
		tother, ok := other.(int64)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(int64) == tother {
			return 0, nil
		}

		if value.(int64) < tother {
			return -1, nil
		}
		return 1, nil
	case uint:
		tother, ok := other.(uint)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(uint) == tother {
			return 0, nil
		}

		if value.(uint) < tother {
			return -1, nil
		}
		return 1, nil
	case uint8:
		tother, ok := other.(uint8)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(uint8) == tother {
			return 0, nil
		}

		if value.(uint8) < tother {
			return -1, nil
		}
		return 1, nil

	case uint16:
		tother, ok := other.(uint16)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(uint16) == tother {
			return 0, nil
		}

		if value.(uint16) < tother {
			return -1, nil
		}
		return 1, nil
	case uint32:
		tother, ok := other.(uint32)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(uint32) == tother {
			return 0, nil
		}

		if value.(uint32) < tother {
			return -1, nil
		}
		return 1, nil

	case uint64:
		tother, ok := other.(uint64)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(uint64) == tother {
			return 0, nil
		}

		if value.(uint64) < tother {
			return -1, nil
		}
		return 1, nil
	case float32:
		tother, ok := other.(float32)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(float32) == tother {
			return 0, nil
		}

		if value.(float32) < tother {
			return -1, nil
		}
		return 1, nil
	case float64:
		tother, ok := other.(float64)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(float64) == tother {
			return 0, nil
		}

		if value.(float64) < tother {
			return -1, nil
		}
		return 1, nil
	case string:
		tother, ok := other.(string)
		if !ok {
			return 0, &errTypeMismatch{t, other}
		}

		if value.(string) == tother {
			return 0, nil
		}

		if value.(string) < tother {
			return -1, nil
		}
		return 1, nil
	case Comparer:
		return value.(Comparer).Compare(other)
	default:
		valS := fmt.Sprintf("%s", value)
		otherS := fmt.Sprintf("%s", other)
		if valS == otherS {
			return 0, nil
		}

		if valS < otherS {
			return -1, nil
		}

		return 1, nil
	}
}
