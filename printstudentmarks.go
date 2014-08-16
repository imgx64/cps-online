// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	"math"

	"net/http"
)

type studentMarksTerm struct {
	Term        Term
	SubjectRows []studentMarksRow
	Behavior    []float64
	Remark      string
}

type studentMarksRow struct {
	Subject string

	DetailsCols  []colDescription
	DetailsMarks []float64

	Letter string
}

func init() {
	http.HandleFunc("/printstudentmarks", accessHandler(printStudentMarksHandler))
}

func printStudentMarksHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	studentId := r.Form.Get("id")
	stu, err := getStudent(c, studentId)
	if err != nil {
		c.Errorf("Could not get student %s: %s", studentId, err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var marksTerms []studentMarksTerm
	for _, term := range terms {

		var studentMarksRows []studentMarksRow
		var subjectsCols []colDescription
		var subjectsMarks []float64
		var behavior []float64
		var remark string
		total := negZero
		numInAverage := 0
		ls := getLetterSystem(stu.Class)
		for _, subject := range subjects {
			if subject == "Remarks" {
				continue
			}
			gs := getGradingSystem(c, stu.Class, subject)
			if gs == nil {
				continue
			}
			// TODO: don't loop over terms outside
			marks, err := getStudentMarks(c, stu.ID, subject)
			if err != nil {
				c.Errorf("Could not get marks: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			gs.evaluate(term, marks)
			if subject == "Behavior" {
				behavior = marks[term]
				continue
			}

			mark := gs.get100(term, marks)
			letter := ls.getLetter(mark)
			if subjectInAverage(subject, stu.Class) && !math.Signbit(mark) {
				total += mark
				numInAverage++
			}

			subjectsCols = append(subjectsCols, colDescription{subject, 100, false})
			subjectsMarks = append(subjectsMarks, mark)

			studentMarksRows = append(studentMarksRows, studentMarksRow{
				subject, gs.description(term), marks[term], letter})
		}

		average := total / float64(numInAverage)
		subjectsCols = append(subjectsCols, colDescription{"Average", 100, false})
		subjectsMarks = append(subjectsMarks, average)
		studentMarksRows = append(studentMarksRows, studentMarksRow{
			"All", subjectsCols, subjectsMarks, ""})

		remark, err = getStudentRemark(c, stu.ID, term)
		if err != nil {
			c.Errorf("Could not get remark: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		marksTerms = append(marksTerms, studentMarksTerm{
			term, studentMarksRows, behavior, remark})
	}

	data := struct {
		BehaviorDesc []colDescription

		Name    string
		Class   string
		Section string

		MarksTerms []studentMarksTerm
	}{
		behaviorDesc,

		stu.Name,
		stu.Class,
		stu.Section,

		marksTerms,
	}

	if err := render(w, r, "printstudentmarks", data); err != nil {
		c.Errorf("Could not render template printstudentmarks: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
