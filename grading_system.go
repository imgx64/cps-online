// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Term struct {
	Typ termType
	N   uint
}

var negZero = math.Copysign(0, -1) // sentinel for non-entered marks

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

type termType uint

const (
	Quarter termType = iota
	Semester
	EndOfYear
)

func classWeights(class string) (quarter, semester float64) {
	intClass, err := strconv.Atoi(class)
	if err != nil {
		// class is KG1, KG2, or SN
		if class == "KG1" || class == "KG2" {
			return 40.0, 20.0
		} else if class == "SN" {
			return 40.0, 20.0
		}
	} else {
		if intClass <= 2 {
			return 40.0, 20.0
		} else if intClass <= 5 {
			return 30.0, 40.0
		} else if intClass <= 12 {
			return 25.0, 50.0
		}
	}
	panic(fmt.Sprintf("Invalid class: %s", class))
}

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

	return Term{typ, uint(n)}, nil
}

// Value is used in forms
func (t Term) Value() string {
	return fmt.Sprintf("%d|%d", t.Typ, t.N)
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

var subjects = []string{
	"Arabic",
	"English",
	"Math",
	"Science",
	"Social Studies",
	"Religion",
	"Islamic Studies",
	"Citizenship",
	"Computer",
	"P.E.",
}

func classHasSubject(class, subject string) bool {
	// TODO: behavior
	ss, ok := gradingSystems[class]
	if !ok {
		// invalid class
		return false
	}
	_, ok = ss[subject]
	if ok {
		return true
	}
	return false
}

type colDescription struct {
	Name     string
	Max      float64
	Editable bool
}

type studentMarks map[Term][]float64

// bestQuizzes returns the sum of all values except the smallest value
func bestQuizzes(marks []float64) float64 {
	var min, total float64
	for _, f := range marks {
		min = math.Min(min, f)
		total += f
	}
	return total - min
}

type gradingSystem interface {
	description(term Term) []colDescription
	evaluate(term Term, marks studentMarks) error
}

// class -> subject -> gradingSystem
var gradingSystems map[string]map[string]gradingSystem

func init() {
	gradingSystems = make(map[string]map[string]gradingSystem)
	for _, class := range classes {
		var gsMap map[string]gradingSystem
		intClass, err := strconv.Atoi(class)
		if err != nil {
			// class is KG1, KG2, or SN
			if class == "KG1" || class == "KG2" {
				gsMap = map[string]gradingSystem{
					"Arabic":   newGGS(class),
					"English":  newGGS(class),
					"Math":     newGGS(class),
					"Science":  newGGS(class),
					"Religion": newSingleGS("Evaluation", class),
				}
			} else if class == "SN" {
				gsMap = map[string]gradingSystem{
					"Arabic":      newGGS(class),
					"English":     newGGS(class),
					"Math":        newGGS(class),
					"Science":     newGGS(class),
					"Religion":    newGGS(class),
					"Citizenship": newCitizenshipGS(class),
					"Computer":    newSingleGS("Evaluation", class),
				}
			}
		} else {
			// class is numeric
			gsMap = map[string]gradingSystem{
				"Arabic":         newGGS(class),
				"English":        newGGS(class),
				"Math":           newGGS(class),
				"Science":        newGGS(class),
				"Social Studies": newGGS(class),
				"Citizenship":    newCitizenshipGS(class),
			}
			if intClass <= 6 {
				gsMap["Religion"] = newGGS(class)
			} else {
				gsMap["Religion"] = newReligion7GS(class)
			}
			if intClass > 1 {
				gsMap["Computer"] = newSingleGS("Evaluation", class)
			}
			if intClass <= 6 {
				gsMap["Islamic Studies"] = newSingleGS("Evaluation", class)
			}
		}

		gsMap["P.E."] = peGradingSystem{}
		gsMap["Behavior"] = behaviorGradingSystem{}
		gradingSystems[class] = gsMap
	}
}

// genericGradingSystem is used for most subjects:
// [5:Homework] [5:Participation] [20:Daily Work]
// [50:best 5 quizzes out of 6] [20:Quarter Exam]
type genericGradingSystem struct {
	quarterWeight  float64
	semesterWeight float64
}

func newGGS(class string) gradingSystem {
	q, s := classWeights(class)
	return genericGradingSystem{q, s}
}

func (ggs genericGradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return []colDescription{
			{"Homework", 5, true},
			{"Participation", 5, true},
			{"Daily Work", 20, true},
			{"Quiz 1", 10, true},
			{"Quiz 2", 10, true},
			{"Quiz 3", 10, true},
			{"Quiz 4", 10, true},
			{"Quiz 5", 10, true},
			{"Quiz 6", 10, true},
			{"Best 5 Quizzes", 50, false},
			{"Quarter Exam", 20, true},
			{"Quarter Mark", 100, false},
			{"Quarter %", ggs.quarterWeight, false},
		}
	} else if term.Typ == Semester {
		return []colDescription{
			// TODO: per-subject exam marks
			{"Semester Exam", 100, true},
			{"Semester Exam %", ggs.semesterWeight, false},
			{"Semester Mark", 100, false},
		}
	} else if term.Typ == EndOfYear {
		return []colDescription{
			{"Semester 1 %", 50, false},
			{"Semester 2 %", 50, false},
			{"Final mark", 100, false},
		}
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (ggs genericGradingSystem) evaluate(term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := ggs.description(term)

	switch {
	case m == nil: // first time to evaluate it
		m = make([]float64, len(desc))
		for i, _ := range desc {
			m[i] = negZero
		}
	case len(m) != len(desc): // sanity check
		err = invalidNumberOfMarks
		m = make([]float64, len(desc))
		for i, _ := range desc {
			m[i] = negZero
		}
	}

	// more sanity checks
	for i, d := range desc {
		if m[i] < 0 || m[i] > d.Max {
			m[i] = negZero
			if err == nil {
				err = invalidRangeOfMarks
			}
		}
	}

	if term.Typ == Quarter {
		// Best 5 Quizzes
		m[9] = bestQuizzes(m[3:9])

		// Quarter mark
		m[11] = m[0] + m[1] + m[2] + m[9] + m[10]

		// Quarter %
		m[12] = m[11] * ggs.quarterWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam %
		m[1] = m[0] * ggs.semesterWeight / 100.0

		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		ggs.evaluate(Term{Quarter, q1}, marks)
		ggs.evaluate(Term{Quarter, q2}, marks)
		q1Mark := marks[Term{Quarter, q1}][12]
		q2Mark := marks[Term{Quarter, q2}][12]

		m[2] = m[1] + q1Mark + q2Mark
	} else if term.Typ == EndOfYear {
		ggs.evaluate(Term{Semester, 1}, marks)
		ggs.evaluate(Term{Semester, 2}, marks)
		m[0] = marks[Term{Semester, 1}][2] / 2.0
		m[1] = marks[Term{Semester, 2}][2] / 2.0

		m[2] = m[0] + m[1]
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

// peGradingSystem is used for P.E.
// 25% each quarter, no semesters
type peGradingSystem struct {
}

func (pgs peGradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return []colDescription{
			{"Evaluation", 100, true},
		}
	} else if term.Typ == Semester {
		return []colDescription{
			{"Semester Mark", 100, false},
		}
	} else if term.Typ == EndOfYear {
		return []colDescription{
			{"Semester 1 %", 50, false},
			{"Semester 2 %", 50, false},
			{"Final mark", 100, false},
		}
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (pgs peGradingSystem) evaluate(term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := pgs.description(term)

	switch {
	case m == nil: // first time to evaluate it
		m = make([]float64, len(desc))
		for i, _ := range desc {
			m[i] = negZero
		}
	case len(m) != len(desc): // sanity check
		err = invalidNumberOfMarks
		m = make([]float64, len(desc))
		for i, _ := range desc {
			m[i] = negZero
		}
	}

	// more sanity checks
	for i, d := range desc {
		if m[i] < 0 || m[i] > d.Max {
			m[i] = negZero
			if err == nil {
				err = invalidRangeOfMarks
			}
		}
	}

	if term.Typ == Quarter {
		// no calculations
	} else if term.Typ == Semester {
		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		pgs.evaluate(Term{Quarter, q1}, marks)
		pgs.evaluate(Term{Quarter, q2}, marks)
		q1Mark := marks[Term{Quarter, q1}][0] / 2.0
		q2Mark := marks[Term{Quarter, q2}][0] / 2.0

		m[0] = q1Mark + q2Mark
	} else if term.Typ == EndOfYear {
		pgs.evaluate(Term{Semester, 1}, marks)
		pgs.evaluate(Term{Semester, 2}, marks)
		m[0] = marks[Term{Semester, 1}][0] / 2.0
		m[1] = marks[Term{Semester, 2}][0] / 2.0

		m[2] = m[0] + m[1]
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

// simpleGradingSystem contains a number of columns that are simply added
// varies from subject to subject
type simpleGradingSystem struct {
	columns        []colDescription
	quarterWeight  float64
	semesterWeight float64
}

func newSingleGS(column, class string) gradingSystem {
	q, s := classWeights(class)
	return simpleGradingSystem{
		[]colDescription{{column, 100, true}},
		q,
		s,
	}
}

func newCitizenshipGS(class string) gradingSystem {
	q, s := classWeights(class)
	return simpleGradingSystem{
		[]colDescription{
			{"DW", 15, true},
			{"Oral", 50, true},
			{"Project", 10, true},
			{"Exam", 20, true},
		},
		q,
		s,
	}
}

func newReligion7GS(class string) gradingSystem {
	q, s := classWeights(class)
	return simpleGradingSystem{
		[]colDescription{
			{"DW", 15, true},
			{"HW", 5, true},
			{"Quran", 30, true},
			{"Exam", 50, true},
		},
		q,
		s,
	}
}

func (sgs simpleGradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		var desc []colDescription
		desc = append(desc, sgs.columns...)
		quarterCols := []colDescription{
			{"Quarter Mark", 100, false},
			{"Quarter %", sgs.quarterWeight, false},
		}
		desc = append(desc, quarterCols...)
		return desc
	} else if term.Typ == Semester {
		return []colDescription{
			// TODO: per-subject exam marks
			{"Semester Exam", 100, true},
			{"Semester Exam %", sgs.semesterWeight, false},
			{"Semester Mark", 100, false},
		}
	} else if term.Typ == EndOfYear {
		return []colDescription{
			{"Semester 1 %", 50, false},
			{"Semester 2 %", 50, false},
			{"Final mark", 100, false},
		}
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (sgs simpleGradingSystem) evaluate(term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := sgs.description(term)

	switch {
	case m == nil: // first time to evaluate it
		m = make([]float64, len(desc))
		for i, _ := range desc {
			m[i] = negZero
		}
	case len(m) != len(desc): // sanity check
		err = invalidNumberOfMarks
		m = make([]float64, len(desc))
		for i, _ := range desc {
			m[i] = negZero
		}
	}

	// more sanity checks
	for i, d := range desc {
		if m[i] < 0 || m[i] > d.Max {
			m[i] = negZero
			if err == nil {
				err = invalidRangeOfMarks
			}
		}
	}

	l := len(desc)
	if term.Typ == Quarter {
		var sum float64
		for i := 0; i < l-2; i++ {
			sum += m[i]
		}
		m[l-2] = sum
		m[l-1] = sum * sgs.quarterWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam %
		m[1] = m[0] * sgs.semesterWeight / 100.0

		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		sgs.evaluate(Term{Quarter, q1}, marks)
		sgs.evaluate(Term{Quarter, q2}, marks)
		q1Mark := marks[Term{Quarter, q1}][l-2]
		q2Mark := marks[Term{Quarter, q2}][l-2]

		m[2] = m[1] + q1Mark + q2Mark
	} else if term.Typ == EndOfYear {
		sgs.evaluate(Term{Semester, 1}, marks)
		sgs.evaluate(Term{Semester, 2}, marks)
		m[0] = marks[Term{Semester, 1}][0] / 2.0
		m[1] = marks[Term{Semester, 2}][0] / 2.0

		m[2] = m[0] + m[2]
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

// behaviorGradingSystem contains a number of columns that are simply added
// varies from subject to subject
type behaviorGradingSystem struct {
}

func (behaviorGradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return []colDescription{
			{"Follows school guidelines for safe and appropriate behaviour", 4, true},
			{"Demonstrates courtesy and respect", 4, true},
			{"Listens and responds", 4, true},
			{"Strives for quality work", 4, true},
			{"Shows initiative / is a self - starter", 4, true},
			{"Participates enthusiastically in activities", 4, true},
			{"Uses time efficiently and appropriately", 4, true},
			{"Completes class work on time ", 4, true},
			{"Contributes to discussion and group tasks", 4, true},
			{"Works cooperatively with others", 4, true},
			{"Works well independently", 4, true},
			{"Returns complete homework", 4, true},
			{"Organizes shelf, materials and belongings ", 4, true},
			{"Asks questions to clarify content", 4, true},
			{"Clearly communicates to teachers ", 4, true},
		}
	} else if term.Typ == Semester || term.Typ == EndOfYear {
		return nil
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (bgs behaviorGradingSystem) evaluate(term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := bgs.description(term)

	switch {
	case m == nil: // first time to evaluate it
		m = make([]float64, len(desc))
		for i, _ := range desc {
			m[i] = negZero
		}
	case len(m) != len(desc): // sanity check
		err = invalidNumberOfMarks
		m = make([]float64, len(desc))
		for i, _ := range desc {
			m[i] = negZero
		}
	}

	// more sanity checks
	for i, d := range desc {
		if m[i] < 0 || m[i] > d.Max {
			m[i] = negZero
			if err == nil {
				err = invalidRangeOfMarks
			}
		}
	}

	// no calculations

	marks[term] = m
	return
}
