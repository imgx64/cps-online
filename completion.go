// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	"appengine/datastore"

	"fmt"
	"net/http"
)

// completion will be stored in the datastore
type completion struct {
	ClassSection string
	Term         string
	Subject      string
	N            int
}

func getCompletions(c appengine.Context, term Term, classSection string) ([]completion, error) {
	q := datastore.NewQuery("completion").
		Filter("Term =", term.Value()).
		Filter("ClassSection =", classSection)
	var completions []completion
	_, err := q.GetAll(c, &completions)
	if err != nil {
		return nil, err
	}

	return completions, nil
}

func storeCompletion(c appengine.Context, classSection string, term Term,
	subject string, nComplete int) error {

	cr := completion{classSection, term.Value(), subject, nComplete}
	keyStr := fmt.Sprintf("%s|%s|%s", classSection, term, subject)
	key := datastore.NewKey(c, "completion", keyStr, 0, nil)
	_, err := datastore.Put(c, key, &cr)
	if err != nil {
		return err
	}

	return nil
}

type completionRow struct {
	ClassSection string
	NumStudents  int
	Completion   map[string]int
}

func init() {
	http.HandleFunc("/completion", accessHandler(completionHandler))
}

func completionHandler(w http.ResponseWriter, r *http.Request) {
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

	var completionRows []completionRow

	classes := getClasses(c)
	sections := getClassSections(c)

	if (term != Term{}) {
		for _, class := range classes {
			for _, section := range sections[class] {
				var cr completionRow

				cr.ClassSection = class + section

				classSection := fmt.Sprintf("%s|%s", class, section)

				numStudents, err := getStudentsCount(c, true, classSection)
				if err != nil {
					c.Errorf("Could not retrieve number of students: %s", err)
					renderError(w, r, http.StatusInternalServerError)
					return
				}
				cr.NumStudents = numStudents

				cr.Completion = make(map[string]int)
				for _, subject := range subjects {
					if getGradingSystem(c, class, subject) == nil {
						// class doesn't have subject
						cr.Completion[subject] = -1
					}
				}
				cr.Completion["Remarks"] = 0
				completions, err := getCompletions(c, term, classSection)
				if err != nil {
					c.Errorf("Could not retrieve completions: %s", err)
					renderError(w, r, http.StatusInternalServerError)
					return
				}
				for _, comp := range completions {
					cr.Completion[comp.Subject] = comp.N
				}

				completionRows = append(completionRows, cr)
			}
		}
	}

	data := struct {
		Terms []Term
		Term  Term

		Subjects       []string
		CompletionRows []completionRow
	}{
		terms,
		term,

		subjects,
		completionRows,
	}

	if err := render(w, r, "completion", data); err != nil {
		c.Errorf("Could not render template completion: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
