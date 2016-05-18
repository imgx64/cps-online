// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	htmltemplate "html/template"
	"math"
	"net/http"
	"path/filepath"
	"strings"
	"time"
)

func init() {
	http.HandleFunc("/gpareportcard", accessHandler(gpaReportcardHandler))
}

type GPAYear struct {
	Class string
	SY    string

	Rows []GPARow

	CreditsEarned float64
	YearAverage   string
	GPA           float64
}

type GPARow struct {
	Subject string

	S1Available bool
	S1CA        float64
	S1CE        float64
	S1AV        string
	S1WGP       float64

	S2Available bool
	S2CA        float64
	S2CE        float64
	S2AV        string
	S2WGP       float64

	FinalMark float64
	FinalGpa  float64
}

func gpaReportcardHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	id := r.Form.Get("id")

	stu, err := getStudent(c, id)
	if err != nil {
		log.Errorf(c, "Could not get student: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var gpaYears []GPAYear

	var includedClassesSome [][]string
	var includedClassesLast = false
	var includedClassesAll []string
	totalCreditsSome := 0.0
	totalCreditsAll := 0.0
	totalCreditsEarnedSome := 0.0
	totalCreditsEarnedAll := 0.0
	totalWeightedTotalSome := 0.0
	totalWeightedTotalAll := 0.0

	s1Term := Term{Semester, 1}
	s2Term := Term{Semester, 2}

	for _, sy := range getSchoolYears(c) {
		var gpaRows []GPARow

		class, _, err := getStudentClass(c, id, sy)
		if err != nil {
			continue
		}
		if class == "" {
			continue
		}

		var classSetting classSetting
		for _, cs := range getClassSettings(c, sy) {
			if cs.Class == class {
				classSetting = cs
				break
			}
		}

		yearCredits := 0.0
		yearCreditsEarned := 0.0
		yearWeightedTotal := 0.0
		yearSubjectCount := 0.0
		yearMarksTotal := 0.0
		yearGpTotal := 0.0

		subjects, err := getSubjects(c, sy, class)
		if err != nil {
			log.Errorf(c, "Could not get subjects %s %s: %s", sy, class, err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
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

			marks, err := getStudentMarks(c, id, sy, subject)
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
				S1AV:        "",
				S1WGP:       math.NaN(),

				S2Available: false,
				S2CA:        math.NaN(),
				S2CE:        math.NaN(),
				S2AV:        "",
				S2WGP:       math.NaN(),

				FinalMark: math.NaN(),
				FinalGpa:  math.NaN(),
			}

			var s1Mark, s2Mark float64

			if sub.S1Credits > 0 {
				gpaRow.S1Available = true
				gs.evaluate(s1Term, marks)

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
					gpaRow.S1AV = formatMarkTrim(s1Mark)

					yearWeightedTotal += gpaRow.S1CE * s1Mark
				}
			}

			if sub.S2Credits > 0 {
				gpaRow.S2Available = true
				gs.evaluate(s2Term, marks)

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
					gpaRow.S2AV = formatMarkTrim(s2Mark)

					yearWeightedTotal += gpaRow.S2CE * s2Mark
				}
			}

			if !math.IsNaN(gpaRow.S1CE) && !math.IsNaN(gpaRow.S2CE) {
				gpaRow.FinalMark =
					(s1Mark*gpaRow.S1CE + s2Mark*gpaRow.S2CE) /
						(gpaRow.S1CA + gpaRow.S2CA)
				gpaRow.FinalGpa = (gpaRow.S1WGP + gpaRow.S2WGP) / 2
			} else if !math.IsNaN(gpaRow.S1CE) {
				gpaRow.FinalMark = s1Mark
				gpaRow.FinalGpa = gpaRow.S1WGP
			} else if !math.IsNaN(gpaRow.S2CE) {
				gpaRow.FinalMark = s2Mark
				gpaRow.FinalGpa = gpaRow.S2WGP
			}

			yearSubjectCount += 1
			yearMarksTotal += gpaRow.FinalMark
			yearGpTotal += gpaRow.FinalGpa

			gpaRows = append(gpaRows, gpaRow)
		}

		if len(gpaRows) == 0 {
			continue
		}

		_, yearGpa := gpaAvWgp(yearWeightedTotal / yearCredits)
		_ = yearGpa
		yearFinalGpa := yearGpTotal / yearSubjectCount

		yearAv := formatMarkTrim(yearWeightedTotal / yearCredits)
		// Weighted average, ignored because of ministry
		_ = yearAv
		yearAverage := formatMarkTrim(yearMarksTotal / yearSubjectCount)

		if !classSetting.IgnoreInTotalGPA {
			if includedClassesLast {
				n := len(includedClassesSome) - 1
				lastClasses := includedClassesSome[n]
				lastClasses = append(lastClasses, trimStream(class))
				includedClassesSome[n] = lastClasses
			} else {
				includedClassesLast = true
				includedClassesSome = append(includedClassesSome, []string{trimStream(class)})
			}
			totalCreditsSome += yearCredits
			totalCreditsEarnedSome += yearCreditsEarned
			totalWeightedTotalSome += yearWeightedTotal
		} else {
			includedClassesLast = false
		}

		includedClassesAll = append(includedClassesAll, trimStream(class))
		totalCreditsAll += yearCredits
		totalCreditsEarnedAll += yearCreditsEarned
		totalWeightedTotalAll += yearWeightedTotal

		gpaYear := GPAYear{
			Class: trimStream(class),
			SY:    sy,

			Rows: gpaRows,

			CreditsEarned: yearCreditsEarned,
			YearAverage:   yearAverage,
			GPA:           yearFinalGpa,
		}

		gpaYears = append(gpaYears, gpaYear)

	}

	_, cumulativeGpaSome := gpaAvWgp(totalWeightedTotalSome / totalCreditsSome)
	cumulateAvgSome := formatMarkTrim(totalWeightedTotalSome / totalCreditsSome)
	_, cumulativeGpaAll := gpaAvWgp(totalWeightedTotalAll / totalCreditsAll)
	cumulateAvgAll := formatMarkTrim(totalWeightedTotalAll / totalCreditsAll)

	dob := ""
	if stu.DateOfBirth.After(time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)) {
		dob = stu.DateOfBirth.Format("2006-01-02")
	}

	data := struct {
		Name        string
		Sex         string
		Nationality string
		DOB         string
		ID          string
		Stream      string
		CPR         string

		Years []GPAYear

		IncludedClassesSome string
		TotalCreditsSome    float64
		CumulativeGpaSome   float64
		CumulativeAvgSome   string

		IncludedClassesAll string
		TotalCreditsAll    float64
		CumulativeGpaAll   float64
		CumulativeAvgAll   string
	}{
		stu.Name,
		stu.Gender,
		stu.Nationality,
		dob,
		stu.ID,
		stu.Stream,
		stu.CPR,

		gpaYears,

		multiGradesStr(includedClassesSome),
		totalCreditsSome,
		cumulativeGpaSome,
		cumulateAvgSome,

		gradesStr(includedClassesAll),
		totalCreditsAll,
		cumulativeGpaAll,
		cumulateAvgAll,
	}

	// Note: not using render() because we don't want the base template
	templateFile := filepath.Join("template", "gpareportcard.html")
	tmpl, err := htmltemplate.New("gpareportcard.html").Funcs(funcMap).
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

func gpaAvWgp(mark float64) (string, float64) {
	if math.IsNaN(mark) || mark > 100 {
		return "N/A", math.NaN()
	} else if mark >= 97 {
		return "A+", 4
	} else if mark >= 93 {
		return "A", 4
	} else if mark >= 90 {
		return "A-", 3.7
	} else if mark >= 87 {
		return "B+", 3.3
	} else if mark >= 83 {
		return "B", 3
	} else if mark >= 80 {
		return "B-", 2.7
	} else if mark >= 77 {
		return "C+", 2.3
	} else if mark >= 73 {
		return "C", 2
	} else if mark >= 70 {
		return "C-", 1.7
	} else if mark >= 67 {
		return "D+", 1.3
	} else if mark >= 63 {
		return "D", 1
	} else if mark >= 60 {
		return "D-", 1
	} else if mark >= 0 {
		return "F", 0
	}

	return "N/A", math.NaN()
}

func multiGradesStr(multiGrades [][]string) string {
	s := ""

	for i, grades := range multiGrades {
		if len(grades) == 0 {
			continue
		}
		if i != 0 {
			s += ", "
		}
		s += gradesStr(grades)
	}

	return s
}

func gradesStr(grades []string) string {
	switch len(grades) {
	case 0:
		return ""
	case 1:
		return grades[0]
	case 2:
		return grades[0] + ", " + grades[1]
	default:
		return grades[0] + " - " + grades[len(grades)-1]
	}
}

func notDigit(r rune) bool {
	if r < '0' || r > '9' {
		return true
	}
	return false
}

func trimStream(class string) string {
	return strings.TrimRightFunc(class, notDigit)
}
