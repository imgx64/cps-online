// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Term struct {
	Typ termType
	N   int
}

var (
	invalidNumberOfMarks = fmt.Errorf("Invalid number of marks.")
	invalidRangeOfMarks  = fmt.Errorf("Invalid range of marks.")
)

var terms = []Term{
	{Quarter, 1},
	{Quarter, 2},
	{Semester, 1},
	{Quarter, 3},
	{Quarter, 4},
	{Semester, 2},
	{EndOfYear, 0},
}

type termType int

const (
	Quarter termType = iota + 1
	Semester
	EndOfYear
)

var termStrings = map[termType]string{
	Quarter:   "Quarter",
	Semester:  "Semester",
	EndOfYear: "End of Year",
}

func parseTerm(s string) (Term, error) {
	cs := strings.Split(s, "|")
	if len(cs) != 2 {
		return Term{}, fmt.Errorf("Invalid term: %s", s)
	}

	typeNumber, err := strconv.Atoi(cs[0])
	if err != nil {
		return Term{}, fmt.Errorf("Invalid term: %s", s)
	}
	typ := termType(typeNumber)
	_, ok := termStrings[typ]
	if !ok {
		return Term{}, fmt.Errorf("Invalid term: %s", s)
	}

	n, err := strconv.Atoi(cs[1])
	if err != nil {
		return Term{}, fmt.Errorf("Invalid term: %s", s)
	}

	return Term{typ, n}, nil
}

// Value is used in forms
func (t Term) Value() string {
	return fmt.Sprintf("%d|%d", t.Typ, t.N)
}

// Used in reportcards template
func (t Term) IsQuarter() bool {
	return t.Typ == Quarter
}

func (t Term) String() string {
	s, ok := termStrings[t.Typ]
	if !ok {
		panic(fmt.Sprintf("Invalid term type: %d", t.Typ))
	}
	if t.N == 0 {
		return s
	}
	return fmt.Sprintf("%s %d", s, t.N)
}

// sumQuizzes returns the sum of all values except the smallest value
// returns NaN if more than one value is NaN
func sumQuizzes(marks ...float64) float64 {
	var min, total float64
	nNaN := 0
	for i, v := range marks {
		if math.IsNaN(v) {
			nNaN++
			v = 0
		}
		if i == 0 {
			min = v
		} else {
			min = math.Min(min, v)
		}
		total += v

	}
	if nNaN > 1 {
		return math.NaN()
	}
	return total - min
}

type gradingSystem interface {
	description(term Term) []colDescription
	evaluate(term Term, marks studentMarks) error
	get100(term Term, marks studentMarks) float64
	getExam(term Term, marks studentMarks) float64
	ready(term Term, marks studentMarks) bool

	quarterWeight() float64
	semesterWeight() float64
}

type letterType struct {
	letter      string
	description string
	minMark     float64
}

type letterSystem []letterType

var ABCDF = letterSystem{
	{"A", "Excellent", 90.0},
	{"B", "Good", 80.0},
	{"C", "Satisfactory", 70.0},
	{"D", "Needs Improvement", 60.0},
	{"F", "Fail Insufficient", 0.0},
}

var OVSLU = letterSystem{
	{"O", "Outstanding", 90.0},
	{"V", "Very Good", 80.0},
	{"S", "Satisfactory", 70.0},
	{"L", "Limited Progress", 60.0},
	{"U", "Unsatisfactory", 0.0},
}

// String returns a description of the letter system
func (ls letterSystem) String() string {
	buf := new(bytes.Buffer)
	previousMin := 101.0
	for i, l := range ls {
		if i > 0 {
			fmt.Fprint(buf, " - ")
		}
		fmt.Fprintf(buf, "%s: %s (%.0f-%.0f)",
			l.letter, l.description, l.minMark, previousMin-1)
		previousMin = l.minMark
	}
	return buf.String()
}

func (ls letterSystem) getLetter(mark float64) string {
	if math.IsNaN(mark) {
		return "N/A"
	}

	for _, l := range ls {
		if mark >= l.minMark {
			return l.letter
		}
	}
	// something wrong with the letterSystem
	return "Error"
}
