// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"

	"fmt"
	"net/http"
	"sort"
)

func init() {
	http.HandleFunc("/printallmarks", accessHandler(printAllHandler))
}

type printAllRow struct {
	ClassSection string
	Name         string
	Marks        []float64
	SortBy       float64
}

type printAllRowSorter []printAllRow

func (pars printAllRowSorter) Len() int {
	return len(pars)
}

func (pars printAllRowSorter) Less(i, j int) bool {
	return pars[i].SortBy > pars[j].SortBy
}

func (pars printAllRowSorter) Swap(i, j int) {
	pars[i], pars[j] = pars[j], pars[i]
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
	doSort := r.Form.Get("Sort") != ""

	var cols []colDescription
	studentRows := make(map[string][]printAllRow)

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
		if subject == "All" {
			// FIXME: Get all marks + average
		} else if classHasSubject(stu.Class, subject) {
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
			classSection := fmt.Sprintf("%s|%s", stu.Class, stu.Section)
			row := printAllRow{stu.Class + stu.Section, stu.Name, m[term], gs.get100(term, m)}
			studentRows[classSection] = append(studentRows[classSection], row)
		}
	}

	for len(maxCols) < 13 {
		maxCols = append(maxCols, colDescription{"____", 0, false})
	}

	var allRows []printAllRow
	for _, cg := range classGroups {
		for _, sec := range cg.Sections {
			classSection := fmt.Sprintf("%s|%s", cg.Class, sec)
			rows := studentRows[classSection]
			if doSort {
				sort.Sort(printAllRowSorter(rows))
			}
			allRows = append(allRows, rows...)
		}
	}

	data := struct {
		Term    Term
		Subject string
		Sort    bool

		Terms    []Term
		Subjects []string

		Cols     []colDescription
		Students []printAllRow
	}{
		term,
		subject,
		doSort,

		terms,
		subjects,

		maxCols,
		allRows,
	}

	if err := render(w, r, "printallmarks", data); err != nil {
		c.Errorf("Could not render template marks: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
