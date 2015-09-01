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

type reportcardRow struct {
	Subject   string
	Mark      float64
	Letter    string
	InAverage bool

	DetailsCols  []colDescription
	DetailsMarks []float64
}

func init() {
	http.HandleFunc("/reportcard", accessHandler(reportcardHandler))
}

func reportcardHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	term, err := parseTerm(r.Form.Get("Term"))
	if err != nil {
		term = Term{}
	}

	user, err := getUser(c)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if user.Student == nil {
		log.Errorf(c, "User is not a student: %s", user.Email)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	stu := *user.Student

	class, section, err := getStudentClass(c, stu.ID, sy)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	publish := published(c, term)
	if term == (Term{}) {
		publish = false
	}

	var reportcardRows []reportcardRow
	var average float64
	var behavior []float64
	var remark, letterDesc string
	if publish {
		total := negZero
		numInAverage := 0
		ls := getLetterSystem(class)
		letterDesc = ls.String()
		for _, subject := range subjects {
			if subject == "Remarks" {
				continue
			}
			gs := getGradingSystem(c, sy, class, subject)
			if gs == nil {
				continue
			}
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
			var inAverage bool
			if subjectInAverage(subject, class) && !math.Signbit(mark) {
				total += mark
				numInAverage++
				inAverage = true
			}
			reportcardRows = append(reportcardRows, reportcardRow{
				subject, mark, letter, inAverage, gs.description(term), marks[term]})
		}
		average = total / float64(numInAverage)

		remark, err = getStudentRemark(c, stu.ID, sy, term)
		if err != nil {
			log.Errorf(c, "Could not get remark: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	data := struct {
		Terms     []Term
		Term      Term
		Published bool

		Name    string
		Class   string
		Section string

		SubjectRows  []reportcardRow
		Average      float64
		Behavior     []float64
		BehaviorDesc []colDescription
		Remark       string

		LetterDesc string
	}{
		terms,
		term,
		publish,

		stu.Name,
		class,
		section,

		reportcardRows,
		average,
		behavior,
		behaviorDesc,
		remark,

		letterDesc,
	}

	if err := render(w, r, "reportcard", data); err != nil {
		log.Errorf(c, "Could not render template reportcard: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
