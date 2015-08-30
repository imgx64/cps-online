// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"bytes"
	"fmt"
	"math"
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
	// greater than for reverse sort
	if pars[i].SortBy > pars[j].SortBy {
		return true
	}
	if pars[i].SortBy < pars[j].SortBy {
		return false
	}

	return bytes.Compare([]byte(pars[i].Name), []byte(pars[j].Name)) < 0
}

func (pars printAllRowSorter) Swap(i, j int) {
	pars[i], pars[j] = pars[j], pars[i]
}

func printAllHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classSection := r.Form.Get("ClassSection")
	if classSection == "" {
		classSection = "all"
	}

	term, err := parseTerm(r.Form.Get("Term"))
	if err != nil {
		term = Term{}
	}

	subject := r.Form.Get("Subject")
	doSort := r.Form.Get("Sort") != ""

	var cols []colDescription
	studentRows := make(map[string][]printAllRow)

	students, err := getStudents(c, classSection)
	if err != nil {
		log.Errorf(c, "Could not get students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	prevClass := ""
	maxLen := 0
	var maxCols []colDescription
	if subject == "All" {
		for _, sub := range subjects {
			if sub == "Remarks" || sub == "Behavior" {
				continue
			}
			maxCols = append(maxCols, colDescription{sub, 100, false})
		}
		maxCols = append(maxCols, colDescription{"Average", 100, false})
		for _, stu := range students {
			total := negZero
			totalMax := negZero
			numInAverage := 0
			var studentMarks []float64
			for _, subject := range subjects {
				if subject == "Remarks" || subject == "Behavior" {
					continue
				}
				gs := getGradingSystem(c, sy, stu.Class, subject)
				if gs == nil {
					// class doesn't have subject
					studentMarks = append(studentMarks, negZero)
					continue
				}
				marks, err := getStudentMarks(c, stu.ID, sy, subject)
				if err != nil {
					log.Errorf(c, "Could not get marks: %s", err)
					renderError(w, r, http.StatusInternalServerError)
					return
				}
				gs.evaluate(term, marks)

				mark := gs.get100(term, marks)

				if subjectInAverage(subject, stu.Class) {
					if !math.Signbit(mark) {
						total += mark
						totalMax += 100
						numInAverage++
					}
				}
				studentMarks = append(studentMarks, mark)
			}
			average := negZero
			if numInAverage > 0 {
				average = total / float64(numInAverage)
			}
			studentMarks = append(studentMarks, average)

			//TODO: Sort by marks other than average
			row := printAllRow{stu.Class + stu.Section, stu.Name, studentMarks, average}
			classSection := fmt.Sprintf("%s|%s", stu.Class, stu.Section)
			studentRows[classSection] = append(studentRows[classSection], row)
		}
	} else {
		for _, stu := range students {
			if gs := getGradingSystem(c, sy, stu.Class, subject); gs != nil {
				if stu.Class != prevClass {
					cols = gs.description(term)
					if len(cols) > maxLen {
						maxLen = len(cols)
						maxCols = cols
					}
				}
				m, err := getStudentMarks(c, stu.ID, sy, subject)
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
	}

	for len(maxCols) < 13 {
		maxCols = append(maxCols, colDescription{"____", 0, false})
	}

	var allRows []printAllRow
	classGroups := getClassGroups(c, sy)
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

	subjectsData := []string{}
	for _, sub := range subjects {
		if sub == "Remarks" {
			continue
		}
		subjectsData = append(subjectsData, sub)
	}
	subjectsData = append(subjectsData, "All")

	data := struct {
		ClassSection string
		Term         Term
		Subject      string
		Sort         bool

		Terms    []Term
		Subjects []string
		CG       []classGroup

		Cols     []colDescription
		Students []printAllRow
	}{
		classSection,
		term,
		subject,
		doSort,

		terms,
		subjectsData,
		classGroups,

		maxCols,
		allRows,
	}

	if err := render(w, r, "printallmarks", data); err != nil {
		log.Errorf(c, "Could not render template marks: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
