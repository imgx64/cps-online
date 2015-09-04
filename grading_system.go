// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"golang.org/x/net/context"

	"fmt"
	"math"
	"strconv"
	"strings"
)

var _subjects = []string{
	"Arabic",
	"English",
	"Math",
	"Science",
	"Biology",
	"Chemistry",
	"Physics",
	"Economics",
	"Accounts",
	"Business Studies",
	"Social Studies",
	"Social Studies (Arabic)",
	"Religion",
	"UCMAS",
	"Citizenship",
	"Computer",
	"P.E.",
	"Speech and Drama",
	"Behavior",
	"Remarks",
}

func getSubjects(c context.Context, sy, class string) ([]string, error) {
	var subjects []string
	for _, subject := range _subjects {
		if getGradingSystem(c, sy, class, subject) != nil {
			subjects = append(subjects, subject)
		}
	}
	return subjects, nil
}

func getAllSubjects(c context.Context, sy string) ([]string, error) {
	return _subjects, nil
}

func getGradingSystem(c context.Context, sy, class, subject string) gradingSystem {
	classes := getClasses(c, sy)

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
					"Arabic":   newGGS(c, sy, class),
					"English":  newEGS(c, sy, class),
					"Math":     newMGS(c, sy, class),
					"Science":  newSCGS(c, sy, class),
					"Religion": newSingleGS(c, sy, "Evaluation", class),
				}
				if class == "KG2" {
					gsMap["UCMAS"] = ucmasGradingSystem{}
				}

			} else if class == "SN" {
				gsMap = map[string]gradingSystem{
					"Arabic":      newGGS(c, sy, class),
					"English":     newEGS(c, sy, class),
					"Math":        newMGS(c, sy, class),
					"Science":     newSCGS(c, sy, class),
					"Religion":    newGGS(c, sy, class),
					"Citizenship": newCitizenshipGS(c, sy, class),
					"Computer":    newSingleGS(c, sy, "Evaluation", class),
				}
			} else if strings.HasSuffix(class, "sci") {
				gsMap = map[string]gradingSystem{
					"Arabic":           newGGS(c, sy, class),
					"English":          newEGS(c, sy, class),
					"Math":             newMGS(c, sy, class),
					"Religion":         newReligion7GS(c, sy, class),
					"Computer":         newComputer6to12GS(c, sy, class),
					"Speech and Drama": newSpeechGS(c, sy, class),

					"Biology":   newSCGS(c, sy, class),
					"Chemistry": newSCGS(c, sy, class),
					"Physics":   newSCGS(c, sy, class),
				}
			} else if strings.HasSuffix(class, "com") {
				gsMap = map[string]gradingSystem{
					"Arabic":           newGGS(c, sy, class),
					"English":          newEGS(c, sy, class),
					"Math":             newMGS(c, sy, class),
					"Religion":         newReligion7GS(c, sy, class),
					"Computer":         newComputer6to12GS(c, sy, class),
					"Speech and Drama": newSpeechGS(c, sy, class),

					"Economics":        newGGS(c, sy, class),
					"Accounts":         newGGS(c, sy, class),
					"Business Studies": newGGS(c, sy, class),
				}
			}
		} else {
			// class is numeric
			gsMap = map[string]gradingSystem{
				"Arabic":           newGGS(c, sy, class),
				"English":          newEGS(c, sy, class),
				"Math":             newMGS(c, sy, class),
				"Citizenship":      newCitizenshipGS(c, sy, class),
				"Speech and Drama": newSpeechGS(c, sy, class),
			}
			if intClass <= 8 {
				gsMap["Social Studies"] = newGGS(c, sy, class)
				gsMap["Social Studies (Arabic)"] = newCitizenshipGS(c, sy, class)
				gsMap["Science"] = newSCGS(c, sy, class)
			}
			if intClass <= 6 {
				gsMap["UCMAS"] = ucmasGradingSystem{}
			}
			if intClass <= 5 {
				gsMap["Religion"] = newGGS(c, sy, class)
				if intClass >= 2 {
					gsMap["Computer"] = newComputer2to5GS(c, sy, class)
				}
			} else {
				gsMap["Religion"] = newReligion7GS(c, sy, class)
				gsMap["Computer"] = newComputer6to12GS(c, sy, class)
			}
		}

		trimmed := strings.TrimSuffix(class, "sci")
		trimmed = strings.TrimSuffix(trimmed, "com")
		trimmedIntClass, err := strconv.Atoi(trimmed)
		if err == nil {
			if trimmedIntClass == 9 {
				gsMap["Citizenship"] = newCitizenshipGS(c, sy, class)
				gsMap["Social Studies (Arabic)"] = newCitizenshipGS(c, sy, class)
			} else if trimmedIntClass == 10 {
				gsMap["Citizenship"] = newCitizenshipGS(c, sy, class)
				gsMap["Social Studies"] = newCitizenshipGS(c, sy, class)
			} else if trimmedIntClass == 11 {
				gsMap["Social Studies"] = newCitizenshipGS(c, sy, class)
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

func newGGS(c context.Context, sy, class string) gradingSystem {
	qWeight, sWeight := classWeights(c, sy, class)
	return Subject{
		"Arabic",
		"Arabic",
		true,

		[]gradingColumn{
			{directGrading, "Homework", 5, 0, 0},
			{directGrading, "Participation", 5, 0, 0},
			{directGrading, "Daily Work", 20, 0, 0},
			{quizGrading, "Quizzes", 10, 6, 5},
			{directGrading, "Quarter Exam", 20, 0, 0},
		},

		[]gradingColumn{
			{directGrading, "Semester Exam", 100, 0, 0},
		},

		qWeight,
		sWeight,
	}
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
		q2 := term.N * 2
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
		return math.NaN()
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (ggs genericGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.IsNaN(m[len(m)-1])
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

func newMGS(c context.Context, sy, class string) gradingSystem {
	q, s := classWeights(c, sy, class)
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
		q2 := term.N * 2
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
		return math.NaN()
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (mgs mathGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.IsNaN(m[len(m)-1])
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

func newEGS(c context.Context, sy, class string) gradingSystem {
	q, s := classWeights(c, sy, class)
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
		q2 := term.N * 2
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
		return math.NaN()
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (egs englishGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.IsNaN(m[len(m)-1])
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

func newSCGS(c context.Context, sy, class string) gradingSystem {
	q, s := classWeights(c, sy, class)
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
		q2 := term.N * 2
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
		return math.NaN()
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (scgs scienceGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.IsNaN(m[len(m)-1])
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

	if term.Typ == Quarter {
		// no calculations
	} else if term.Typ == Semester {
		// Semester Mark
		q2 := term.N * 2
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
		return math.NaN()
	} else if term.Typ == EndOfYear {
		return math.NaN()
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (pgs peGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.IsNaN(m[len(m)-1])
}

func (pgs peGradingSystem) quarterWeight() float64 {
	return 50.0
}

func (pgs peGradingSystem) semesterWeight() float64 {
	return math.NaN()
}

// simpleGradingSystem contains a number of columns that are simply added
// varies from subject to subject
type simpleGradingSystem struct {
	columns []colDescription
	qWeight float64
	sWeight float64
}

func newSingleGS(c context.Context, sy, column, class string) gradingSystem {
	q, s := classWeights(c, sy, class)
	return simpleGradingSystem{
		[]colDescription{{column, 100, true}},
		q,
		s,
	}
}

func newCitizenshipGS(c context.Context, sy, class string) gradingSystem {
	q, s := classWeights(c, sy, class)
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

func newReligion7GS(c context.Context, sy, class string) gradingSystem {
	q, s := classWeights(c, sy, class)
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

func newSpeechGS(c context.Context, sy, class string) gradingSystem {
	q, s := classWeights(c, sy, class)
	trimmed := strings.TrimSuffix(class, "sci")
	trimmed = strings.TrimSuffix(trimmed, "com")
	intClass, err := strconv.Atoi(trimmed)
	if err != nil {
		panic("Class " + class + " does not have Speech and Drama. " + err.Error())
	}

	if intClass <= 5 {
		// 1-5
		return simpleGradingSystem{
			[]colDescription{
				{"Performance", 50, true},
				{"Sight Reading", 30, true},
				{"Discussion by Portfolio", 20, true},
			},
			q,
			s,
		}
	} else {
		// 6-12
		return simpleGradingSystem{
			[]colDescription{
				{"Preparation", 25, true},
				{"Context", 25, true},
				{"Delivery", 25, true},
				{"Attendance / Homework", 25, true},
			},
			q,
			s,
		}
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

	if term.Typ == Quarter {
		sum := sumMarks(m[0 : len(m)-2]...)
		m[len(m)-2] = sum
		m[len(m)-1] = sum * sgs.qWeight / 100.0
	} else if term.Typ == Semester {
		// Semester Exam %
		m[1] = m[0] * sgs.sWeight / 100.0

		// Semester Mark
		q2 := term.N * 2
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
		return math.NaN()
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (sgs simpleGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.IsNaN(m[len(m)-1])
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

func newComputer2to5GS(c context.Context, sy, class string) gradingSystem {
	q, s := classWeights(c, sy, class)
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
		q2 := term.N * 2
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
		return math.NaN()
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (c2gs computer2to5GradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.IsNaN(m[len(m)-1])
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

func newComputer6to12GS(c context.Context, sy, class string) gradingSystem {
	q, s := classWeights(c, sy, class)
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
		q2 := term.N * 2
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
		return math.NaN()
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (c6gs computer6to12GradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	return !math.IsNaN(m[len(m)-1])
}

func (c6gs computer6to12GradingSystem) quarterWeight() float64 {
	return c6gs.qWeight
}

func (c6gs computer6to12GradingSystem) semesterWeight() float64 {
	return c6gs.sWeight
}

// ucmasGradingSystem is used for UCMAS
// No quarters
// Semesters: [Speed Writing: 5], [Flash Cards: 5],
// [Using Abacus: 10], [Magic Bar: 10], [Oral: 10],
// [Mental: 10], [Classwork: 10], [H.W.: 10], [Summative Test: 30]
type ucmasGradingSystem struct {
}

func (ugs ucmasGradingSystem) description(term Term) []colDescription {
	if term.Typ == Quarter {
		return []colDescription{}
	} else if term.Typ == Semester {
		return []colDescription{
			{"Speed Writing", 5, true},
			{"Flash Cards", 5, true},
			{"Using Abacus", 10, true},
			{"Magic Bar", 10, true},
			{"Oral", 10, true},
			{"Mental", 10, true},
			{"Classwork", 10, true},
			{"H.W.", 10, true},
			{"Summative Test", 30, true},
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

func (ugs ucmasGradingSystem) evaluate(term Term, marks studentMarks) (err error) {
	m := marks[term]
	desc := ugs.description(term)

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

	if term.Typ == Quarter {
		// no calculations
	} else if term.Typ == Semester {
		// Semester Mark
		m[9] = sumMarks(m[0:9]...)
	} else if term.Typ == EndOfYear {
		ugs.evaluate(Term{Semester, 1}, marks)
		ugs.evaluate(Term{Semester, 2}, marks)
		m[0] = ugs.get100(Term{Semester, 1}, marks) / 2.0
		m[1] = ugs.get100(Term{Semester, 2}, marks) / 2.0

		m[2] = sumMarks(m[0], m[1])
	} else {
		panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
	}

	marks[term] = m
	return
}

func (ugs ucmasGradingSystem) get100(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return math.NaN()
	} else if term.Typ == Semester {
		return m[9]
	} else if term.Typ == EndOfYear {
		return m[2]
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (ugs ucmasGradingSystem) getExam(term Term, marks studentMarks) float64 {
	m := marks[term]
	if term.Typ == Quarter {
		return math.NaN()
	} else if term.Typ == Semester {
		return m[9]
	} else if term.Typ == EndOfYear {
		return math.NaN()
	}
	panic(fmt.Sprintf("Invalid term type: %d", term.Typ))
}

func (ugs ucmasGradingSystem) ready(term Term, marks studentMarks) bool {
	m := marks[term]
	if term.Typ == Quarter {
		return true
	} else if term.Typ == Semester {
		return !math.IsNaN(m[len(m)-1])
	} else if term.Typ == EndOfYear {
		return !math.IsNaN(m[len(m)-1])
	}
	return false
}

func (ugs ucmasGradingSystem) quarterWeight() float64 {
	return 50.0
}

func (ugs ucmasGradingSystem) semesterWeight() float64 {
	return math.NaN()
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

// subjectInAverage returns true if subject should be calculated in average
func subjectInAverage(subject, class string) bool {
	if (class == "KG1" || class == "KG2" || class == "PreKG") && subject == "Religion" {
		return false
	}

	if subject == "Computer" {
		intClass, err := strconv.Atoi(class)
		if err == nil && intClass >= 9 {
			return true
		} else if strings.HasSuffix(class, "sci") ||
			strings.HasSuffix(class, "com") {
			return true
		}
	}

	if subject == "Social Studies" {
		trimmed := strings.TrimSuffix(class, "sci")
		trimmed = strings.TrimSuffix(trimmed, "com")
		trimmedIntClass, err := strconv.Atoi(trimmed)
		if err == nil && trimmedIntClass >= 10 {
			return false
		}
	}

	subjectsInAverage := map[string]bool{
		"Arabic":           true,
		"English":          true,
		"Math":             true,
		"Science":          true,
		"Biology":          true,
		"Chemistry":        true,
		"Physics":          true,
		"Economics":        true,
		"Accounts":         true,
		"Business Studies": true,
		"Social Studies":   true,
		"Religion":         true,
	}

	return subjectsInAverage[subject]
}

func displayName(subject, class string, term Term) string {
	trimmed := strings.TrimSuffix(class, "sci")
	trimmed = strings.TrimSuffix(trimmed, "com")
	trimmedIntClass, err := strconv.Atoi(trimmed)
	if err != nil {
		return subject
	}

	var semNo = semesterNumber(term)

	if trimmedIntClass == 10 {
		if subject == "Arabic" {
			if semNo == 1 {
				return "Arab 101"
			} else if semNo == 2 {
				return "Arab 102"
			} else {
				return subject
			}
		}
		if subject == "Religion" {
			if semNo == 1 {
				return "Islamic Studies 101"
			} else if semNo == 2 {
				return "Islamic Studies 301"
			} else {
				return subject
			}
		}
		if subject == "Citizenship" {
			if semNo == 1 {
				return "Citizenship 101"
			} else if semNo == 2 {
				return ""
			} else {
				return subject
			}
		}
		if subject == "Social Studies" {
			if semNo == 1 {
				return ""
			} else if semNo == 2 {
				return "Economic Geography 102"
			} else {
				return subject
			}
		}
		if subject == "P.E." {
			return "PE 101 (Physical Preparation)"
		}
		return subject
	} else if trimmedIntClass == 11 {
		if subject == "Arabic" {
			if semNo == 1 {
				return "Arab 201"
			} else if semNo == 2 {
				return "Arab 202"
			} else {
				return subject
			}
		}
		if subject == "Religion" {
			if semNo == 1 {
				return "Islamic Studies 201"
			} else if semNo == 2 {
				return "Islamic Studies 103"
			} else {
				return subject
			}
		}
		if subject == "Social Studies" {
			if semNo == 1 {
				return "Bahrain History and the Gulf 201"
			} else if semNo == 2 {
				return ""
			} else {
				return subject
			}
		}
		if subject == "P.E." {
			return "PE 201 (Basketball)"
		}
		return subject
	} else if trimmedIntClass == 12 {
		if subject == "Arabic" {
			if semNo == 1 {
				return "Arabic 301"
			} else if semNo == 2 {
				return "Arabic 302"
			} else {
				return subject
			}
		}
		if subject == "Religion" {
			if semNo == 1 {
				return "Islamic Studies 104"
			} else if semNo == 2 {
				return ""
			} else {
				return subject
			}
		}
		if subject == "Social Studies" {
			if semNo == 1 {
				return "History of the World of Modern and Contemporary 103"
			} else if semNo == 2 {
				return ""
			} else {
				return subject
			}
		}
		if subject == "P.E." {
			return "PE 301 (Athletics)"
		}
		return subject
	}

	return subject
}

func semesterNumber(term Term) int {
	if term.Typ == Quarter {
		return (term.N-1)/2 + 1
	}

	if term.Typ == Semester {
		return term.N
	}

	return 0
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
