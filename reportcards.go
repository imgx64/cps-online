// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"

	"fmt"
	htmltemplate "html/template"
	"math"
	"net/http"
	"path/filepath"
)

type reportcard struct {
	SY   string
	Term Term

	Name  string
	Class string

	Cols      []string
	Academics []reportcardsRow
	Other     []reportcardsRow
	Total     reportcardsRow

	Remark string

	Behavior     []float64
	BehaviorDesc []colDescription

	LetterDesc   string
	CalculateAll bool
}

type reportcardsRow struct {
	Name   string
	Marks  []float64
	Letter string
}

func init() {
	http.HandleFunc("/reportcards", accessHandler(reportcardsHandler))
	http.HandleFunc("/reportcards/print", accessHandler(reportcardsPrintHandler))
}

func reportcardsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	classGroups := getClassGroups(c)

	data := struct {
		Terms []Term
		CG    []classGroup
	}{
		Terms: terms,
		CG:    classGroups,
	}

	if err := render(w, r, "reportcards", data); err != nil {
		c.Errorf("Could not render template reportcards: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func reportcardsPrintHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	term, err := parseTerm(r.Form.Get("Term"))
	if err != nil {
		c.Errorf("Invalid term: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	// TODO: Check if published

	classSection := r.Form.Get("ClassSection")
	calculateAll := r.Form.Get("CalculateAll") != ""

	var reportcards []reportcard

	students, err := getStudents(c, true, classSection)
	if err != nil {
		c.Errorf("Could not retrieve students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	sy := getSchoolYear(c)

	for _, stu := range students {
		rc := reportcard{
			SY:   sy,
			Term: term,

			Name:  stu.Name,
			Class: stu.Class + stu.Section,
		}

		ls := getLetterSystem(stu.Class)

		if term.Typ == Quarter {
			rc.Cols = []string{"Max Mark", "Mark Obtained"}
		} else if term.Typ == Semester {
			q2 := term.N * 2
			q1 := q2 - 1
			gs := getGradingSystem(c, stu.Class, "English") //TODO: find a better way
			rc.Cols = []string{
				fmt.Sprintf("Quarter %d (%.0f%%)", q1, gs.quarterWeight()),
				fmt.Sprintf("Quarter %d (%.0f%%)", q2, gs.quarterWeight()),
				fmt.Sprintf("Semester Exam (%.0f%%)", gs.semesterWeight()),
				"Mark Obtained (100%)",
			}
		} else if term.Typ == EndOfYear {
			rc.Cols = []string{
				"Semester 1 (50%)",
				"Semester 2 (50%)",
				"Mark Obtained (100%)",
			}
		} else {
			c.Errorf("Invalid term type: %d", term.Typ)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		total := negZero
		totalMax := negZero
		numInAverage := 0
		for _, subject := range subjects {
			if subject == "Remarks" {
				continue
			}

			subjectDisplayName := displayName(subject, stu.Class, term)
			if subjectDisplayName == "" {
				continue
			}

			gs := getGradingSystem(c, stu.Class, subject)
			if gs == nil {
				continue
			}
			marks, err := getStudentMarks(c, stu.ID, subject)
			if err != nil {
				c.Errorf("Could not get marks: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			gs.evaluate(term, marks)
			if subject == "Behavior" {
				rc.Behavior = marks[term]
				continue
			}

			mark := gs.get100(term, marks)
			letter := ls.getLetter(mark)
			rcRow := reportcardsRow{Name: subjectDisplayName, Letter: letter}

			if term.Typ == Quarter {
				rcRow.Marks = []float64{100, mark}
			} else if term.Typ == Semester {
				q2 := term.N * 2
				q1 := q2 - 1
				rcRow.Marks = []float64{
					gs.get100(Term{Quarter, q1}, marks) * gs.quarterWeight() / 100.0,
					gs.get100(Term{Quarter, q2}, marks) * gs.quarterWeight() / 100.0,
					gs.getExam(term, marks),
					gs.get100(term, marks),
				}
			} else if term.Typ == EndOfYear {
				rcRow.Marks = []float64{
					gs.get100(Term{Semester, 1}, marks) * 50.0 / 100.0,
					gs.get100(Term{Semester, 2}, marks) * 50.0 / 100.0,
					gs.get100(term, marks),
				}
			}
			if subjectInAverage(subject, stu.Class) {
				if !math.Signbit(mark) {
					total += mark
					totalMax += 100
					numInAverage++
				}
				rc.Academics = append(rc.Academics, rcRow)
			} else {
				if calculateAll && !math.Signbit(mark) {
					total += mark
					totalMax += 100
					numInAverage++
				}
				rc.Other = append(rc.Other, rcRow)
			}
		}
		average := total / float64(numInAverage)
		totalRow := reportcardsRow{
			Name:   "Total",
			Letter: formatMark(average) + "%",
		}
		if term.Typ == Quarter {
			totalRow.Marks = []float64{totalMax, total}
		} else if term.Typ == Semester {
			totalRow.Marks = []float64{negZero, negZero, negZero, total}
		} else if term.Typ == EndOfYear {
			totalRow.Marks = []float64{negZero, negZero, total}
		}
		rc.Total = totalRow

		remark, err := getStudentRemark(c, stu.ID, term)
		if err != nil {
			c.Errorf("Could not get remark: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		rc.Remark = remark

		rc.BehaviorDesc = behaviorDesc
		rc.LetterDesc = ls.String()
		rc.CalculateAll = calculateAll

		reportcards = append(reportcards, rc)
	}

	data := reportcards

	// Note: not using render() because we don't want the base template
	templateFile := filepath.Join("template", "reportcardsprint.html")
	tmpl, err := htmltemplate.New("reportcardsprint.html").Funcs(funcMap).
		ParseFiles(templateFile)
	if err != nil {
		c.Errorf("Could not parse template reportcardsprint: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		c.Errorf("Could not execute template reportcardsprint: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

}
