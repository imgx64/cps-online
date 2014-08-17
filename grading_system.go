// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"

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
		// class is KG or SN
		if class == "KG1" || class == "KG2" || class == "PreKG" {
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

func getGradingSystem(c appengine.Context, class string, subject string) gradingSystem {
	classes := getClasses(c)

	// class -> subject -> gradingSystem
	var gradingSystems map[string]map[string]gradingSystem

	gradingSystems = make(map[string]map[string]gradingSystem)
	for _, class := range classes {
		var gsMap map[string]gradingSystem
		intClass, err := strconv.Atoi(class)
		if err != nil {
			// class is KG or SN
			if class == "KG1" || class == "KG2" || class == "PreKG" {
				gsMap = map[string]gradingSystem{
					"Arabic":   newGGS(class),
					"English":  newEGS(class),
					"Math":     newMGS(class),
					"Science":  newSCGS(class),
					"Religion": newSingleGS("Evaluation", class),
				}
			} else if class == "SN" {
				gsMap = map[string]gradingSystem{
					"Arabic":      newGGS(class),
					"English":     newEGS(class),
					"Math":        newMGS(class),
					"Science":     newSCGS(class),
					"Religion":    newGGS(class),
					"Citizenship": newCitizenshipGS(class),
					"Computer":    newSingleGS("Evaluation", class),
				}
			}
		} else {
			// class is numeric
			gsMap = map[string]gradingSystem{
				"Arabic":      newGGS(class),
				"English":     newEGS(class),
				"Math":        newMGS(class),
				"Citizenship": newCitizenshipGS(class),
			}
			if intClass <= 8 {
				gsMap["Social Studies"] = newGGS(class)
				gsMap["Science"] = newSCGS(class)
			} else {
				gsMap["Biology"] = newSCGS(class)
				gsMap["Chemistry"] = newSCGS(class)
				gsMap["Physics"] = newSCGS(class)
			}
			if intClass <= 5 {
				gsMap["Religion"] = newGGS(class)
				gsMap["Islamic Studies"] = newSingleGS("Evaluation", class)
				if intClass >= 2 {
					gsMap["Computer"] = newComputer2to5GS(class)
				}
			} else {
				gsMap["Religion"] = newReligion7GS(class)
				gsMap["Computer"] = newComputer6to12GS(class)
			}
		}

		gsMap["P.E."] = peGradingSystem{}
		gsMap["Behavior"] = behaviorGradingSystem{}
		gradingSystems[class] = gsMap
	}

	classGradingSystem, ok := gradingSystems[class]
	if !ok {
		return nil
	}

	return classGradingSystem[subject]
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
		m[0] = ggs.get100(Term{Semester, 1}, marks) / 2.0
		m[1] = ggs.get100(Term{Semester, 2}, marks) / 2.0

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

// mathGradingSystem is used for Math:
// [5:Homework] [5:Daily Work] [20:Mental Math]
// [50:best 5 quizzes out of 6] [20:Quarter Exam]
type mathGradingSystem struct {
	qWeight float64
	sWeight float64
}

func newMGS(class string) gradingSystem {
	q, s := classWeights(class)
	return mathGradingSystem{q, s}
}

func (mgs mathGradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return []colDescription{
			{"Homework", 5, true},
			{"Daily Work", 5, true},
			{"Mental Math", 20, true},
			{"Quiz 1", 10, true},
			{"Quiz 2", 10, true},
			{"Quiz 3", 10, true},
			{"Quiz 4", 10, true},
			{"Quiz 5", 10, true},
			{"Quiz 6", 10, true},
			{"Best 5 Quizzes", 50, false},
			{"Quarter Exam", 20, true},
			{"Quarter Mark", 100, false},
			{"Quarter %", mgs.qWeight, false},
		}
	} else if term.Typ == Semester {
		return []colDescription{
			// TODO: per-subject exam marks
			{"Semester Exam", 100, true},
			{"Semester Exam %", mgs.sWeight, false},
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

func (mgs mathGradingSystem) evaluate(term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := mgs.description(term)

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
		m[12] = m[11] * mgs.qWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam %
		m[1] = m[0] * mgs.sWeight / 100.0

		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		mgs.evaluate(Term{Quarter, q1}, marks)
		mgs.evaluate(Term{Quarter, q2}, marks)
		q1Mark := marks[Term{Quarter, q1}][12]
		q2Mark := marks[Term{Quarter, q2}][12]

		m[2] = sumMarks(m[1], q1Mark, q2Mark)
	} else if term.Typ == EndOfYear {
		mgs.evaluate(Term{Semester, 1}, marks)
		mgs.evaluate(Term{Semester, 2}, marks)
		m[0] = mgs.get100(Term{Semester, 1}, marks) / 2.0
		m[1] = mgs.get100(Term{Semester, 2}, marks) / 2.0

		m[2] = sumMarks(m[0], m[1])
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

func (mgs mathGradingSystem) get100(term Term, marks studentMarks) float64 {
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

func (mgs mathGradingSystem) getExam(term Term, marks studentMarks) float64 {
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

func (mgs mathGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.Signbit(m[len(m)-1])
}

func (mgs mathGradingSystem) quarterWeight() float64 {
	return mgs.qWeight
}

func (mgs mathGradingSystem) semesterWeight() float64 {
	return mgs.sWeight
}

// englishGradingSystem is used for English:
// [5:Homework] [5:Daily Work] [10:Reading] [10:Writing]
// [50:best 5 quizzes out of 6] [20:Quarter Exam]
type englishGradingSystem struct {
	qWeight float64
	sWeight float64
}

func newEGS(class string) gradingSystem {
	q, s := classWeights(class)
	return englishGradingSystem{q, s}
}

func (egs englishGradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return []colDescription{
			{"Homework", 5, true},
			{"Daily Work", 5, true},
			{"Reading", 10, true},
			{"Writing", 10, true},
			{"Quiz 1", 10, true},
			{"Quiz 2", 10, true},
			{"Quiz 3", 10, true},
			{"Quiz 4", 10, true},
			{"Quiz 5", 10, true},
			{"Quiz 6", 10, true},
			{"Best 5 Quizzes", 50, false},
			{"Quarter Exam", 20, true},
			{"Quarter Mark", 100, false},
			{"Quarter %", egs.qWeight, false},
		}
	} else if term.Typ == Semester {
		return []colDescription{
			// TODO: per-subject exam marks
			{"Semester Exam", 100, true},
			{"Semester Exam %", egs.sWeight, false},
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

func (egs englishGradingSystem) evaluate(term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := egs.description(term)

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
		m[10] = sumQuizzes(m[4:10]...)

		// Quarter mark
		m[12] = sumMarks(m[0], m[1], m[2], m[3], m[10], m[11])

		// Quarter %
		m[13] = m[12] * egs.qWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam %
		m[1] = m[0] * egs.sWeight / 100.0

		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		egs.evaluate(Term{Quarter, q1}, marks)
		egs.evaluate(Term{Quarter, q2}, marks)
		q1Mark := marks[Term{Quarter, q1}][13]
		q2Mark := marks[Term{Quarter, q2}][13]

		m[2] = sumMarks(m[1], q1Mark, q2Mark)
	} else if term.Typ == EndOfYear {
		egs.evaluate(Term{Semester, 1}, marks)
		egs.evaluate(Term{Semester, 2}, marks)
		m[0] = egs.get100(Term{Semester, 1}, marks) / 2.0
		m[1] = egs.get100(Term{Semester, 2}, marks) / 2.0

		m[2] = sumMarks(m[0], m[1])
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

func (egs englishGradingSystem) get100(term Term, marks studentMarks) float64 {
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

func (egs englishGradingSystem) getExam(term Term, marks studentMarks) float64 {
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

func (egs englishGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.Signbit(m[len(m)-1])
}

func (egs englishGradingSystem) quarterWeight() float64 {
	return egs.qWeight
}

func (egs englishGradingSystem) semesterWeight() float64 {
	return egs.sWeight
}

// scienceGradingSystem is used for Science:
// [5:Homework] [5:Daily Work] [10:Definitions] [10:Experiment]
// [50:best 5 quizzes out of 6] [20:Quarter Exam]
type scienceGradingSystem struct {
	qWeight float64
	sWeight float64
}

func newSCGS(class string) gradingSystem {
	q, s := classWeights(class)
	return scienceGradingSystem{q, s}
}

func (scgs scienceGradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return []colDescription{
			{"Homework", 5, true},
			{"Daily Work", 5, true},
			{"Definitions", 10, true},
			{"Experiment / practical", 10, true},
			{"Quiz 1", 10, true},
			{"Quiz 2", 10, true},
			{"Quiz 3", 10, true},
			{"Quiz 4", 10, true},
			{"Quiz 5", 10, true},
			{"Quiz 6", 10, true},
			{"Best 5 Quizzes", 50, false},
			{"Quarter Exam", 20, true},
			{"Quarter Mark", 100, false},
			{"Quarter %", scgs.qWeight, false},
		}
	} else if term.Typ == Semester {
		return []colDescription{
			// TODO: per-subject exam marks
			{"Semester Exam", 100, true},
			{"Semester Exam %", scgs.sWeight, false},
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

func (scgs scienceGradingSystem) evaluate(term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := scgs.description(term)

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
		m[10] = sumQuizzes(m[4:10]...)

		// Quarter mark
		m[12] = sumMarks(m[0], m[1], m[2], m[3], m[10], m[11])

		// Quarter %
		m[13] = m[12] * scgs.qWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam %
		m[1] = m[0] * scgs.sWeight / 100.0

		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		scgs.evaluate(Term{Quarter, q1}, marks)
		scgs.evaluate(Term{Quarter, q2}, marks)
		q1Mark := marks[Term{Quarter, q1}][13]
		q2Mark := marks[Term{Quarter, q2}][13]

		m[2] = sumMarks(m[1], q1Mark, q2Mark)
	} else if term.Typ == EndOfYear {
		scgs.evaluate(Term{Semester, 1}, marks)
		scgs.evaluate(Term{Semester, 2}, marks)
		m[0] = scgs.get100(Term{Semester, 1}, marks) / 2.0
		m[1] = scgs.get100(Term{Semester, 2}, marks) / 2.0

		m[2] = sumMarks(m[0], m[1])
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

func (scgs scienceGradingSystem) get100(term Term, marks studentMarks) float64 {
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

func (scgs scienceGradingSystem) getExam(term Term, marks studentMarks) float64 {
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

func (scgs scienceGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.Signbit(m[len(m)-1])
}

func (scgs scienceGradingSystem) quarterWeight() float64 {
	return scgs.qWeight
}

func (scgs scienceGradingSystem) semesterWeight() float64 {
	return scgs.sWeight
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
		m[0] = pgs.get100(Term{Semester, 1}, marks) / 2.0
		m[1] = pgs.get100(Term{Semester, 2}, marks) / 2.0

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
			{"Daily Work", 20, true},
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
			{"Daily Work", 15, true},
			{"Homework", 5, true},
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

	if term.Typ == Quarter {
		sum := sumMarks(m[0 : len(m)-2]...)
		m[len(m)-2] = sum
		m[len(m)-1] = sum * sgs.qWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam %
		m[1] = m[0] * sgs.sWeight / 100.0

		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		sgs.evaluate(Term{Quarter, q1}, marks)
		sgs.evaluate(Term{Quarter, q2}, marks)
		q1m := marks[Term{Quarter, q1}]
		q1Mark := q1m[len(q1m)-1]
		q2m := marks[Term{Quarter, q2}]
		q2Mark := q2m[len(q2m)-1]

		m[2] = sumMarks(m[1], q1Mark, q2Mark)
	} else if term.Typ == EndOfYear {
		sgs.evaluate(Term{Semester, 1}, marks)
		sgs.evaluate(Term{Semester, 2}, marks)
		m[0] = sgs.get100(Term{Semester, 1}, marks) / 2.0
		m[1] = sgs.get100(Term{Semester, 2}, marks) / 2.0

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

// computer2to5GradingSystem is computer from grades 2 to 5
// Quarters [Homework: 5, Participation: 5, Behavior: 10, Project1: 10, Project2: 10
// Exam: 30, Practical: 30]
// Semesters [Written: 25, Practical: 25]
type computer2to5GradingSystem struct {
	qWeight float64
	sWeight float64
}

func newComputer2to5GS(class string) gradingSystem {
	q, s := classWeights(class)
	return computer2to5GradingSystem{
		q,
		s,
	}
}

func (c2gs computer2to5GradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return []colDescription{
			{"Homework", 5, true},
			{"Participation", 5, true},
			{"Behavior", 10, true},
			{"Project1", 10, true},
			{"Project2", 10, true},
			{"Quarter Exam", 30, true},
			{"Practical Exam", 30, true},
			{"Quarter Mark", 100, false},
			{"Quarter %", c2gs.qWeight, false},
		}
	} else if term.Typ == Semester {
		return []colDescription{
			{"Written Exam", 25, true},
			{"Practical Exam", 25, true},
			{"Semester Exam", 100, false},
			{"Semester Exam %", c2gs.sWeight, false},
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

func (c2gs computer2to5GradingSystem) evaluate(term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := c2gs.description(term)

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
		sum := sumMarks(m[0 : len(m)-2]...)
		m[len(m)-2] = sum
		m[len(m)-1] = sum * c2gs.qWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam 100
		m[2] = (m[0] + m[1]) * 2

		// Semester Exam %
		m[3] = m[2] * c2gs.sWeight / 100.0

		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		c2gs.evaluate(Term{Quarter, q1}, marks)
		c2gs.evaluate(Term{Quarter, q2}, marks)
		q1m := marks[Term{Quarter, q1}]
		q1Mark := q1m[len(q1m)-1]
		q2m := marks[Term{Quarter, q2}]
		q2Mark := q2m[len(q2m)-1]

		m[4] = sumMarks(m[3], q1Mark, q2Mark)
	} else if term.Typ == EndOfYear {
		c2gs.evaluate(Term{Semester, 1}, marks)
		c2gs.evaluate(Term{Semester, 2}, marks)
		m[0] = c2gs.get100(Term{Semester, 1}, marks) / 2.0
		m[1] = c2gs.get100(Term{Semester, 2}, marks) / 2.0

		m[2] = sumMarks(m[0], m[1])
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

func (c2gs computer2to5GradingSystem) get100(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return m[len(m)-2]
	} else if term.Typ == Semester {
		return m[len(m)-1]
	} else if term.Typ == EndOfYear {
		return m[len(m)-1]
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (c2gs computer2to5GradingSystem) getExam(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return m[len(m)-3]
	} else if term.Typ == Semester {
		return m[len(m)-2]
	} else if term.Typ == EndOfYear {
		return negZero
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (c2gs computer2to5GradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.Signbit(m[len(m)-1])
}

func (c2gs computer2to5GradingSystem) quarterWeight() float64 {
	return c2gs.qWeight
}

func (c2gs computer2to5GradingSystem) semesterWeight() float64 {
	return c2gs.sWeight
}

// computer6to12GradingSystem is computer from grades 6 to 12
// Quarters [Homework: 5, Participation: 5, Behavior: 5
// Quizzes: 50, Exam: 20, Practical: 15]
// Semesters [Written: 25, Practical: 25]
type computer6to12GradingSystem struct {
	qWeight float64
	sWeight float64
}

func newComputer6to12GS(class string) gradingSystem {
	q, s := classWeights(class)
	return computer6to12GradingSystem{q, s}
}

func (c6gs computer6to12GradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return []colDescription{
			{"Homework", 5, true},
			{"Participation", 5, true},
			{"Behavior", 5, true},
			{"Quiz 1", 10, true},
			{"Quiz 2", 10, true},
			{"Quiz 3", 10, true},
			{"Quiz 4", 10, true},
			{"Quiz 5", 10, true},
			{"Quiz 6", 10, true},
			{"Best 5 Quizzes", 50, false},
			{"Quarter Exam", 20, true},
			{"Practical Exam", 15, true},
			{"Quarter Mark", 100, false},
			{"Quarter %", c6gs.qWeight, false},
		}
	} else if term.Typ == Semester {
		return []colDescription{
			{"Written Exam", 25, true},
			{"Practical Exam", 25, true},
			{"Semester Exam", 100, false},
			{"Semester Exam %", c6gs.sWeight, false},
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

func (c6gs computer6to12GradingSystem) evaluate(term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := c6gs.description(term)

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
		m[12] = sumMarks(m[0], m[1], m[2], m[9], m[10], m[11])

		// Quarter %
		m[13] = m[12] * c6gs.qWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam 100
		m[2] = (m[0] + m[1]) * 2

		// Semester Exam %
		m[3] = m[2] * c6gs.sWeight / 100.0

		// Semester Mark
		q2 := uint(term.N * 2)
		q1 := q2 - 1
		c6gs.evaluate(Term{Quarter, q1}, marks)
		c6gs.evaluate(Term{Quarter, q2}, marks)
		q1m := marks[Term{Quarter, q1}]
		q1Mark := q1m[len(q1m)-1]
		q2m := marks[Term{Quarter, q2}]
		q2Mark := q2m[len(q2m)-1]

		m[4] = sumMarks(m[3], q1Mark, q2Mark)
	} else if term.Typ == EndOfYear {
		c6gs.evaluate(Term{Semester, 1}, marks)
		c6gs.evaluate(Term{Semester, 2}, marks)
		m[0] = c6gs.get100(Term{Semester, 1}, marks) / 2.0
		m[1] = c6gs.get100(Term{Semester, 2}, marks) / 2.0

		m[2] = sumMarks(m[0], m[1])
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

func (c6gs computer6to12GradingSystem) get100(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return m[len(m)-2]
	} else if term.Typ == Semester {
		return m[len(m)-1]
	} else if term.Typ == EndOfYear {
		return m[len(m)-1]
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (c6gs computer6to12GradingSystem) getExam(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return m[len(m)-3] + m[len(m)-2]
	} else if term.Typ == Semester {
		return m[len(m)-2]
	} else if term.Typ == EndOfYear {
		return negZero
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (c6gs computer6to12GradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.Signbit(m[len(m)-1])
}

func (c6gs computer6to12GradingSystem) quarterWeight() float64 {
	return c6gs.qWeight
}

func (c6gs computer6to12GradingSystem) semesterWeight() float64 {
	return c6gs.sWeight
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
		// class is KG or SN
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
		if mark >= l.minMark {
			return l.letter
		}
	}
	// something wrong with the letterSystem
	return "Error"
}

// subjectInAverage returns true if subject should be calculated in average
func subjectInAverage(subject, class string) bool {
	if (class == "KG1" || class == "KG2" || class == "PreKG") && subject == "Religion" {
		return false
	}

	if subject == "Computer" {
		intClass, err := strconv.Atoi(class)
		if err == nil && intClass >= 9 {
			return true
		}
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
