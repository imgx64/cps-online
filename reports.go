package main

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"fmt"
	"math"
	"net/http"
)

func init() {
	http.HandleFunc("/reports", accessHandler(reportsHandler))
	http.HandleFunc("/reports/select", accessHandler(reportsSelectHandler))
	http.HandleFunc("/reports/generate", accessHandler(reportsGenerateHandler))
}

const ReportProficiencyAndExcellence = "Proficiency and Excellence"
const ReportProficiencyAndExcellenceBySubject = "Proficiency and Excellence (By Subject)"
const ReportFinalMarksSummary = "Final Marks Summary"
const ReportSemesterTestResultComparison = "Semester Test Result Comparison"

var reportNames = []string{
	ReportProficiencyAndExcellence,
	ReportProficiencyAndExcellenceBySubject,
	ReportFinalMarksSummary,
	ReportSemesterTestResultComparison,
}

type ReportCell struct {
	Value   string
	Colspan int
	Rowspan int
}

func reportsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	schoolYears := getSchoolYears(c)

	data := struct {
		ReportNames []string
		SchoolYears []string
	}{
		reportNames,
		schoolYears,
	}

	if err := render(w, r, "reports", data); err != nil {
		log.Errorf(c, "Could not render template reports: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func reportsSelectHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	reportName := r.PostForm.Get("ReportName")

	schoolYears := r.PostForm["SchoolYears"]

	classes := make(map[string][]string)
	subjects := make(map[string][]string)
	for _, sy := range schoolYears {
		classes[sy] = getClasses(c, sy)
		subjects[sy] = getAllSubjects(c, sy)
	}

	switch reportName {
	case ReportProficiencyAndExcellence:
	case ReportProficiencyAndExcellenceBySubject:
	case ReportFinalMarksSummary:
		schoolYears = schoolYears[:1]
	case ReportSemesterTestResultComparison:
		schoolYears = schoolYears[:1]
	default:
	}

	data := struct {
		ReportName  string
		SchoolYears []string
		Classes     map[string][]string
		Subjects    map[string][]string
	}{
		reportName,
		schoolYears,
		classes,
		subjects,
	}

	if err := render(w, r, "reportsselect", data); err != nil {
		log.Errorf(c, "Could not render template reportsselect: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func reportsGenerateHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	reportName := r.PostForm.Get("ReportName")

	schoolYears := r.PostForm["SchoolYears"]

	classes := make([][]string, len(r.PostForm["classes-"+schoolYears[0]]))
	for i := 0; i < len(classes); i++ {
		classes[i] = make([]string, len(schoolYears))
		for syi, sy := range schoolYears {
			formClasses := r.PostForm["classes-"+sy]
			classes[i][syi] = formClasses[i]
		}
	}

	var classesSingle []string
	for _, class := range classes {
		classesSingle = append(classesSingle, class[0])
	}

	subjects := make([][]string, len(r.PostForm["subjects-"+schoolYears[0]]))
	for i := 0; i < len(subjects); i++ {
		subjects[i] = make([]string, len(schoolYears))
		for syi, sy := range schoolYears {
			formsubjects := r.PostForm["subjects-"+sy]
			subjects[i][syi] = formsubjects[i]
		}
	}

	var subjectsSingle []string
	for _, subjectSy := range subjects {
		subjectsSingle = append(subjectsSingle, subjectSy[0])
	}

	var report [][]ReportCell
	var err error
	switch reportName {
	case ReportProficiencyAndExcellence:
		report, err = generateReportProficiencyAndExcellence(c, schoolYears, classes, subjects)
	case ReportProficiencyAndExcellenceBySubject:
		report, err = generateReportProficiencyAndExcellenceBySubject(c, schoolYears, classes, subjects)
	case ReportFinalMarksSummary:
		report, err = generateReportFinalMarksSummary(c, schoolYears[0], classesSingle, subjectsSingle)
	case ReportSemesterTestResultComparison:
		report, err = generateReportSemesterTestResultComparison(c, schoolYears[0], classesSingle, subjectsSingle)
	default:
	}

	if err != nil {
		log.Errorf(c, "Could not get report: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		ReportName string
		Report     [][]ReportCell
	}{
		reportName,
		report,
	}

	if err := render(w, r, "reportsview", data); err != nil {
		log.Errorf(c, "Could not render template reportsview: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func generateReportProficiencyAndExcellence(c context.Context, schoolYears []string, classes, subjects [][]string) ([][]ReportCell, error) {
	var rows [][]ReportCell

	rows = append(rows, []ReportCell{
		{"", 2, 1},
		{"Excellence", len(subjects), 1},
		{"Proficiency", len(subjects), 1},
	})

	var subjectTitles []ReportCell
	subjectTitles = append(subjectTitles, ReportCell{"Year", 1, 1}, ReportCell{"Grade", 1, 1})
	for i := 0; i < 2; i++ {
		for _, subjectSy := range subjects {
			subjectTitles = append(subjectTitles, ReportCell{subjectSy[0], 1, 1})
		}
	}
	rows = append(rows, subjectTitles)

	for syi, sy := range schoolYears {
		syHeaderAdded := false
		for _, classSy := range classes {
			class := classSy[syi]

			var excellenceCells []ReportCell
			var proficiencyCells []ReportCell

			students, err := findStudentsSorted(c, sy, class+"|", false)
			if err != nil {
				log.Errorf(c, "Could not get students: %s", err)
				continue
			}

			for _, subjectSy := range subjects {
				subject := subjectSy[syi]

				excellenceCount, proficiencyCount, totalCount := getExcellenceProficiencyCounts(c, sy, class, subject, students)

				excellenceCells = append(excellenceCells,
					ReportCell{formatPercent(float64(excellenceCount*100) / float64(totalCount)), 1, 1})
				proficiencyCells = append(proficiencyCells,
					ReportCell{formatPercent(float64(proficiencyCount*100) / float64(totalCount)), 1, 1})
			}

			var row []ReportCell
			if !syHeaderAdded {
				syHeaderAdded = true
				row = append(row, ReportCell{sy, 1, len(classes)})
			}
			row = append(row, ReportCell{class, 1, 1})
			row = append(row, excellenceCells...)
			row = append(row, proficiencyCells...)

			rows = append(rows, row)
		}
	}

	return rows, nil
}
func generateReportProficiencyAndExcellenceBySubject(c context.Context, schoolYears []string, classes, subjects [][]string) ([][]ReportCell, error) {
	var rows [][]ReportCell

	rows = append(rows, []ReportCell{
		{"", 2, 1},
		{"Excellence", len(subjects), 1},
		{"Proficiency", len(subjects), 1},
	})

	for _, subjectSy := range subjects {
		subject := subjectSy[0]

		var classTitles []ReportCell
		classTitles = append(classTitles, ReportCell{subject, 1, len(schoolYears) + 1}, ReportCell{"", 1, 1})
		for i := 0; i < 2; i++ {
			for _, classSy := range classes {
				classTitles = append(classTitles, ReportCell{classSy[0], 1, 1})
			}
		}
		rows = append(rows, classTitles)

		for syi, sy := range schoolYears {
			var excellenceCells []ReportCell
			var proficiencyCells []ReportCell

			for _, classSy := range classes {
				class := classSy[syi]

				students, err := findStudentsSorted(c, sy, class+"|", false)
				if err != nil {
					log.Errorf(c, "Could not get students: %s", err)
					excellenceCells = append(excellenceCells, ReportCell{"", 1, 1})
					proficiencyCells = append(proficiencyCells, ReportCell{"", 1, 1})
					continue
				}

				excellenceCount, proficiencyCount, totalCount := getExcellenceProficiencyCounts(c, sy, class, subject, students)

				excellenceCells = append(excellenceCells,
					ReportCell{formatPercent(float64(excellenceCount*100) / float64(totalCount)), 1, 1})
				proficiencyCells = append(proficiencyCells,
					ReportCell{formatPercent(float64(proficiencyCount*100) / float64(totalCount)), 1, 1})
			}

			var row []ReportCell
			row = append(row, ReportCell{sy, 1, 1})
			row = append(row, excellenceCells...)
			row = append(row, proficiencyCells...)

			rows = append(rows, row)
		}
	}

	return rows, nil
}

func getExcellenceProficiencyCounts(c context.Context, sy, class, subject string, students []studentClass) (int, int, int) {
	term := Term{EndOfYear, 0}

	gs := getGradingSystem(c, sy, class, subject)
	if gs == nil {
		// class doesn't have subject
		return 0, 0, 0
	}

	excellenceCount := 0
	proficiencyCount := 0
	totalCount := 0
	for _, s := range students {
		marks, err := getStudentTermMarks(c, s.ID, sy, subject, term, gs)
		if err != nil {
			log.Errorf(c, "Could not get marks: %s", err)
			continue
		}
		m := make(studentMarks)
		m[term] = marks
		mark := gs.get100(term, m)
		if mark >= 90 && mark <= 100 {
			excellenceCount++
		}
		if mark >= 80 && mark <= 100 {
			proficiencyCount++
		}
		if !math.IsNaN(mark) {
			totalCount++
		}
	}

	return excellenceCount, proficiencyCount, totalCount
}

func generateReportFinalMarksSummary(c context.Context, sy string, classes, subjects []string) ([][]ReportCell, error) {
	var rows [][]ReportCell

	term := Term{EndOfYear, 0}

	for _, subject := range subjects {
		rows = append(rows, []ReportCell{
			{subject, 1, 2},
			{"Final Marks", len(classes), 1},
		})

		var classesTitles []ReportCell
		count90To100 := make([]int, len(classes))
		count80To90 := make([]int, len(classes))
		count70To80 := make([]int, len(classes))
		count60To70 := make([]int, len(classes))
		countLess60 := make([]int, len(classes))
		studentCount := make([]int, len(classes))
		for classi, class := range classes {
			classesTitles = append(classesTitles, ReportCell{class, 1, 1})

			gs := getGradingSystem(c, sy, class, subject)
			if gs == nil {
				// class doesn't have subject
				continue
			}
			students, err := findStudentsSorted(c, sy, class+"|", false)
			if err != nil {
				log.Errorf(c, "Could not get students: %s", err)
				continue
			}
			for _, s := range students {
				marks, err := getStudentTermMarks(c, s.ID, sy, subject, term, gs)
				if err != nil {
					log.Errorf(c, "Could not get marks: %s", err)
					continue
				}
				m := make(studentMarks)
				m[term] = marks
				// needs full m
				// gs.evaluate(c, s.ID, sy, term, m)
				mark := gs.get100(term, m)
				if mark >= 90 && mark <= 100 {
					count90To100[classi]++
					studentCount[classi]++
				} else if mark >= 80 && mark < 90 {
					count80To90[classi]++
					studentCount[classi]++
				} else if mark >= 70 && mark < 80 {
					count70To80[classi]++
					studentCount[classi]++
				} else if mark >= 60 && mark < 70 {
					count60To70[classi]++
					studentCount[classi]++
				} else if mark >= 0 && mark < 60 {
					countLess60[classi]++
					studentCount[classi]++
				}
			}
		}
		rows = append(rows, classesTitles)

		rows = append(rows, intRow("Number of Students 90 - 100", count90To100))
		rows = append(rows, intRow("Number of Students 80 - 90", count80To90))
		rows = append(rows, intRow("Number of Students 70 - 80", count70To80))
		rows = append(rows, intRow("Number of Students 60 - 70", count60To70))
		rows = append(rows, intRow("Number of Students 59.99 and below", countLess60))
		rows = append(rows, emptyCells(len(classes)+1))
		rows = append(rows, intRow("Total Number", studentCount))
		rows = append(rows, emptyRow(len(classes)+1))

		rows = append(rows, mapRow("Number of Proficient Students (80 - 100)", len(classes), func(i int) string {
			return fmt.Sprint(count90To100[i] + count80To90[i])
		}))
		rows = append(rows, mapRow("Number of Non-Proficient Students (79.99 and below)", len(classes), func(i int) string {
			return fmt.Sprint(count70To80[i] + count60To70[i] + countLess60[i])
		}))
		rows = append(rows, intRow("Total", studentCount))
		rows = append(rows, emptyRow(len(classes)+1))

		rows = append(rows, mapRow("Proficiency Rate (%)", len(classes), func(i int) string {
			return formatMark(float64(count90To100[i]+count80To90[i]) / float64(studentCount[i]) * float64(100))
		}))
		rows = append(rows, mapRow("Non-Proficiency Rate (%)", len(classes), func(i int) string {
			return formatMark(float64(count70To80[i]+count60To70[i]+countLess60[i]) / float64(studentCount[i]) * float64(100))
		}))
		rows = append(rows, mapRow("Total", len(classes), func(i int) string {
			return "100"
		}))
		rows = append(rows, emptyRow(len(classes)+1))

		rows = append(rows, intRow("Number of Excellent Students (90 - 100)", count90To100))
		rows = append(rows, mapRow("Excellence Rate (%)", len(classes), func(i int) string {
			return formatMark(float64(count90To100[i]) / float64(studentCount[i]) * float64(100))
		}))

		rows = append(rows, emptyRow(len(classes)+1))
	}

	return rows, nil
}

func generateReportSemesterTestResultComparison(c context.Context, sy string, classes, subjects []string) ([][]ReportCell, error) {
	var rows [][]ReportCell

	s1 := Term{Semester, 1}
	s2 := Term{Semester, 2}

	rows = append(rows, []ReportCell{
		{"", 1, 3},
		{"Proficient", len(subjects) * 2, 1},
		{"Total", 1, 3},
		{"", 1, 3 + len(classes)},
		{"", 1, 3},
		{"Proficiency Rate", len(subjects) * 2, 1},
	})

	var subjectTitles []ReportCell
	for i := 0; i < 2; i++ {
		for _, subject := range subjects {
			subjectTitles = append(subjectTitles, ReportCell{subject, 2, 1})
		}
	}
	rows = append(rows, subjectTitles)

	var s1s2Titles []ReportCell
	for i := 0; i < 2; i++ {
		for range subjects {
			s1s2Titles = append(s1s2Titles, ReportCell{"S1T", 1, 1}, ReportCell{"S2T", 1, 1})
		}
	}
	rows = append(rows, s1s2Titles)

	for _, class := range classes {
		var countCells []ReportCell
		var percentCells []ReportCell

		students, err := findStudentsSorted(c, sy, class+"|", false)
		if err != nil {
			log.Errorf(c, "Could not get students: %s", err)
			continue
		}

		for _, subject := range subjects {
			gs := getGradingSystem(c, sy, class, subject)
			if gs == nil {
				// class doesn't have subject
				countCells = append(countCells, ReportCell{"", 1, 1}, ReportCell{"", 1, 1})
				percentCells = append(percentCells, ReportCell{"", 1, 1}, ReportCell{"", 1, 1})
				continue
			}

			s1ProficientCount := 0
			s1TotalCount := 0
			s2ProficientCount := 0
			s2TotalCount := 0
			for _, s := range students {
				s1Marks, err := getStudentTermMarks(c, s.ID, sy, subject, s1, gs)
				if err != nil {
					log.Errorf(c, "Could not get marks: %s", err)
					continue
				}
				s1m := make(studentMarks)
				s1m[s1] = s1Marks
				s1Mark := gs.getExam(s1, s1m)
				if s1Mark >= 80 && s1Mark <= 100 {
					s1ProficientCount++
				}
				if !math.IsNaN(s1Mark) {
					s1TotalCount++
				}

				s2Marks, err := getStudentTermMarks(c, s.ID, sy, subject, s2, gs)
				if err != nil {
					log.Errorf(c, "Could not get marks: %s", err)
					continue
				}
				s2m := make(studentMarks)
				s2m[s2] = s2Marks
				s2Mark := gs.getExam(s2, s2m)
				if s2Mark >= 80 && s2Mark <= 100 {
					s2ProficientCount++
				}
				if !math.IsNaN(s2Mark) {
					s2TotalCount++
				}
			}
			countCells = append(countCells,
				ReportCell{fmt.Sprint(s1ProficientCount), 1, 1},
				ReportCell{fmt.Sprint(s2ProficientCount), 1, 1})
			percentCells = append(percentCells,
				ReportCell{formatMark(float64(s1ProficientCount*100) / float64(s1TotalCount)), 1, 1},
				ReportCell{formatMark(float64(s2ProficientCount*100) / float64(s2TotalCount)), 1, 1})

		}

		var row []ReportCell
		row = append(row, ReportCell{class, 1, 1})
		row = append(row, countCells...)
		row = append(row, ReportCell{fmt.Sprint(len(students)), 1, 1})
		row = append(row, ReportCell{class, 1, 1})
		row = append(row, percentCells...)
		rows = append(rows, row)
	}

	return rows, nil
}

func emptyRow(count int) []ReportCell {
	return []ReportCell{{"\u00A0", count, 1}}
}

func emptyCells(count int) []ReportCell {
	row := []ReportCell{}
	for i := 0; i < count; i++ {
		row = append(row, ReportCell{"\u00A0", 1, 1})
	}
	return row
}

func intRow(title string, ints []int) []ReportCell {
	row := []ReportCell{}
	row = append(row, ReportCell{title, 1, 1})
	for _, val := range ints {
		row = append(row, ReportCell{fmt.Sprint(val), 1, 1})
	}
	return row
}

func mapRow(title string, count int, f func(int) string) []ReportCell {
	row := []ReportCell{}
	row = append(row, ReportCell{title, 1, 1})
	for i := 0; i < count; i++ {
		row = append(row, ReportCell{f(i), 1, 1})
	}
	return row
}
