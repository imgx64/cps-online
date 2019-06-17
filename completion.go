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
	NumStudents  map[string]int
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

				numStudentsStream := make(map[string]int)
				cr.NumStudents = make(map[string]int)
				cr.Completion = make(map[string]int)
				for _, subject := range subjects {
					gs := getGradingSystem(c, sy, class, subject)
					if gs == nil {
						// class doesn't have subject
						cr.Completion[subject] = -1
					} else if subjectSubject, ok := gs.(Subject); ok {
						stream := subjectSubject.Stream
						numStudentsStream[stream], ok = numStudentsStream[stream]
						if !ok {
							numStudentsStream[stream], err = findStudentsCount(c, sy, classSection, stream)
							if err != nil {
								log.Errorf(c, "Could not retrieve number of students: %s", err)
								renderError(w, r, http.StatusInternalServerError)
								return
							}
						}
						cr.NumStudents[subject] = numStudentsStream[stream]
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

	var weekS1Terms []Term
	var weekS2Terms []Term
	maxWeeks := getMaxWeeks(c)
	for i := 1; i <= maxWeeks; i++ {
		weekS1Terms = append(weekS1Terms, Term{WeekS1, i})
		weekS2Terms = append(weekS2Terms, Term{WeekS2, i})
	}

	allSubjects := getAllSubjects(c, sy)

	data := struct {
		Terms       []Term
		WeekS1Terms []Term
		WeekS2Terms []Term
		Term        Term

		Subjects       []string
		CompletionRows []completionRow
	}{
		terms,
		weekS1Terms,
		weekS2Terms,
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
