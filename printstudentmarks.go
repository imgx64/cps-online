// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

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

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	studentId := r.Form.Get("id")
	stu, err := getStudent(c, studentId)
	if err != nil {
		log.Errorf(c, "Could not get student %s: %s", studentId, err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	class, section, err := getStudentClass(c, stu.ID, sy)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
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
		total := math.NaN()
		numInAverage := 0
		ls := getLetterSystem(c, sy, class)
		for _, subject := range subjects {
			if subject == "Remarks" {
				continue
			}
			gs := getGradingSystem(c, sy, class, subject)
			if gs == nil {
				continue
			}
			// TODO: don't loop over terms outside
			marks, err := getStudentMarks(c, stu.ID, sy, subject)
			if err != nil {
				log.Errorf(c, "Could not get marks: %s", err)
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
			if subjectInAverage(subject, class) && !math.IsNaN(mark) {
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

		remark, err = getStudentRemark(c, stu.ID, sy, term)
		if err != nil {
			log.Errorf(c, "Could not get remark: %s", err)
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
		class,
		section,

		marksTerms,
	}

	if err := render(w, r, "printstudentmarks", data); err != nil {
		log.Errorf(c, "Could not render template printstudentmarks: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
