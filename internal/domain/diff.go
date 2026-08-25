package domain

import (
	"fmt"
	"reflect"
	"sort"
)

func DiffPlans(before, after Plan) []FieldChange {
	bv, av := reflect.ValueOf(before), reflect.ValueOf(after)
	t := bv.Type()
	changes := make([]FieldChange, 0)
	for i := 0; i < bv.NumField(); i++ {
		b, a := fmt.Sprint(bv.Field(i).Interface()), fmt.Sprint(av.Field(i).Interface())
		if b == a {
			continue
		}
		name := t.Field(i).Tag.Get("json")
		changes = append(changes, FieldChange{Field: name, Before: b, After: a})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Field < changes[j].Field })
	return changes
}
