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
	htmltemplate "html/template"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
)

func init() {
	http.HandleFunc("/progressreports/settings", accessHandler(progressreportsSettingsHandler))
	http.HandleFunc("/progressreports/settings/save", accessHandler(progressreportsSettingsSaveHandler))
	http.HandleFunc("/progressreports/report", accessHandler(progressreportsReportHandler))
	http.HandleFunc("/progressreports/report/save", accessHandler(progressreportsReportSaveHandler))
	http.HandleFunc("/progressreports/report/print", accessHandler(progressreportsReportPrintHandler))
}

type ProgressReportSettings struct {
	SchoolYear  string
	Class       string
	ShortName   string
	Description string
	Language    string
	Rows        []ProgressReportRow
}

type ProgressReportRow struct {
	Description string
	Section     bool
	Deleted     bool
}

func getProgressReportSettings(c context.Context, sy, class, shortName string) (ProgressReportSettings, error) {
	keyStr := fmt.Sprintf("%s|%s|%s", sy, class, shortName)
	key := datastore.NewKey(c, "progress_report_settings", keyStr, 0, nil)

	var prs ProgressReportSettings
	err := nds.Get(c, key, &prs)
	if err == datastore.ErrNoSuchEntity {
		return ProgressReportSettings{}, nil
	} else if err != nil {
		return ProgressReportSettings{}, err
	}
	return prs, nil
}

func getClassProgressReportSettings(c context.Context, sy, class string) ([]ProgressReportSettings, error) {
	q := datastore.NewQuery("progress_report_settings")
	q = q.Filter("SchoolYear =", sy)
	q = q.Filter("Class =", class)
	q = q.Project("ShortName")

	var result []ProgressReportSettings
	if _, err := q.GetAll(c, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func getAllProgressReportSettings(c context.Context, sy string) (map[string][]ProgressReportSettings, error) {
	q := datastore.NewQuery("progress_report_settings")
	q = q.Filter("SchoolYear =", sy)
	q = q.Project("Class", "ShortName")

	var list []ProgressReportSettings
	if _, err := q.GetAll(c, &list); err != nil {
		return nil, err
	}

	result := make(map[string][]ProgressReportSettings)
	for _, prs := range list {
		classList := result[prs.Class]
		classList = append(classList, prs)
		result[prs.Class] = classList
	}
	return result, nil
}

func saveProgressReportSettings(c context.Context, prs ProgressReportSettings) error {
	keyStr := fmt.Sprintf("%s|%s|%s", prs.SchoolYear, prs.Class, prs.ShortName)
	key := datastore.NewKey(c, "progress_report_settings", keyStr, 0, nil)

	_, err := nds.Put(c, key, &prs)
	if err != nil {
		return err
	}
	return nil
}

type ProgressReportData struct {
	Marks       []string
	Comments    string
	Teacher     int64
	TeacherName string `datastore:"-"`
}

type ProgressReportPrintData struct {
	StudentName string
	PRD         ProgressReportData
	Absence     float64
	Tardiness   float64
}

func getProgressReportData(c context.Context, term Term, shortName string, studentId string) (ProgressReportData, error) {
	keyStr := fmt.Sprintf("%s|%s|%s", term.Value(), shortName, studentId)
	key := datastore.NewKey(c, "progress_report_data", keyStr, 0, nil)

	var prd ProgressReportData
	err := nds.Get(c, key, &prd)
	if err == datastore.ErrNoSuchEntity {
		return ProgressReportData{}, nil
	} else if err != nil {
		return ProgressReportData{}, err
	}
	return prd, nil
}

func saveProgressReportData(c context.Context, term Term, shortName string, studentId string, prd ProgressReportData) error {
	keyStr := fmt.Sprintf("%s|%s|%s", term.Value(), shortName, studentId)
	key := datastore.NewKey(c, "progress_report_data", keyStr, 0, nil)

	_, err := nds.Put(c, key, &prd)
	if err != nil {
		return err
	}
	return nil
}

type ProgressReportMark struct {
	Value        string
	Number       string
	Letter       string
	ArabicLetter string
}

var progressReportMarks = []ProgressReportMark{
	{"1", "1", "C", "1"},
	{"2", "2", "M", "2"},
	{"3", "3", "R", "3"},
	{"4", "4", "E", "4"},
	{"0", "0", "N/A", "غ/م"},
}

var progressReportMarksMap = map[string]ProgressReportMark{
	"1": progressReportMarks[0],
	"2": progressReportMarks[1],
	"3": progressReportMarks[2],
	"4": progressReportMarks[3],
	"0": progressReportMarks[4],
}

func progressreportsSettingsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	class := r.Form.Get("class")
	shortName := r.Form.Get("shortName")

	prs, err := getProgressReportSettings(c, sy, class, shortName)
	if err != nil {
		log.Errorf(c, "Could not get ProgressReportSettings: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classes := getClasses(c, sy)

	data := struct {
		Classes   []string
		Languages []string

		PRS ProgressReportSettings
	}{
		classes,
		[]string{"Arabic", "English"},

		prs,
	}

	if err := render(w, r, "progressreportsettings", data); err != nil {
		log.Errorf(c, "Could not render template progressreportsettings: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func progressreportsSettingsSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	prs := ProgressReportSettings{
		SchoolYear:  sy,
		Class:       r.PostForm.Get("Class"),
		ShortName:   r.PostForm.Get("ShortName"),
		Description: r.PostForm.Get("Description"),
		Language:    r.PostForm.Get("Language"),
	}

	for i := 0; ; i++ {
		_, ok := r.PostForm[fmt.Sprintf("prr-description-%d", i)]
		if !ok {
			break
		}

		prr := ProgressReportRow{
			Description: r.PostForm.Get(fmt.Sprintf("prr-description-%d", i)),
			Section:     r.PostForm.Get(fmt.Sprintf("prr-type-%d", i)) == "Section",
			Deleted:     r.PostForm.Get(fmt.Sprintf("prr-type-%d", i)) == "Delete",
		}

		prs.Rows = append(prs.Rows, prr)
	}

	// Delete trailing "Deleted"
	for i := len(prs.Rows) - 1; i >= 0; i-- {
		if prs.Rows[i].Deleted {
			prs.Rows = prs.Rows[:i]
		} else {
			break
		}
	}

	if err := saveProgressReportSettings(c, prs); err != nil {
		log.Errorf(c, "Could not save ProgressReportSettings: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings", http.StatusFound)
}

func progressreportsReportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	stu, err := getStudent(c, r.Form.Get("StudentId"))
	if err != nil {
		log.Errorf(c, "Could not retrieve student details: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	sc, err := getStudentClass(c, stu.ID, sy)
	if err != nil {
		log.Errorf(c, "Could not retrieve student class details: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	term, err := parseTerm(r.Form.Get("Term"))
	if err != nil {
		log.Errorf(c, "Could not parse term: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	shortName := r.Form.Get("ShortName")
	prs, err := getProgressReportSettings(c, sy, sc.Class, shortName)
	if err != nil {
		log.Errorf(c, "Could not retrieve progress report settings: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var studentName string
	if prs.Language == "Arabic" && stu.ArabicName != "" {
		studentName = stu.ArabicName
	} else {
		studentName = stu.Name
	}

	teachers, err := getEmployees(c, true, "Teacher")
	if err != nil {
		log.Errorf(c, "Could not get teachers: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	prd, err := getProgressReportData(c, term, prs.ShortName, stu.ID)
	if err != nil {
		log.Errorf(c, "Could not get ProgressReportData: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	if len(prd.Marks) < len(prs.Rows) {
		prd.Marks = append(prd.Marks, make([]string, len(prs.Rows)-len(prd.Marks))...)
	}

	if prd.Teacher == 0 {
		user, err := getUser(c)
		if err == nil && user.Roles.Teacher {
			emp, err := getEmployeeFromEmail(c, user.Email)
			if err == nil {
				prd.Teacher = emp.ID
			}
		}
	}

	data := struct {
		Teachers []employeeType
		Marks    []ProgressReportMark

		Class       string
		Section     string
		Term        Term
		PRS         ProgressReportSettings
		StudentId   string
		StudentName string

		PRD ProgressReportData
	}{
		teachers,
		progressReportMarks,

		sc.Class,
		sc.Section,
		term,
		prs,
		stu.ID,
		studentName,

		prd,
	}

	if err := render(w, r, "progressreport", data); err != nil {
		log.Errorf(c, "Could not render template progressreport: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func progressreportsReportSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	stu, err := getStudent(c, r.PostForm.Get("StudentId"))
	if err != nil {
		log.Errorf(c, "Could not retrieve student details: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	sc, err := getStudentClass(c, stu.ID, sy)
	if err != nil {
		log.Errorf(c, "Could not retrieve student class details: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	term, err := parseTerm(r.PostForm.Get("Term"))
	if err != nil {
		log.Errorf(c, "Could not parse term: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	shortName := r.PostForm.Get("ShortName")
	prs, err := getProgressReportSettings(c, sy, sc.Class, shortName)
	if err != nil {
		log.Errorf(c, "Could not retrieve progress report settings: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	marks := make([]string, len(prs.Rows))

	for i, row := range prs.Rows {
		if row.Section || row.Deleted {
			continue
		}
		marks[i] = r.PostForm.Get(fmt.Sprintf("ProgressReportMark-%d", i))
	}

	teacherId, err := strconv.ParseInt(r.PostForm.Get("Teacher"), 10, 64)
	if err != nil {
		log.Errorf(c, "Could not parse teacher ID: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	prd := ProgressReportData{
		Marks:    marks,
		Comments: r.PostForm.Get("Comments"),
		Teacher:  teacherId,
	}

	if err := saveProgressReportData(c, term, prs.ShortName, stu.ID, prd); err != nil {
		log.Errorf(c, "Could not save ProgressReportData: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	urlValues := url.Values{
		"Term":         []string{term.Value()},
		"ClassSection": []string{fmt.Sprintf("%s|%s", sc.Class, sc.Section)},
		"Subject":      []string{"Progress Reports"},
	}
	redirectURL := fmt.Sprintf("/marks?%s", urlValues.Encode())
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func progressreportsReportPrintHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	term, err := parseTerm(r.Form.Get("Term"))
	if err != nil {
		log.Errorf(c, "Could not parse term: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	shortName := r.Form.Get("ShortName")

	var students []studentType
	var class, section string

	if _, ok := r.Form["StudentId"]; ok {
		stu, err := getStudent(c, r.Form.Get("StudentId"))
		if err != nil {
			log.Errorf(c, "Could not retrieve student details: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		sc, err := getStudentClass(c, stu.ID, sy)
		if err != nil {
			log.Errorf(c, "Could not retrieve student class details: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		students = []studentType{stu}
		class = sc.Class
		section = sc.Section

	} else if _, ok := r.Form["ClassSection"]; ok {
		class, section, err = parseClassSection(r.Form.Get("ClassSection"))
		if err != nil {
			log.Errorf(c, "Invalid ClassSection: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		scs, err := findStudents(c, sy, class+"|"+section)
		if err != nil {
			log.Errorf(c, "Invalid ClassSection: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		for _, sc := range scs {
			stu, err := getStudent(c, sc.ID)
			if err != nil {
				log.Errorf(c, "Could not retrieve student details: %s", err)
				continue
			}
			students = append(students, stu)
		}

	}

	prs, err := getProgressReportSettings(c, sy, class, shortName)
	if err != nil {
		log.Errorf(c, "Could not retrieve progress report settings: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var reports []ProgressReportPrintData

	for _, stu := range students {
		var studentName string
		if prs.Language == "Arabic" && stu.ArabicName != "" {
			studentName = stu.ArabicName
		} else {
			studentName = stu.Name
		}

		prd, err := getProgressReportData(c, term, prs.ShortName, stu.ID)
		if err != nil {
			log.Errorf(c, "Could not get ProgressReportData: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		if len(prd.Marks) < len(prs.Rows) {
			prd.Marks = append(prd.Marks, make([]string, len(prs.Rows)-len(prd.Marks))...)
		}

		if prd.Teacher != 0 {
			emp, err := getEmployee(c, fmt.Sprintf("%d", prd.Teacher))
			if err != nil {
				log.Errorf(c, "Could not get teacher: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			if prs.Language == "Arabic" && emp.ArabicName != "" {
				prd.TeacherName = emp.ArabicName
			} else {
				prd.TeacherName = emp.Name
			}
		}

		var absence, tardiness float64
		attM, err := getStudentMarks(c, stu.ID, sy, "Attendance")
		if err != nil {
			log.Errorf(c, "Could not get Attendance: %s", err)
		} else {
			// see attendanceGradingSystem
			gs := getGradingSystem(c, sy, class, "Attendance")
			gs.evaluate(c, stu.ID, sy, term, attM)
			m := attM[term]
			for i, n := range m {
				if i < len(m)/2 {
					absence += n
				} else {
					tardiness += n
				}
			}
		}

		reports = append(reports, ProgressReportPrintData{
			studentName,
			prd,
			absence,
			tardiness,
		})
	}

	data := struct {
		Marks []ProgressReportMark

		SY      string
		Class   string
		Section string
		Term    Term
		PRS     ProgressReportSettings

		Reports []ProgressReportPrintData
	}{
		progressReportMarks,

		sy,
		class,
		section,
		term,
		prs,

		reports,
	}

	var templateName string
	if prs.Language == "Arabic" {
		templateName = "progressreportprintarabic.html"
	} else {
		templateName = "progressreportprint.html"
	}

	// Note: not using render() because we don't want the base template
	templateFile := filepath.Join("template", templateName)
	tmpl, err := htmltemplate.New(templateName).Funcs(funcMap).ParseFiles(templateFile)
	if err != nil {
		log.Errorf(c, "Could not parse template %s: %s", templateName, err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if err := tmpl.Execute(w, data); err != nil {
		log.Errorf(c, "Could not execute template %s: %s", templateName, err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
