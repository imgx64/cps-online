// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/qedus/nds"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

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

func getCompletions(c context.Context, term Term, classSection string) ([]completion, error) {
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

func storeCompletion(c context.Context, classSection string, term Term,
	subject string, nComplete int) error {

	cr := completion{classSection, term.Value(), subject, nComplete}
	keyStr := fmt.Sprintf("%s|%s|%s", classSection, term, subject)
	key := datastore.NewKey(c, "completion", keyStr, 0, nil)
	_, err := nds.Put(c, key, &cr)
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

	var completionRows []completionRow

	classes := getClasses(c, sy)
	sections := getClassSections(c, sy)

	if (term != Term{}) {
		for _, class := range classes {
			subjects, err := getSubjects(c, sy, class)
			if err != nil {
				log.Errorf(c, "Could not get subjects: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			for _, section := range sections[class] {
				var cr completionRow

				cr.ClassSection = class + section

				classSection := fmt.Sprintf("%s|%s", class, section)

				numStudents, err := findStudentsCount(c, sy, classSection)
				if err != nil {
					log.Errorf(c, "Could not retrieve number of students: %s", err)
					renderError(w, r, http.StatusInternalServerError)
					return
				}
				cr.NumStudents = numStudents

				cr.Completion = make(map[string]int)
				for _, subject := range subjects {
					if getGradingSystem(c, sy, class, subject) == nil {
						// class doesn't have subject
						cr.Completion[subject] = -1
					}
				}
				cr.Completion["Remarks"] = 0
				completions, err := getCompletions(c, term, classSection)
				if err != nil {
					log.Errorf(c, "Could not retrieve completions: %s", err)
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

	allSubjects, err := getAllSubjects(c, sy)
	if err != nil {
		log.Errorf(c, "Could not get subjects: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		Terms []Term
		Term  Term

		Subjects       []string
		CompletionRows []completionRow
	}{
		terms,
		term,

		allSubjects,
		completionRows,
	}

	if err := render(w, r, "completion", data); err != nil {
		log.Errorf(c, "Could not render template completion: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
