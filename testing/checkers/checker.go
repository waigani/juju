// Copyright 2012, 2013 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package checkers

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	. "launchpad.net/gocheck"
)

func TimeBetween(start, end time.Time) Checker {
	if end.Before(start) {
		return &timeBetweenChecker{end, start}
	}
	return &timeBetweenChecker{start, end}
}

type timeBetweenChecker struct {
	start, end time.Time
}

func (checker *timeBetweenChecker) Info() *CheckerInfo {
	info := CheckerInfo{
		Name:   "TimeBetween",
		Params: []string{"obtained"},
	}
	return &info
}

func (checker *timeBetweenChecker) Check(params []interface{}, names []string) (result bool, error string) {
	when, ok := params[0].(time.Time)
	if !ok {
		return false, "obtained value type must be time.Time"
	}
	if when.Before(checker.start) {
		return false, fmt.Sprintf("obtained value %#v type must before start value of %#v", when, checker.start)
	}
	if when.After(checker.end) {
		return false, fmt.Sprintf("obtained value %#v type must after end value of %#v", when, checker.end)
	}
	return true, ""
}

// DurationLessThan checker

type durationLessThanChecker struct {
	*CheckerInfo
}

var DurationLessThan Checker = &durationLessThanChecker{
	&CheckerInfo{Name: "DurationLessThan", Params: []string{"obtained", "expected"}},
}

func (checker *durationLessThanChecker) Check(params []interface{}, names []string) (result bool, error string) {
	obtained, ok := params[0].(time.Duration)
	if !ok {
		return false, "obtained value type must be time.Duration"
	}
	expected, ok := params[1].(time.Duration)
	if !ok {
		return false, "expected value type must be time.Duration"
	}
	return obtained.Nanoseconds() < expected.Nanoseconds(), ""
}

// HasPrefix checker for checking strings

func stringOrStringer(value interface{}) (string, bool) {
	result, isString := value.(string)
	if !isString {
		if stringer, isStringer := value.(fmt.Stringer); isStringer {
			result, isString = stringer.String(), true
		}
	}
	return result, isString
}

type hasPrefixChecker struct {
	*CheckerInfo
}

var HasPrefix Checker = &hasPrefixChecker{
	&CheckerInfo{Name: "HasPrefix", Params: []string{"obtained", "expected"}},
}

func (checker *hasPrefixChecker) Check(params []interface{}, names []string) (result bool, error string) {
	expected, ok := params[1].(string)
	if !ok {
		return false, "expected must be a string"
	}

	obtained, isString := stringOrStringer(params[0])
	if isString {
		return strings.HasPrefix(obtained, expected), ""
	}

	return false, "Obtained value is not a string and has no .String()"
}

type hasSuffixChecker struct {
	*CheckerInfo
}

var HasSuffix Checker = &hasSuffixChecker{
	&CheckerInfo{Name: "HasSuffix", Params: []string{"obtained", "expected"}},
}

func (checker *hasSuffixChecker) Check(params []interface{}, names []string) (result bool, error string) {
	expected, ok := params[1].(string)
	if !ok {
		return false, "expected must be a string"
	}

	obtained, isString := stringOrStringer(params[0])
	if isString {
		return strings.HasSuffix(obtained, expected), ""
	}

	return false, "Obtained value is not a string and has no .String()"
}

type containsChecker struct {
	*CheckerInfo
}

var Contains Checker = &containsChecker{
	&CheckerInfo{Name: "Contains", Params: []string{"obtained", "expected"}},
}

func (checker *containsChecker) Check(params []interface{}, names []string) (result bool, error string) {
	expected, ok := params[1].(string)
	if !ok {
		return false, "expected must be a string"
	}

	obtained, isString := stringOrStringer(params[0])
	if isString {
		return strings.Contains(obtained, expected), ""
	}

	return false, "Obtained value is not a string and has no .String()"
}

type sameContents struct {
	*CheckerInfo
}

// SameContents checks that the obtained slice contains all the values (and same number of values) of
// the expected slice and vice versa, without worrying about order. SameContents uses DeepEquals to
// compare values.
//
// This is a dumb implementation that takes n^2 time if the slices are the same lenth, so don't
// use it on very large slices
var SameContents Checker = &sameContents{
	&CheckerInfo{Name: "SameContents", Params: []string{"obtained", "expected"}},
}

func (checker *sameContents) Check(params []interface{}, names []string) (result bool, error string) {
	if len(params) != 2 {
		return false, "SameContents expects two slice arguments"
	}
	obtained := params[0]
	expected := params[1]
	tob := reflect.TypeOf(obtained)
	if tob.Kind() != reflect.Slice {
		return false, fmt.Sprintf("SameContents expects the obtained value to be a slice, got %q",
			tob.Kind())
	}

	texp := reflect.TypeOf(expected)
	if texp.Kind() != reflect.Slice {
		return false, fmt.Sprintf("SameContents expects the expected value to be a slice, got %q",
			texp.Kind())
	}

	if texp != tob {
		return false, fmt.Sprintf(
			"SameContents expects two slices of the same type, expected: %q, got: %q",
			texp, tob)
	}

	vexp := reflect.ValueOf(expected)
	vob := reflect.ValueOf(obtained)

	lexp := vexp.Len()
	lob := vob.Len()

	if lexp != lob {
		// Slice has incorrect number of elements
		return false, ""
	}

	// as we find matches from the expected slice, remove them from the obtained slice,
	// that way we make sure the count of duplicate items is the same
	// i.e. 1 1 2 won't match 1 2 2
outer:
	for i := 0; i < lexp; i++ {
		val := vexp.Index(i)
		for j := 0; j < vob.Len(); j++ {
			if reflect.DeepEqual(val.Interface(), vob.Index(j).Interface()) {
				if vob.Len() == 1 {
					// found the last match in the obtained slice, all done
					return true, ""
				}
				// remove the match from the obtained slice
				if j == 0 {
					vob = vob.Slice(1, vob.Len())
				} else {
					vob = reflect.AppendSlice(vob.Slice(0, j), vob.Slice(j+1, vob.Len()))
				}
				continue outer
			}
		}
		// Value in expected slice not found in obtained slice
		return false, ""
	}

	// only ever get here with two empty slices
	return true, ""
}
