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
	EndOfYearGpa
	Midterm
	WeekS1
	WeekS2
)

var termStrings = map[termType]string{
	Quarter:      "Quarter",
	Semester:     "Semester",
	EndOfYear:    "End of Year",
	EndOfYearGpa: "End of Year (GPA)",
	Midterm:      "Midterm",
	WeekS1:       "S1 Week",
	WeekS2:       "S2 Week",
}

type Term struct {
	Typ termType
	N   int
}

var terms = []Term{
	{Quarter, 1},
	{Quarter, 2},
	{Midterm, 1},
	{Semester, 1},
	{Quarter, 3},
	{Quarter, 4},
	{Midterm, 2},
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

type semesterType int

const (
	QuarterSemester semesterType = iota + 1
	MidtermSemester
)

var semesterTypes = []semesterType{
	QuarterSemester,
	MidtermSemester,
}

var semesterTypeStrings = map[semesterType]string{
	QuarterSemester: "Quarter",
	MidtermSemester: "Midterm",
}

func (st semesterType) Value() int {
	return int(st)
}

func (st semesterType) String() string {
	s, ok := semesterTypeStrings[st]
	if !ok {
		panic(fmt.Sprintf("Invalid semesterType type: %d", st))
	}
	return s
}

type colDescription struct {
	Name        string
	Max         float64
	FinalWeight float64
	Editable    bool
}

type studentMarks map[Term][]float64

type gradingSystem interface {
	description(term Term) []colDescription
	evaluate(c context.Context, studentID, sy string, term Term, marks studentMarks) error
	get100(term Term, marks studentMarks) float64
	getExam(term Term, marks studentMarks) float64
	ready(term Term, marks studentMarks) bool

	quarterWeight() float64
	semesterWeight() float64
	subjectInAverage() bool
	displayName() string
}

type letterType struct {
	letter      string
	description string
	minMark     float64
}

func getGradingSystem(c context.Context, sy, class, subjectname string) gradingSystem {
	if subjectname == "Behavior" {
		return behaviorGradingSystem{}
	}
	// TODO: err
	subject, err := getSubject(c, sy, class, subjectname)
	if err != nil {
		log.Errorf(c, "Could not get subject %s %s %s: %s", sy, class, subjectname, err)
		return nil
	}

	var qWeight, sWeight float64
	found := false
	for _, classSetting := range getClassSettings(c, sy) {
		if classSetting.Class != class {
			continue
		}

		qWeight = classSetting.QuarterWeight
		if qWeight > 50 {
			qWeight = 50
		}
		if qWeight < 0 {
			qWeight = 0
		}

		sWeight = 100 - qWeight*2

		found = true
		break
	}
	if !found {
		log.Errorf(c, "Could not get class settings: %s %s", sy, class)
		return nil
	}

	if qWeight == 0 {
		sWeight = 100
	} else if sWeight == 0 {
		qWeight = 50
	}

	if sWeight+qWeight*2 != 100 {
		log.Errorf(c, "Invalid weights for class: %s %s", sy, class)
		return nil
	}

	subject.qWeight = qWeight
	subject.sWeight = sWeight

	return subject
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
	noGrading gradingColumnType = iota
	directGrading
	quizGrading
)

var gradingColumnTypeStrings = map[gradingColumnType]string{
	noGrading:     "Unused",
	directGrading: "Direct",
	quizGrading:   "Quizzes",
}

type gradingColumn struct {
	Type        gradingColumnType
	Name        string
	Max         float64
	FinalWeight float64

	NumQuizzes  int
	BestQuizzes int
}

type Subject struct {
	// TODO: add SY, Class
	ShortName          string
	Description        string
	CalculateInAverage bool
	S1Credits          float64
	S2Credits          float64
	SemesterType       semesterType
	MidtermWeeksS1     int
	TotalWeeksS1       int
	MidtermWeeksS2     int
	TotalWeeksS2       int

	WeeklyGradingColumns   []gradingColumn
	QuarterGradingColumns  []gradingColumn
	SemesterGradingColumns []gradingColumn

	qWeight float64 `datastore:"-"`
	sWeight float64 `datastore:"-"`
}

func (s Subject) description(term Term) []colDescription {
	// TODO: check total max = 100
	if term.Typ == Quarter {
		if s.SemesterType != QuarterSemester {
			return nil
		}
		var cols []colDescription
		for _, gcol := range s.QuarterGradingColumns {
			if gcol.Type == directGrading {
				cols = append(cols, colDescription{gcol.Name, gcol.Max, gcol.FinalWeight, true})
			} else if gcol.Type == quizGrading {
				cols = append(cols, quizColDescriptions(gcol, false)...)
			}
		}
		cols = append(cols, colDescription{"Quarter Mark", 100, math.NaN(), false})
		cols = append(cols, colDescription{"Quarter %", s.qWeight, math.NaN(), false})
		return cols

	} else if term.Typ == WeekS1 || term.Typ == WeekS2 {
		if s.SemesterType != MidtermSemester {
			return nil
		}
		if (term.Typ == WeekS1 && term.N > s.TotalWeeksS1) ||
			(term.Typ == WeekS2 && term.N > s.TotalWeeksS2) {
			return nil
		}
		var cols []colDescription
		for _, gcol := range s.WeeklyGradingColumns {
			if gcol.Type == directGrading {
				cols = append(cols, colDescription{gcol.Name, gcol.Max, math.NaN(), true})
			}
		}
		return cols

	} else if term.Typ == Midterm {
		if s.SemesterType != MidtermSemester {
			return nil
		}
		var cols []colDescription

		for _, gcol := range s.WeeklyGradingColumns {
			if gcol.Type == directGrading {
				cols = append(cols, colDescription{gcol.Name, 100.0, 100.0, false})
			}
		}

		for _, gcol := range s.QuarterGradingColumns {
			if gcol.Type == directGrading {
				cols = append(cols, colDescription{gcol.Name, gcol.Max, 100.0, true})
			} else if gcol.Type == quizGrading {
				cols = append(cols, quizColDescriptions(gcol, true)...)
			}
		}
		cols = append(cols, colDescription{"Midterm Mark", 100, math.NaN(), false})
		return cols

	} else if term.Typ == Semester {
		var cols []colDescription
		if s.SemesterType == MidtermSemester {
			for _, gcol := range s.WeeklyGradingColumns {
				if gcol.Type == directGrading {
					cols = append(cols, colDescription{gcol.Name, gcol.FinalWeight, gcol.FinalWeight, false})
				}
			}

			for _, gcol := range s.QuarterGradingColumns {
				if gcol.Type == directGrading {
					cols = append(cols, colDescription{gcol.Name, gcol.Max, gcol.FinalWeight, false})
				} else if gcol.Type == quizGrading {
					cols = append(cols, colDescription{gcol.Name,
						gcol.Max * float64(gcol.BestQuizzes), gcol.FinalWeight, false})
				}
			}
		}
		for _, gcol := range s.SemesterGradingColumns {
			if gcol.Type == directGrading {
				cols = append(cols, colDescription{gcol.Name, gcol.Max, gcol.FinalWeight, true})
			} else if gcol.Type == quizGrading {
				cols = append(cols, quizColDescriptions(gcol, false)...)
			}
		}
		if s.SemesterType == QuarterSemester {
			cols = append(cols, colDescription{"Semester %", s.sWeight, math.NaN(), false})
		}
		cols = append(cols, colDescription{"Semester Mark", 100, math.NaN(), false})
		return cols

	} else if term.Typ == EndOfYear {
		return []colDescription{
			{"Semester 1 %", 100, 50, false},
			{"Semester 2 %", 100, 50, false},
			{"Final mark", 100, math.NaN(), false},
		}
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}
}

func quizColDescriptions(gc gradingColumn, midterm bool) []colDescription {
	if gc.NumQuizzes < 0 || gc.BestQuizzes > gc.NumQuizzes {
		// invalid, so return empty columns
		return []colDescription{}
	}

	var cols []colDescription
	for i := 0; i < gc.NumQuizzes; i++ {
		cols = append(cols, colDescription{
			fmt.Sprintf("%s %d", gc.Name, i+1),
			gc.Max,
			math.NaN(),
			true,
		})
	}

	var weight float64
	if midterm {
		weight = 100.0
	} else {
		weight = gc.FinalWeight
	}

	cols = append(cols, colDescription{
		fmt.Sprintf("%s (Best %d)", gc.Name, gc.BestQuizzes),
		gc.Max * float64(gc.BestQuizzes),
		weight,
		false,
	})

	return cols
}

func (s Subject) evaluate(c context.Context, studentID, sy string, term Term, marks studentMarks) error {
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

	marks[term] = m

	if term.Typ == Quarter {
		if s.SemesterType != QuarterSemester {
			return nil
		}
		total100 := 0.0
		nextMark := 0
		for _, gcol := range s.QuarterGradingColumns {
			if gcol.Type == directGrading {
				total100 += m[nextMark] * gcol.FinalWeight / gcol.Max
				nextMark++
			} else if gcol.Type == quizGrading {
				totalQuiz := quizSum(gcol.BestQuizzes, m[nextMark:nextMark+gcol.NumQuizzes])
				nextMark += gcol.NumQuizzes
				m[nextMark] = totalQuiz
				nextMark++

				total100 += totalQuiz * gcol.FinalWeight /
					(float64(gcol.BestQuizzes) * gcol.Max)
			}
		}

		if len(s.QuarterGradingColumns) == 0 {
			total100 = math.NaN()
		}

		// Quarter mark
		m[nextMark] = total100
		nextMark++

		// Quarter %
		m[nextMark] = total100 * s.qWeight / 100.0

	} else if term.Typ == WeekS1 || term.Typ == WeekS2 {
		// No calculations
	} else if term.Typ == Midterm {
		if s.SemesterType != MidtermSemester {
			return nil
		}
		totalMark := 0.0
		totalWeight := 0.0
		nextMark := 0
		if len(s.WeeklyGradingColumns) > 0 {
			var weekMarks [][]float64
			if term.N == 1 {
				for i := 1; i <= s.MidtermWeeksS1; i++ {
					wm, err := getWeeklyStudentMarks(c, studentID, sy, s.ShortName, Term{WeekS1, i}, s)
					if err != nil {
						weekMarks = append(weekMarks, nil)
						continue
					}
					weekMarks = append(weekMarks, wm)
				}
			} else if term.N == 2 {
				for i := 1; i <= s.MidtermWeeksS2; i++ {
					wm, err := getWeeklyStudentMarks(c, studentID, sy, s.ShortName, Term{WeekS2, i}, s)
					if err != nil {
						weekMarks = append(weekMarks, nil)
						continue
					}
					weekMarks = append(weekMarks, wm)
				}
			}
			for i, gcol := range s.WeeklyGradingColumns {
				colTotal := 0.0
				for _, wm := range weekMarks {
					colTotal += wm[i]
				}
				m[nextMark] = colTotal * 100.0 / (gcol.Max * float64(len(weekMarks)))
				totalMark += m[nextMark]
				totalWeight += 100.0
				nextMark++
			}
		}
		for _, gcol := range s.QuarterGradingColumns {
			if gcol.Type == directGrading {
				totalMark += m[nextMark] * 100.0 / gcol.Max
				totalWeight += 100.0
				nextMark++
			} else if gcol.Type == quizGrading {
				totalQuiz := quizSum(gcol.BestQuizzes, m[nextMark:nextMark+gcol.NumQuizzes])
				nextMark += gcol.NumQuizzes
				m[nextMark] = totalQuiz
				nextMark++

				totalMark += totalQuiz * 100.0 /
					(float64(gcol.BestQuizzes) * gcol.Max)
				totalWeight += 100.0
			}
		}

		if len(s.QuarterGradingColumns) == 0 {
			totalMark = math.NaN()
		}

		// Midterm mark
		m[nextMark] = totalMark / totalWeight * 100.0

	} else if term.Typ == Semester {
		total100 := 0.0
		nextMark := 0
		if s.SemesterType == MidtermSemester {
			if len(s.WeeklyGradingColumns) > 0 {
				var weekMarks [][]float64
				if term.N == 1 {
					for i := 1; i <= s.TotalWeeksS1; i++ {
						wm, err := getWeeklyStudentMarks(c, studentID, sy, s.ShortName, Term{WeekS1, i}, s)
						if err != nil {
							weekMarks = append(weekMarks, nil)
							continue
						}
						weekMarks = append(weekMarks, wm)
					}
				} else if term.N == 2 {
					for i := 1; i <= s.TotalWeeksS2; i++ {
						wm, err := getWeeklyStudentMarks(c, studentID, sy, s.ShortName, Term{WeekS2, i}, s)
						if err != nil {
							weekMarks = append(weekMarks, nil)
							continue
						}
						weekMarks = append(weekMarks, wm)
					}
				}
				for i, gcol := range s.WeeklyGradingColumns {
					colTotal := 0.0
					for _, wm := range weekMarks {
						colTotal += wm[i]
					}
					m[nextMark] = colTotal * gcol.FinalWeight / (gcol.Max * float64(len(weekMarks)))
					total100 += m[nextMark]
					nextMark++
				}
			}

			midterm := Term{Midterm, term.N}
			s.evaluate(c, studentID, sy, midterm, marks)
			midtermMarks := marks[midterm]
			midtermNextMark := 0
			for _, gcol := range s.QuarterGradingColumns {
				if gcol.Type == directGrading {
					m[nextMark] =
						midtermMarks[midtermNextMark]
					total100 += m[nextMark] * gcol.FinalWeight / gcol.Max
					nextMark++
					midtermNextMark++
				} else if gcol.Type == quizGrading {
					midtermNextMark += gcol.NumQuizzes
					m[nextMark] = midtermMarks[midtermNextMark]
					total100 += m[nextMark] * gcol.FinalWeight /
						(float64(gcol.BestQuizzes) * gcol.Max)

					nextMark++
					midtermNextMark++
				}
			}
		}
		for _, gcol := range s.SemesterGradingColumns {
			if gcol.Type == directGrading {
				total100 += m[nextMark] * gcol.FinalWeight / gcol.Max
				nextMark++
			} else if gcol.Type == quizGrading {
				totalQuiz := quizSum(gcol.BestQuizzes, m[nextMark:nextMark+gcol.NumQuizzes])
				nextMark += gcol.NumQuizzes
				m[nextMark] = totalQuiz
				total100 += totalQuiz * gcol.FinalWeight /
					(float64(gcol.BestQuizzes) * gcol.Max)

				nextMark++
			}
		}

		if s.SemesterType == QuarterSemester {
			// Semester Exam % (or just Semester % if there are columns other than exam)
			m[nextMark] = total100 * s.sWeight / 100.0
			nextMark++

			// Semester Mark
			q2 := term.N * 2
			q1 := q2 - 1

			s.evaluate(c, studentID, sy, Term{Quarter, q1}, marks)
			s.evaluate(c, studentID, sy, Term{Quarter, q2}, marks)

			if len(s.QuarterGradingColumns) == 0 {
				m[nextMark] = total100
			} else {
				q1Marks := marks[Term{Quarter, q1}]
				q1Mark := q1Marks[len(q1Marks)-1]

				q2Marks := marks[Term{Quarter, q2}]
				q2Mark := q2Marks[len(q2Marks)-1]

				m[nextMark] = sumMarks(m[nextMark-1], q1Mark, q2Mark)
			}
		} else if s.SemesterType == MidtermSemester {
			// Semester Mark
			m[nextMark] = total100
			nextMark++
		}

	} else if term.Typ == EndOfYear {
		s.evaluate(c, studentID, sy, Term{Semester, 1}, marks)
		s.evaluate(c, studentID, sy, Term{Semester, 2}, marks)
		m[0] = s.get100(Term{Semester, 1}, marks)
		m[1] = s.get100(Term{Semester, 2}, marks)

		m[2] = sumMarks(m[0], m[1]) / 2.0
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
		if s.SemesterType != QuarterSemester {
			return math.NaN()
		}
		return m[len(m)-2]
	} else if term.Typ == WeekS2 || term.Typ == WeekS1 {
		return math.NaN()
	} else if term.Typ == Midterm {
		if s.SemesterType != MidtermSemester {
			return math.NaN()
		}
		return m[len(m)-1]
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
	} else if term.Typ == WeekS2 || term.Typ == WeekS1 {
		return math.NaN()
	} else if term.Typ == Midterm {
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

func (s Subject) subjectInAverage() bool {
	return s.CalculateInAverage
}

func (s Subject) displayName() string {
	return s.Description
}

// behaviorGradingSystem contains behavrior. There are no calculations to make
type behaviorGradingSystem struct {
}

var behaviorDesc = []colDescription{
	{"Follows school guidelines for safe and appropriate behaviour", 4, 4, true},
	{"Demonstrates courtesy and respect", 4, 4, true},
	{"Listens and responds", 4, 4, true},
	{"Strives for quality work", 4, 4, true},
	{"Shows initiative / is a self - starter", 4, 4, true},
	{"Participates enthusiastically in activities", 4, 4, true},
	{"Uses time efficiently and appropriately", 4, 4, true},
	{"Completes class work on time ", 4, 4, true},
	{"Contributes to discussion and group tasks", 4, 4, true},
	{"Works cooperatively with others", 4, 4, true},
	{"Works well independently", 4, 4, true},
	{"Returns complete homework", 4, 4, true},
	{"Organizes shelf, materials and belongings ", 4, 4, true},
	{"Asks questions to clarify content", 4, 4, true},
	{"Clearly communicates to teachers ", 4, 4, true},
}

func (behaviorGradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return behaviorDesc
	} else if term.Typ == Midterm {
		return behaviorDesc
	} else if term.Typ == Semester || term.Typ == EndOfYear ||
		term.Typ == WeekS1 || term.Typ == WeekS2 {
		// No calculations
		return nil
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (bgs behaviorGradingSystem) evaluate(c context.Context, studentID, sy string, term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := bgs.description(term)

	switch {
	case m == nil: // first time to evaluate it
		m = make([]float64, len(desc))
		for i, _ := range desc {
			m[i] = math.NaN()
		}
	case len(m) != len(desc): // sanity check
		err = invalidNumberOfMarks
		m = make([]float64, len(desc))
		for i, _ := range desc {
			m[i] = math.NaN()
		}
	}

	// more sanity checks
	for i, d := range desc {
		if m[i] < 0 || m[i] > d.Max {
			m[i] = math.NaN()
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
	return math.NaN()
}

func (bgs behaviorGradingSystem) getExam(term Term, marks studentMarks) float64 {
	return math.NaN()
}

func (bgs behaviorGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	for _, v := range m {
		if math.IsNaN(v) {
			// mark not entered
			return false
		}
	}

	return true
}

func (bgs behaviorGradingSystem) quarterWeight() float64 {
	return math.NaN()
}

func (bgs behaviorGradingSystem) semesterWeight() float64 {
	return math.NaN()
}

func (bgs behaviorGradingSystem) displayName() string {
	return "Behavior"
}

func (bgs behaviorGradingSystem) subjectInAverage() bool {
	return false
}
