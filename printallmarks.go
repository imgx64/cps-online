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
	var err error
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

	calculateAll := true

	subject := r.Form.Get("Subject")
	doSort := r.Form.Get("Sort") != ""

	var cols []colDescription
	studentRows := make(map[string][]printAllRow)

	students, err := findStudents(c, sy, classSection)
	if err != nil {
		log.Errorf(c, "Could not get students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	allSubjects := getAllSubjects(c, sy)

	maxLen := 0
	var maxCols []colDescription
	if subject == "All" {
		var subjects []string
		if classSection == "all" {
			subjects = allSubjects
		} else {
			var class string
			if classSection != "all" {
				class, _, err = parseClassSection(classSection)
				if err != nil {
					log.Errorf(c, "Could not parse class %s: %s", classSection, err)
					renderError(w, r, http.StatusInternalServerError)
					return
				}
			}

			subjects, err = getSubjects(c, sy, class)
			if err != nil {
				log.Errorf(c, "Could not subjects of class %s: %s", class, err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
		}
		for _, sub := range subjects {
			if sub == "Remarks" || sub == "Behavior" || sub == "Attendance" {
				continue
			}
			maxCols = append(maxCols, colDescription{sub, 100, math.NaN(), false})
		}
		maxCols = append(maxCols, colDescription{"Average", 100, math.NaN(), false})
		for _, stu := range students {
			total := math.NaN()
			numInAverage := 0
			var studentMarksArr []float64
			for _, subject := range subjects {
				if subject == "Remarks" || subject == "Behavior" || subject == "Attendance" {
					continue
				}
				gs := getGradingSystem(c, sy, stu.Class, subject)
				if gs == nil {
					// class doesn't have subject
					studentMarksArr = append(studentMarksArr, math.NaN())
					continue
				}
				marks, err := getStudentTermMarks(c, stu.ID, sy, subject, term, gs)
				if err != nil {
					log.Errorf(c, "Could not get marks: %s", err)
					renderError(w, r, http.StatusInternalServerError)
					return
				}
				m := make(studentMarks)
				m[term] = marks

				mark := gs.get100(term, m)

				if (calculateAll || gs.subjectInAverage()) && !math.IsNaN(mark) {
					if math.IsNaN(total) {
						total = 0
					}
					total += mark
					numInAverage++
				}
				studentMarksArr = append(studentMarksArr, mark)
			}
			average := math.NaN()
			if numInAverage > 0 {
				average = total / float64(numInAverage)
			}
			studentMarksArr = append(studentMarksArr, average)

			//TODO: Sort by marks other than average
			row := printAllRow{stu.Class + stu.Section, stu.Name, studentMarksArr, average}
			classSection := fmt.Sprintf("%s|%s", stu.Class, stu.Section)
			studentRows[classSection] = append(studentRows[classSection], row)
		}
	} else {
		prevClass := ""
		for _, stu := range students {
			if gs := getGradingSystem(c, sy, stu.Class, subject); gs != nil {
				if !gs.inStream(stu.Stream) {
					continue
				}
				if stu.Class != prevClass {
					cols = gs.description(c, sy, term)
					if len(cols) > maxLen {
						maxLen = len(cols)
						maxCols = cols
					}
				}
				marks, err := getStudentTermMarks(c, stu.ID, sy, subject, term, gs)
				if err != nil {
					// TODO: report error
					continue
				}
				m := make(studentMarks)
				m[term] = marks

				classSection := fmt.Sprintf("%s|%s", stu.Class, stu.Section)
				row := printAllRow{stu.Class + stu.Section, stu.Name, m[term], gs.get100(term, m)}
				studentRows[classSection] = append(studentRows[classSection], row)
			}
		}
	}

	for len(maxCols) < 13 {
		maxCols = append(maxCols, colDescription{"____", math.NaN(), math.NaN(), false})
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
	for _, sub := range allSubjects {
		if sub == "Remarks" {
			continue
		}
		subjectsData = append(subjectsData, sub)
	}
	subjectsData = append(subjectsData, "Behavior", "Attendance", "All")

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
