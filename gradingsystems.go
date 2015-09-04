// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine/log"

	"bytes"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

var (
	invalidNumberOfMarks = fmt.Errorf("Invalid number of marks.")
	invalidRangeOfMarks  = fmt.Errorf("Invalid range of marks.")
)

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

type Term struct {
	Typ termType
	N   int
}

var terms = []Term{
	{Quarter, 1},
	{Quarter, 2},
	{Semester, 1},
	{Quarter, 3},
	{Quarter, 4},
	{Semester, 2},
	{EndOfYear, 0},
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

type colDescription struct {
	Name     string
	Max      float64
	Editable bool
}

type studentMarks map[Term][]float64

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

var letterSystemMap = map[string]letterSystem{
	"ABCDF": ABCDF,
	"OVSLU": OVSLU,
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

func getLetterSystem(c context.Context, sy, class string) letterSystem {
	for _, setting := range getClassSettings(c, sy) {
		if setting.Class != class {
			continue
		}

		if ls, ok := letterSystemMap[setting.LetterSystem]; ok {
			return ls
		} else {
			log.Errorf(c, "Invalid letter system of class %s, SY: %s, letter system: %s",
				class, sy, setting.LetterSystem)
			return ABCDF
		}
	}

	// should never happen
	log.Errorf(c, "Could not find letter system of class %s, SY: %s", class, sy)
	return ABCDF
}

func classWeights(c context.Context, sy, class string) (quarter, semester float64) {
	for _, setting := range getClassSettings(c, sy) {
		if setting.Class != class {
			continue
		}

		qw := setting.QuarterWeight
		if qw >= 0 && qw <= 50 {
			return qw, 100 - qw*2
		} else {
			log.Errorf(c, "Invalid quarter weight of class %s, SY: %s, quarter weight: %d",
				class, sy, qw)
			return 40, 20
		}
	}

	// should never happen
	log.Errorf(c, "Could not find quarter weight of class %s, SY: %s", class, sy)
	return 40, 20
}

type gradingColumnType int

const (
	directGrading gradingColumnType = iota + 1
	quizGrading
)

var gradingColumnTypeStrings = map[gradingColumnType]string{
	directGrading: "Direct",
	quizGrading:   "Quizzes",
}

type gradingColumn struct {
	Type gradingColumnType
	Name string
	Max  float64

	NumQuizzes  int
	BestQuizzes int
}

type Subject struct {
	// TODO: add SY, Class
	ShortName          string
	Description        string
	CalculateInAverage bool

	QuarterGradingColumns  []gradingColumn
	SemesterGradingColumns []gradingColumn

	qWeight float64 `datastore:"-"`
	sWeight float64 `datastore:"-"`
}

func (s Subject) description(term Term) []colDescription {
	// TODO: check total max = 100
	if term.Typ == Quarter {
		var cols []colDescription
		for _, gcol := range s.QuarterGradingColumns {
			if gcol.Type == directGrading {
				cols = append(cols, colDescription{gcol.Name, gcol.Max, true})
			} else if gcol.Type == quizGrading {
				cols = append(cols, quizColDescriptions(gcol)...)
			}
		}
		cols = append(cols, colDescription{"Quarter Mark", 100, false})
		cols = append(cols, colDescription{"Quarter %", s.qWeight, false})
		return cols

	} else if term.Typ == Semester {
		var cols []colDescription
		for _, gcol := range s.SemesterGradingColumns {
			if gcol.Type == directGrading {
				cols = append(cols, colDescription{gcol.Name, gcol.Max, true})
			} else if gcol.Type == quizGrading {
				cols = append(cols, quizColDescriptions(gcol)...)
			}
		}
		cols = append(cols, colDescription{"Semester %", s.sWeight, false})
		cols = append(cols, colDescription{"Semester Mark", 100, false})
		return cols

	} else if term.Typ == EndOfYear {
		return []colDescription{
			{"Semester 1 %", 50, false},
			{"Semester 2 %", 50, false},
			{"Final mark", 100, false},
		}
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}
}

func quizColDescriptions(gc gradingColumn) []colDescription {
	if gc.NumQuizzes < 0 || gc.BestQuizzes > gc.NumQuizzes {
		// invalid, so return empty columns
		return []colDescription{}
	}

	var cols []colDescription
	for i := 0; i < gc.NumQuizzes; i++ {
		cols = append(cols, colDescription{
			fmt.Sprintf("%s %d", gc.Name, i+1),
			gc.Max,
			true,
		})
	}
	cols = append(cols, colDescription{
		fmt.Sprintf("%s (Best %d)", gc.Name, gc.BestQuizzes),
		gc.Max * float64(gc.BestQuizzes),
		false,
	})

	return cols
}

func (s Subject) evaluate(term Term, marks studentMarks) error {
	var err error

	m := marks[term]
	cols := s.description(term)

	switch {
	case m == nil: // first time to evaluate it
		m = make([]float64, len(cols))
		for i, _ := range cols {
			m[i] = math.NaN()
		}
	case len(m) != len(cols): // sanity check
		err = invalidNumberOfMarks
		m = make([]float64, len(cols))
		for i, _ := range cols {
			m[i] = math.NaN()
		}
	}

	// more sanity checks
	for i, d := range cols {
		if m[i] < 0 || m[i] > d.Max {
			m[i] = math.NaN()
			if err == nil {
				err = invalidRangeOfMarks
			}
		}
	}

	if term.Typ == Quarter {
		total100 := 0.0
		nextMark := 0
		for _, gcol := range s.QuarterGradingColumns {
			if gcol.Type == directGrading {
				total100 += m[nextMark]
				nextMark++
			} else if gcol.Type == quizGrading {
				totalQuiz := quizSum(gcol.BestQuizzes, m[nextMark:nextMark+gcol.NumQuizzes])
				nextMark += gcol.NumQuizzes
				m[nextMark] = totalQuiz
				nextMark++

				total100 += totalQuiz
			}
		}

		// Quarter mark
		m[nextMark] = total100
		nextMark++

		// Quarter %
		m[nextMark] = total100 * s.qWeight / 100.0

	} else if term.Typ == Semester {
		total100 := 0.0
		nextMark := 0
		for _, gcol := range s.SemesterGradingColumns {
			if gcol.Type == directGrading {
				total100 += m[nextMark]
				nextMark++
			} else if gcol.Type == quizGrading {
				totalQuiz := quizSum(gcol.BestQuizzes, m[nextMark:nextMark+gcol.NumQuizzes])
				nextMark += gcol.NumQuizzes
				m[nextMark] = totalQuiz
				nextMark++

				total100 += totalQuiz
			}
		}

		// Semester Exam % (or just Semester % if there are columns other than exam)
		m[nextMark] = total100 * s.sWeight / 100.0
		nextMark++

		// Semester Mark
		q2 := term.N * 2
		q1 := q2 - 1

		s.evaluate(Term{Quarter, q1}, marks)
		q1Marks := marks[Term{Quarter, q1}]
		q1Mark := q1Marks[len(q1Marks)-1]

		s.evaluate(Term{Quarter, q2}, marks)
		q2Marks := marks[Term{Quarter, q2}]
		q2Mark := q2Marks[len(q2Marks)-1]

		m[nextMark] = sumMarks(m[nextMark-1], q1Mark, q2Mark)
	} else if term.Typ == EndOfYear {
		s.evaluate(Term{Semester, 1}, marks)
		s.evaluate(Term{Semester, 2}, marks)
		m[0] = s.get100(Term{Semester, 1}, marks) / 2.0
		m[1] = s.get100(Term{Semester, 2}, marks) / 2.0

		m[2] = sumMarks(m[0], m[1])
	} else {
		return fmt.Errorf("Invalid term type: %d", term.Typ)
	}

	marks[term] = m
	// TODO: errors?
	return nil
}

// sumMarks sums all values, and if one of them is math.NaN(), returns math.NaN()
func sumMarks(marks ...float64) float64 {
	var total float64
	for _, v := range marks {
		if math.IsNaN(v) {
			return math.NaN()
		}
		total += v
	}
	return total
}

// quizSum adds the top `keep` marks only and ignores the rest
func quizSum(keep int, marks []float64) float64 {
	marksCopy := append([]float64(nil), marks...)
	sort.Float64s(marksCopy)
	return sumMarks(marksCopy[len(marksCopy)-keep:]...)
}

func (s Subject) get100(term Term, marks studentMarks) float64 {
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

func (s Subject) getExam(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return math.NaN()
	} else if term.Typ == Semester {
		return m[len(m)-2]
	} else if term.Typ == EndOfYear {
		return math.NaN()
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (s Subject) ready(term Term, marks studentMarks) bool {
	return !math.IsNaN(s.get100(term, marks))
}

func (s Subject) quarterWeight() float64 {
	return s.qWeight
}

func (s Subject) semesterWeight() float64 {
	return s.sWeight
}
