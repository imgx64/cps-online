// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"fmt"
	htmltemplate "html/template"
	"math"
	"net/http"
	"path/filepath"
)

type reportcard struct {
	SY   string
	Term Term

	Name  string
	Class string

	Cols      []string
	Academics []reportcardsRow
	Other     []reportcardsRow
	Total     reportcardsRow

	Remark string

	Behavior     []float64
	BehaviorDesc []colDescription

	Attendance     []float64
	AttendanceDesc []colDescription

	LetterDesc   string
	CalculateAll bool
}

type eoyGpaReportcard struct {
	Student studentType

	Rows []GPARow

	CreditsEarned float64
	YearAverage   string
	GPA           float64
}

type reportcardsRow struct {
	Name   string
	Marks  []float64
	Letter string
}

func init() {
	http.HandleFunc("/reportcards", accessHandler(reportcardsHandler))
	http.HandleFunc("/reportcards/print", accessHandler(reportcardsPrintHandler))
}

func reportcardsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	classGroups := getClassGroups(c, sy)

	termsWithGpa := append(terms, Term{EndOfYearGpa, 0})

	data := struct {
		Terms []Term
		CG    []classGroup
	}{
		Terms: termsWithGpa,
		CG:    classGroups,
	}

	if err := render(w, r, "reportcards", data); err != nil {
		log.Errorf(c, "Could not render template reportcards: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func reportcardsPrintHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	term, err := parseTerm(r.Form.Get("Term"))
	if err != nil {
		log.Errorf(c, "Invalid term: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	if term.Typ == EndOfYearGpa {
		reportcardsGpaTermHandler(w, r)
		return
	}

	sy := getSchoolYear(c)

	// TODO: Check if published

	classSection := r.Form.Get("ClassSection")
	calculateAll := r.Form.Get("CalculateAll") != ""
	showQuarterCols := r.Form.Get("ShowQuarterColumns") != ""

	var reportcards []reportcard

	students, err := findStudents(c, sy, classSection)
	if err != nil {
		log.Errorf(c, "Could not retrieve students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	class, _, err := parseClassSection(classSection)
	if err != nil {
		log.Errorf(c, "Could not parse classSection %s: %s", classSection, err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	subjects, err := getSubjects(c, sy, class)
	if err != nil {
		log.Errorf(c, "Could not get subjects: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	subjects = append(subjects, "Behavior", "Attendance")

	for _, stu := range students {
		rc := reportcard{
			SY:   sy,
			Term: term,

			Name:  stu.Name,
			Class: stu.Class + stu.Section,
		}

		ls := getLetterSystem(c, sy, stu.Class)

		if term.Typ == Quarter {
			rc.Cols = []string{"Max Mark", "Mark Obtained"}
		} else if term.Typ == Midterm {
			rc.Cols = []string{"Max Mark", "Mark Obtained"}
		} else if term.Typ == Semester {
			if showQuarterCols {
				q2 := term.N * 2
				q1 := q2 - 1

				var qWeight, sWeight float64
				found := false
				for _, classSetting := range getClassSettings(c, sy) {
					if classSetting.Class != class {
						continue
					}

					qWeight = classSetting.QuarterWeight
					if qWeight > 50 {
						qWeight = 50
					}
					if qWeight < 0 {
						qWeight = 0
					}

					sWeight = 100 - qWeight*2

					found = true
					break
				}
				if !found {
					log.Errorf(c, "Could not get class settings: %s %s", sy, class)
					renderError(w, r, http.StatusInternalServerError)
					return
				}
				rc.Cols = []string{
					fmt.Sprintf("Quarter %d (%.0f%%)", q1, qWeight),
					fmt.Sprintf("Quarter %d (%.0f%%)", q2, qWeight),
					fmt.Sprintf("Semester Exam (%.0f%%)", sWeight),
					"Mark Obtained (100%)",
				}
			} else {
				rc.Cols = []string{"Max Mark", "Mark Obtained"}
			}
		} else if term.Typ == EndOfYear {
			rc.Cols = []string{
				"Semester 1",
				"Semester 2",
				"Mark Obtained (100%)",
			}
		} else {
			log.Errorf(c, "Invalid term type: %d", term.Typ)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		total := 0.0
		totalMax := 0.0
		numInAverage := 0
		for _, subject := range subjects {
			if subject == "Remarks" {
				continue
			}

			gs := getGradingSystem(c, sy, stu.Class, subject)
			if gs == nil {
				continue
			}

			subjectDisplayName := gs.displayName()
			if subjectDisplayName == "" {
				continue
			}

			marks, err := getStudentMarks(c, stu.ID, sy, subject)
			if err != nil {
				log.Errorf(c, "Could not get marks: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			gs.evaluate(c, stu.ID, sy, term, marks)
			if subject == "Behavior" {
				rc.Behavior = marks[term]
				continue
			}
			if subject == "Attendance" {
				att := marks[term]
				if term.Typ == Quarter || term.Typ == Midterm {
					rc.Attendance = append(rc.Attendance, att[0]+att[1])
					rc.Attendance = append(rc.Attendance, att[2])
					rc.Attendance = append(rc.Attendance, att[3]+att[4])
					rc.Attendance = append(rc.Attendance, att[5])
				} else if term.Typ == Semester {
					rc.Attendance = append(rc.Attendance, att[4])
					rc.Attendance = append(rc.Attendance, att[8])
					rc.Attendance = append(rc.Attendance, att[13])
					rc.Attendance = append(rc.Attendance, att[17])
				} else if term.Typ == EndOfYear {
					rc.Attendance = append(rc.Attendance, att[2])
					rc.Attendance = append(rc.Attendance, att[5])
					rc.Attendance = append(rc.Attendance, att[8])
					rc.Attendance = append(rc.Attendance, att[11])
				}
				continue
			}

			mark := gs.get100(term, marks)
			letter := ls.getLetter(mark)
			rcRow := reportcardsRow{Name: subjectDisplayName, Letter: letter}

			if term.Typ == Quarter {
				rcRow.Marks = []float64{100, mark}
			} else if term.Typ == Midterm {
				rcRow.Marks = []float64{100, mark}
			} else if term.Typ == Semester {
				if showQuarterCols {
					q2 := term.N * 2
					q1 := q2 - 1
					rcRow.Marks = []float64{
						gs.get100(Term{Quarter, q1}, marks) * gs.quarterWeight() / 100.0,
						gs.get100(Term{Quarter, q2}, marks) * gs.quarterWeight() / 100.0,
						gs.getExam(term, marks),
						gs.get100(term, marks),
					}
				} else {
					rcRow.Marks = []float64{100, mark}
				}
			} else if term.Typ == EndOfYear {
				rcRow.Marks = marks[term]
			}

			if gs.subjectInAverage() {
				if !math.IsNaN(mark) {
					total += mark
					totalMax += 100
					numInAverage++
				}
				rc.Academics = append(rc.Academics, rcRow)
			} else {
				if calculateAll && !math.IsNaN(mark) {
					total += mark
					totalMax += 100
					numInAverage++
				}
				rc.Other = append(rc.Other, rcRow)
			}
		}
		average := math.NaN()
		if numInAverage > 0 {
			average = total / float64(numInAverage)
		} else {
			total = math.NaN()
			totalMax = math.NaN()
		}
		totalRow := reportcardsRow{}
		if !calculateAll {
			totalRow.Name = "Total"
			totalRow.Letter = formatMark(average) + "%"
			if term.Typ == Quarter {
				totalRow.Marks = []float64{totalMax, total}
			} else if term.Typ == Midterm {
				totalRow.Marks = []float64{totalMax, total}
			} else if term.Typ == Semester {
				if showQuarterCols {
					totalRow.Marks = []float64{math.NaN(), math.NaN(), math.NaN(), total}
				} else {
					totalRow.Marks = []float64{totalMax, total}
				}
			} else if term.Typ == EndOfYear {
				totalRow.Marks = []float64{math.NaN(), math.NaN(), total}
			}
		} else {
			totalRow.Name = "General Weighted Average"
			totalRow.Letter = ls.getLetter(average)
			if term.Typ == Quarter {
				totalRow.Marks = []float64{math.NaN(), average}
			} else if term.Typ == Midterm {
				totalRow.Marks = []float64{math.NaN(), average}
			} else if term.Typ == Semester {
				if showQuarterCols {
					totalRow.Marks = []float64{math.NaN(), math.NaN(), math.NaN(), average}
				} else {
					totalRow.Marks = []float64{math.NaN(), average}
				}
			} else if term.Typ == EndOfYear {
				totalRow.Marks = []float64{math.NaN(), math.NaN(), average}
			}
		}

		rc.Total = totalRow

		remark, err := getStudentRemark(c, sy, stu.ID, term)
		if err != nil {
			log.Errorf(c, "Could not get remark: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		rc.Remark = remark

		rc.BehaviorDesc = behaviorDesc
		rc.AttendanceDesc = displayAttendanceDesc
		rc.LetterDesc = ls.String()
		rc.CalculateAll = calculateAll

		reportcards = append(reportcards, rc)
	}

	data := reportcards

	// Note: not using render() because we don't want the base template
	templateFile := filepath.Join("template", "reportcardsprint.html")
	tmpl, err := htmltemplate.New("reportcardsprint.html").Funcs(funcMap).
		ParseFiles(templateFile)
	if err != nil {
		log.Errorf(c, "Could not parse template reportcardsprint: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Errorf(c, "Could not execute template reportcardsprint: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

}

func reportcardsGpaTermHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classSection := r.Form.Get("ClassSection")

	students, err := findStudents(c, sy, classSection)
	if err != nil {
		log.Errorf(c, "Could not retrieve students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	class, _, err := parseClassSection(classSection)
	if err != nil {
		log.Errorf(c, "Could not parse classSection %s: %s", classSection, err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	subjects, err := getSubjects(c, sy, class)
	if err != nil {
		log.Errorf(c, "Could not get subjects: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var reportcards []eoyGpaReportcard

	s1Term := Term{Semester, 1}
	s2Term := Term{Semester, 2}

	for _, stu := range students {

		stuType, err := getStudent(c, stu.ID)
		if err != nil {
			// TODO
			log.Errorf(c, "Could not get student: %s", err)
			continue
		}

		if stuType.Gender == "M" {
			stuType.Gender = "Male"
		} else if stuType.Gender == "F" {
			stuType.Gender = "Female"
		}

		var gpaRows []GPARow

		yearCredits := 0.0
		yearCreditsEarned := 0.0
		yearWeightedTotal := 0.0
		yearSubjectCount := 0.0
		yearMarksTotal := 0.0
		yearGpTotal := 0.0

		for _, subject := range subjects {

			gs := getGradingSystem(c, sy, class, subject)
			if gs == nil {
				continue
			}

			// TODO: add credits to gradingsystem instead of this
			sub, ok := gs.(Subject)
			if !ok {
				continue
			}

			if sub.S1Credits <= 0 && sub.S2Credits <= 0 {
				continue
			}

			marks, err := getStudentMarks(c, stu.ID, sy, subject)
			if err != nil {
				log.Errorf(c, "Could not get student marks: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}

			gpaRow := GPARow{
				Subject: gs.displayName(),

				S1Available: false,
				S1CA:        math.NaN(),
				S1CE:        math.NaN(),
				S1AV:        math.NaN(),
				S1WGP:       math.NaN(),

				S2Available: false,
				S2CA:        math.NaN(),
				S2CE:        math.NaN(),
				S2AV:        math.NaN(),
				S2WGP:       math.NaN(),

				FinalMark: math.NaN(),
				FinalGpa:  math.NaN(),
			}

			var s1Mark, s2Mark float64

			if sub.S1Credits > 0 {
				gpaRow.S1Available = true
				gs.evaluate(c, stu.ID, sy, s1Term, marks)

				s1Mark = gs.get100(s1Term, marks)

				if !math.IsNaN(s1Mark) {
					gpaRow.S1CA = sub.S1Credits
					yearCredits += gpaRow.S1CA

					if s1Mark >= 60 {
						gpaRow.S1CE = gpaRow.S1CA
						yearCreditsEarned += gpaRow.S1CE
					} else {
						gpaRow.S1CE = 0
					}
					_, gpaRow.S1WGP = gpaAvWgp(s1Mark)
					gpaRow.S1AV = s1Mark

					yearWeightedTotal += gpaRow.S1CE * s1Mark
				} else {
					s1Mark = 0
				}
			}

			if sub.S2Credits > 0 {
				gpaRow.S2Available = true
				gs.evaluate(c, stu.ID, sy, s2Term, marks)

				s2Mark = gs.get100(s2Term, marks)

				if !math.IsNaN(s2Mark) {
					gpaRow.S2CA = sub.S2Credits
					yearCredits += gpaRow.S2CA

					if s2Mark >= 60 {
						gpaRow.S2CE = gpaRow.S2CA
						yearCreditsEarned += gpaRow.S2CE
					} else {
						gpaRow.S2CE = 0
					}
					_, gpaRow.S2WGP = gpaAvWgp(s2Mark)
					gpaRow.S2AV = s2Mark

					yearWeightedTotal += gpaRow.S2CE * s2Mark
				} else {
					s2Mark = 0
				}
			}

			if gpaRow.S1Available && gpaRow.S2Available {
				gpaRow.FinalMark =
					(s1Mark*gpaRow.S1CE + s2Mark*gpaRow.S2CE) /
						(gpaRow.S1CA + gpaRow.S2CA)
				gpaRow.FinalGpa = (gpaRow.S1WGP + gpaRow.S2WGP) / 2
			} else if gpaRow.S1Available {
				gpaRow.FinalMark = s1Mark
				gpaRow.FinalGpa = gpaRow.S1WGP
			} else if gpaRow.S2Available {
				gpaRow.FinalMark = s2Mark
				gpaRow.FinalGpa = gpaRow.S2WGP
			}

			yearSubjectCount += 1
			yearMarksTotal += gpaRow.FinalMark
			yearGpTotal += gpaRow.FinalGpa

			gpaRows = append(gpaRows, gpaRow)
		}

		_, yearGpa := gpaAvWgp(yearWeightedTotal / yearCredits)
		_ = yearGpa
		yearFinalGpa := yearGpTotal / yearSubjectCount

		yearAv := formatMarkTrim(yearWeightedTotal / yearCredits)
		// Weighted average, ignored because of ministry
		_ = yearAv

		yearAverage := formatMarkTrim(yearMarksTotal / yearSubjectCount)

		reportcard := eoyGpaReportcard{
			Student: stuType,
			Rows:    gpaRows,

			CreditsEarned: yearCreditsEarned,
			YearAverage:   yearAverage,
			GPA:           yearFinalGpa,
		}

		reportcards = append(reportcards, reportcard)

	}

	data := struct {
		SY          string
		Class       string
		Reportcards []eoyGpaReportcard
	}{
		SY:          sy,
		Class:       class,
		Reportcards: reportcards,
	}

	// Note: not using render() because we don't want the base template
	templateFile := filepath.Join("template", "reportcardsEoyGpa.html")
	tmpl, err := htmltemplate.New("reportcardsEoyGpa.html").Funcs(funcMap).
		ParseFiles(templateFile)
	if err != nil {
		log.Errorf(c, "Could not parse template gpareportcard: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Errorf(c, "Could not execute template gpareportcard: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

}
