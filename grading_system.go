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
	Quarter termType = iota + 1
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

var subjects = []string{
	"Arabic",
	"English",
	"Math",
	"Science",
	"Biology",
	"Chemistry",
	"Physics",
	"Social Studies",
	"Religion",
	"Islamic Studies",
	"Citizenship",
	"Computer",
	"P.E.",
	"Behavior",
	"Remarks",
}

func classHasSubject(class, subject string) bool {
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

// sumMarks sums all values, and if one of them is negZero, returns negZero
func sumMarks(marks ...float64) float64 {
	var total float64
	for _, v := range marks {
		if math.Signbit(v) {
			return negZero
		}
		total += v
	}
	return total
}

// sumQuizzes returns the sum of all values except the smallest value
// returns negZero if more than one value is negZero
func sumQuizzes(marks ...float64) float64 {
	var min, total float64
	nNegZero := 0
	for i, v := range marks {
		if i == 0 {
			min = v
		} else {
			min = math.Min(min, v)
		}
		total += v
		if math.Signbit(v) {
			nNegZero++
		}
	}
	if nNegZero > 1 {
		return negZero
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
				"Arabic":      newGGS(class),
				"English":     newGGS(class),
				"Math":        newGGS(class),
				"Citizenship": newCitizenshipGS(class),
			}
			if intClass <= 8 {
				gsMap["Social Studies"] = newGGS(class)
				gsMap["Science"] = newGGS(class)
			} else {
				gsMap["Biology"] = newGGS(class)
				gsMap["Chemistry"] = newGGS(class)
				gsMap["Physics"] = newGGS(class)
			}
			if intClass <= 5 {
				gsMap["Religion"] = newGGS(class)
			} else {
				gsMap["Religion"] = newReligion7GS(class)
			}
			if intClass > 1 {
				gsMap["Computer"] = newSingleGS("Evaluation", class)
			}
			if intClass <= 5 {
				gsMap["Islamic Studies"] = newSingleGS("Evaluation", class)
			}
		}

		gsMap["P.E."] = peGradingSystem{}
		gsMap["Behavior"] = behaviorGradingSystem{}
		gradingSystems[class] = gsMap
	}
}

func getGradingSystem(class, subject string) gradingSystem {
	return gradingSystems[class][subject]
}

// genericGradingSystem is used for most subjects:
// [5:Homework] [5:Participation] [20:Daily Work]
// [50:best 5 quizzes out of 6] [20:Quarter Exam]
type genericGradingSystem struct {
	qWeight float64
	sWeight float64
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
			{"Quarter %", ggs.qWeight, false},
		}
	} else if term.Typ == Semester {
		return []colDescription{
			// TODO: per-subject exam marks
			{"Semester Exam", 100, true},
			{"Semester Exam %", ggs.sWeight, false},
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
		m[9] = sumQuizzes(m[3:9]...)

		// Quarter mark
		m[11] = sumMarks(m[0], m[1], m[2], m[9], m[10])

		// Quarter %
		m[12] = m[11] * ggs.qWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam %
		m[1] = m[0] * ggs.sWeight / 100.0

		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		ggs.evaluate(Term{Quarter, q1}, marks)
		ggs.evaluate(Term{Quarter, q2}, marks)
		q1Mark := marks[Term{Quarter, q1}][12]
		q2Mark := marks[Term{Quarter, q2}][12]

		m[2] = sumMarks(m[1], q1Mark, q2Mark)
	} else if term.Typ == EndOfYear {
		ggs.evaluate(Term{Semester, 1}, marks)
		ggs.evaluate(Term{Semester, 2}, marks)
		m[0] = marks[Term{Semester, 1}][2] / 2.0
		m[1] = marks[Term{Semester, 2}][2] / 2.0

		m[2] = sumMarks(m[0], m[1])
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

func (ggs genericGradingSystem) get100(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return m[len(m)-2]
	} else if term.Typ == Semester {
		return m[2]
	} else if term.Typ == EndOfYear {
		return m[2]
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (ggs genericGradingSystem) getExam(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return m[len(m)-3]
	} else if term.Typ == Semester {
		return m[1]
	} else if term.Typ == EndOfYear {
		return negZero
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (ggs genericGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.Signbit(m[len(m)-1])
}

func (ggs genericGradingSystem) quarterWeight() float64 {
	return ggs.qWeight
}

func (ggs genericGradingSystem) semesterWeight() float64 {
	return ggs.sWeight
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

		m[0] = sumMarks(q1Mark, q2Mark)
	} else if term.Typ == EndOfYear {
		pgs.evaluate(Term{Semester, 1}, marks)
		pgs.evaluate(Term{Semester, 2}, marks)
		m[0] = marks[Term{Semester, 1}][0] / 2.0
		m[1] = marks[Term{Semester, 2}][0] / 2.0

		m[2] = sumMarks(m[0], m[1])
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

func (pgs peGradingSystem) get100(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return m[0]
	} else if term.Typ == Semester {
		return m[0]
	} else if term.Typ == EndOfYear {
		return m[2]
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (pgs peGradingSystem) getExam(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return m[0]
	} else if term.Typ == Semester {
		return negZero
	} else if term.Typ == EndOfYear {
		return negZero
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (pgs peGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.Signbit(m[len(m)-1])
}

func (pgs peGradingSystem) quarterWeight() float64 {
	return 50.0
}

func (pgs peGradingSystem) semesterWeight() float64 {
	return negZero
}

// simpleGradingSystem contains a number of columns that are simply added
// varies from subject to subject
type simpleGradingSystem struct {
	columns []colDescription
	qWeight float64
	sWeight float64
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
			{"DW", 20, true},
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
			{"Quarter %", sgs.qWeight, false},
		}
		desc = append(desc, quarterCols...)
		return desc
	} else if term.Typ == Semester {
		return []colDescription{
			// TODO: per-subject exam marks
			{"Semester Exam", 100, true},
			{"Semester Exam %", sgs.sWeight, false},
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
		sum := sumMarks(m[0 : l-2]...)
		m[l-2] = sum
		m[l-1] = sum * sgs.qWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam %
		m[1] = m[0] * sgs.sWeight / 100.0

		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		sgs.evaluate(Term{Quarter, q1}, marks)
		sgs.evaluate(Term{Quarter, q2}, marks)
		q1Mark := marks[Term{Quarter, q1}][l-2]
		q2Mark := marks[Term{Quarter, q2}][l-2]

		m[2] = sumMarks(m[1], q1Mark, q2Mark)
	} else if term.Typ == EndOfYear {
		sgs.evaluate(Term{Semester, 1}, marks)
		sgs.evaluate(Term{Semester, 2}, marks)
		m[0] = marks[Term{Semester, 1}][0] / 2.0
		m[1] = marks[Term{Semester, 2}][0] / 2.0

		m[2] = sumMarks(m[0], m[1])
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

func (sgs simpleGradingSystem) get100(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return m[len(m)-2]
	} else if term.Typ == Semester {
		return m[2]
	} else if term.Typ == EndOfYear {
		return m[2]
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (sgs simpleGradingSystem) getExam(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return m[len(m)-3]
	} else if term.Typ == Semester {
		return m[1]
	} else if term.Typ == EndOfYear {
		return negZero
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (sgs simpleGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.Signbit(m[len(m)-1])
}

func (sgs simpleGradingSystem) quarterWeight() float64 {
	return sgs.qWeight
}

func (sgs simpleGradingSystem) semesterWeight() float64 {
	return sgs.sWeight
}

// behaviorGradingSystem contains behavrior. There are no calculations to make
type behaviorGradingSystem struct {
}

var behaviorDesc = []colDescription{
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

func (behaviorGradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return behaviorDesc
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

func (bgs behaviorGradingSystem) get100(term Term, marks studentMarks) float64 {
	if bgs.ready(term, marks) {
		return 100.0
	}
	return negZero
}

func (bgs behaviorGradingSystem) getExam(term Term, marks studentMarks) float64 {
	return negZero
}

func (bgs behaviorGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	for _, v := range m {
		if math.Signbit(v) {
			// mark not entered
			return false
		}
	}

	return true
}

func (bgs behaviorGradingSystem) quarterWeight() float64 {
	return negZero
}

func (bgs behaviorGradingSystem) semesterWeight() float64 {
	return negZero
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

func getLetterSystem(class string) letterSystem {
	intClass, err := strconv.Atoi(class)
	if err != nil {
		// class is KG1, KG2, or SN
		return OVSLU
	} else {
		if intClass <= 2 {
			return OVSLU
		} else {
			// 3-12
			return ABCDF
		}
	}

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
	if math.Signbit(mark) {
		return "N/A"
	}

	for _, l := range ls {
		if mark > l.minMark {
			return l.letter
		}
	}
	// something wrong with the letterSystem
	return "Error"
}

// subjectInAverage returns true if subject should be calculated in average
func subjectInAverage(subject, class string) bool {
	if (class == "KG1" || class == "KG2") && subject == "Religion" {
		return false
	}
	subjectsInAverage := map[string]bool{
		"Arabic":         true,
		"English":        true,
		"Math":           true,
		"Science":        true,
		"Biology":        true,
		"Chemistry":      true,
		"Physics":        true,
		"Social Studies": true,
		"Religion":       true,
	}
	return subjectsInAverage[subject]
}
