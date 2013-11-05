// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"

	"net/http"
)

func init() {
	http.HandleFunc("/printallmarks", accessHandler(printAllHandler))
}

type printAllRow struct {
	ClassSection string
	Name         string
	Marks        []float64
}

func printAllHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	term, err := parseTerm(r.Form.Get("Term"))
	if err != nil {
		term = Term{}
	}

	subject := r.Form.Get("Subject")

	var cols []colDescription
	var studentRows []printAllRow

	students, err := getStudents(c, true, "all")
	if err != nil {
		c.Errorf("Could not get students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	prevClass := ""
	maxLen := 0
	var maxCols []colDescription
	for _, stu := range students {
		if !classHasSubject(stu.Class, subject) {
			continue
		}
		gs := getGradingSystem(stu.Class, subject)
		if stu.Class != prevClass {
			cols = gs.description(term)
			if len(cols) > maxLen {
				maxLen = len(cols)
				maxCols = cols
			}
		}
		m, err := getStudentMarks(c, stu.ID, subject)
		if err != nil {
			// TODO: report error
			continue
		}
		gs.evaluate(term, m) // TODO: check error
		studentRows = append(studentRows, printAllRow{stu.Class + stu.Section, stu.Name, m[term]})
	}

	for len(maxCols) < 13 {
		maxCols = append(maxCols, colDescription{"____", 0, false})
	}

	data := struct {
		Term    Term
		Subject string

		Terms    []Term
		Subjects []string

		Cols     []colDescription
		Students []printAllRow
	}{
		term,
		subject,

		terms,
		subjects,

		maxCols,
		studentRows,
	}

	if err := render(w, r, "printallmarks", data); err != nil {
		c.Errorf("Could not render template marks: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
