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
	"strconv"
	"strings"
)

func init() {
	http.HandleFunc("/settings", accessHandler(settingsHandler))
	http.HandleFunc("/settings/saveschoolyear", accessHandler(settingsSaveSchoolYearHandler))
	http.HandleFunc("/settings/savesections", accessHandler(settingsSaveSectionsHandler))
	http.HandleFunc("/settings/addclass", accessHandler(settingsAddClassHandler))
	http.HandleFunc("/settings/addschoolyear", accessHandler(settingsAddSYHandler))
	http.HandleFunc("/settings/addsubject", accessHandler(settingsAddSubjectHandler))
	http.HandleFunc("/settings/deletesubject", accessHandler(settingsDeleteSubjectHandler))
	http.HandleFunc("/settings/access", accessHandler(settingsAccessHandler))
}

const startYear = 2012

type maxSchoolYearSetting struct {
	Value int
}

func getSchoolYears(c context.Context) []string {
	currentYear := startYear
	maxSy := getMaxSchoolYear(c)

	if maxSy < currentYear {
		log.Warningf(c, "MaxSchoolYear is less than startYear constant")
		currentYear = maxSy
	}

	schoolYears := []string{}
	for ; currentYear <= maxSy; currentYear++ {
		sy := fmt.Sprintf("%d-%d", currentYear, currentYear+1)
		schoolYears = append(schoolYears, sy)
	}

	return schoolYears
}

func getMaxSchoolYear(c context.Context) int {
	key := datastore.NewKey(c, "settings", "max_school_year", 0, nil)

	setting := maxSchoolYearSetting{}
	err := nds.Get(c, key, &setting)
	var sy int
	if err == nil {
		sy = setting.Value
	} else {
		log.Warningf(c, "Could not get max school year: %s\nUsing defaults instead", err)
		sy = startYear
	}

	return sy
}

func saveMaxSchoolYear(c context.Context, value int) error {
	key := datastore.NewKey(c, "settings", "max_school_year", 0, nil)

	_, err := nds.Put(c, key, &maxSchoolYearSetting{value})
	if err != nil {
		return err
	}
	return nil
}

type schoolYearSetting struct {
	Value string
}

func getSchoolYear(c context.Context) string {

	key := datastore.NewKey(c, "settings", "school_year", 0, nil)

	setting := schoolYearSetting{}
	err := nds.Get(c, key, &setting)
	var sy string
	if err == nil {
		sy = setting.Value
	} else {
		log.Warningf(c, "Could not get school year: %s\nUsing defaults instead", err)
		sy = fmt.Sprintf("%d-%d", startYear, startYear+1)
	}

	return sy
}

func saveSchoolYear(c context.Context, sy string) error {

	key := datastore.NewKey(c, "settings", "school_year", 0, nil)

	_, err := nds.Put(c, key, &schoolYearSetting{sy})
	if err != nil {
		return err
	}
	return nil
}

type classSetting struct {
	Class         string
	MaxSection    string
	LetterSystem  string
	QuarterWeight float64
}

type classSettings struct {
	Value []classSetting
}

func getClassSettings(c context.Context, sy string) []classSetting {
	key := datastore.NewKey(c, "settings", "class-settings-"+sy, 0, nil)

	setting := classSettings{}
	if err := nds.Get(c, key, &setting); err != nil {
		log.Warningf(c, "Could not get class settings: %s\n. Returning empty slice instead", err)
		return []classSetting{}
	}

	return setting.Value
}

func saveClassSettings(c context.Context, settings []classSetting) error {
	sy := getSchoolYear(c)

	key := datastore.NewKey(c, "settings", "class-settings-"+sy, 0, nil)
	_, err := nds.Put(c, key, &classSettings{settings})
	if err != nil {
		return err
	}
	return nil
}

type subjectsSettings struct {
	Value []string
}

func getAllSubjects(c context.Context, sy string) []string {
	key := datastore.NewKey(c, "settings", "subjects-"+sy, 0, nil)

	setting := subjectsSettings{}
	if err := nds.Get(c, key, &setting); err != nil {
		log.Warningf(c, "Could not get subjects: %s\n. Returning empty slice instead", err)
		return []string{}
	}

	return setting.Value
}

func saveAllSubjects(c context.Context, sy string, subjects []string) error {
	key := datastore.NewKey(c, "settings", "subjects-"+sy, 0, nil)
	_, err := nds.Put(c, key, &subjectsSettings{subjects})
	if err != nil {
		return err
	}
	return nil
}

type staffAccessSetting struct {
	Value bool
}

func getStaffAccess(c context.Context) bool {

	key := datastore.NewKey(c, "settings", "staff_access", 0, nil)

	setting := staffAccessSetting{}
	err := nds.Get(c, key, &setting)
	var access bool
	if err == nil {
		access = setting.Value
	} else {
		log.Warningf(c, "Could not get staff access: %s\nUsing defaults instead", err)
		access = false
	}

	return access
}

func saveStaffAccess(c context.Context, access bool) error {

	key := datastore.NewKey(c, "settings", "staff_access", 0, nil)

	_, err := nds.Put(c, key, &staffAccessSetting{access})
	if err != nil {
		return err
	}
	return nil
}

type studentAccessValue struct {
	Term   Term
	Access bool
}

type studentAccessSetting struct {
	Value []studentAccessValue
}

func getStudentAccess(c context.Context) map[Term]bool {

	key := datastore.NewKey(c, "settings", "student_access", 0, nil)

	setting := studentAccessSetting{}
	err := nds.Get(c, key, &setting)
	var access []studentAccessValue
	if err == nil {
		access = setting.Value
	} else {
		log.Warningf(c, "Could not get student access: %s\nUsing defaults instead", err)
		access = nil
	}

	result := make(map[Term]bool)
	for _, sav := range access {
		result[sav.Term] = sav.Access
	}

	return result
}

func saveStudentAccess(c context.Context, access map[Term]bool) error {

	key := datastore.NewKey(c, "settings", "student_access", 0, nil)

	var savs []studentAccessValue
	for k, v := range access {
		savs = append(savs, studentAccessValue{k, v})
	}

	_, err := nds.Put(c, key, &studentAccessSetting{savs})
	if err != nil {
		return err
	}
	return nil
}

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sectionChoices := sectionsUntil("Z")

	var letterSystemChoices []string
	for key, _ := range letterSystemMap {
		letterSystemChoices = append(letterSystemChoices, key)
	}

	schoolYears := getSchoolYears(c)
	sy := getSchoolYear(c)

	staffAccess := getStaffAccess(c)
	studentAccess := getStudentAccess(c)

	settings := getClassSettings(c, sy)

	maxSchoolYear := getMaxSchoolYear(c)
	nextSchoolYear := fmt.Sprintf("%d-%d", maxSchoolYear+1, maxSchoolYear+2)

	subjects := getAllSubjects(c, sy)

	data := struct {
		SectionChoices      []string
		LetterSystemChoices []string
		Terms               []Term

		StaffAccess   bool
		StudentAccess map[Term]bool

		SchoolYears []string
		SY          string

		ClassSettings []classSetting

		Subjects []string

		NextSchoolYear string
	}{
		sectionChoices,
		letterSystemChoices,
		terms,

		staffAccess,
		studentAccess,

		schoolYears,
		sy,

		settings,

		subjects,

		nextSchoolYear,
	}

	if err := render(w, r, "settings", data); err != nil {
		log.Errorf(c, "Could not render template settings: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func settingsSaveSchoolYearHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	sy := r.PostForm.Get("sy")
	if sy != "" {
		if err := saveSchoolYear(c, sy); err != nil {
			log.Errorf(c, "Could not save school year: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	// TODO: message of success
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func settingsSaveSectionsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	sy := getSchoolYear(c)

	settings := getClassSettings(c, sy)
	for i, classSetting := range settings {
		section := r.PostForm.Get("sections-" + classSetting.Class)
		if validSection(section) {
			classSetting.MaxSection = section
			settings[i] = classSetting
		}

		ls := r.PostForm.Get("letter-system-" + classSetting.Class)
		if _, ok := letterSystemMap[ls]; ok {
			classSetting.LetterSystem = ls
			settings[i] = classSetting
		}

		qwStr := r.PostForm.Get("quarter-weight-" + classSetting.Class)
		qw, err := strconv.ParseFloat(qwStr, 64)
		if err == nil && qw >= 0 && qw <= 50.0 {
			classSetting.QuarterWeight = qw
			settings[i] = classSetting
		}
	}

	if err := saveClassSettings(c, settings); err != nil {
		log.Errorf(c, "Could not save max sections: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func settingsAddClassHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	if class := r.PostForm.Get("class"); class != "" {
		sy := getSchoolYear(c)
		settings := getClassSettings(c, sy)

		for _, classSetting := range settings {
			if class == classSetting.Class {
				log.Errorf(c, "Class already exists: %s", class)
				renderErrorMsg(w, r, http.StatusInternalServerError, fmt.Sprintf("Class already exists: %s", class))
				return
			}
		}

		newSetting := classSetting{
			Class:         class,
			MaxSection:    "A",
			LetterSystem:  "ABCDF",
			QuarterWeight: 0.0,
		}
		settings = append(settings, newSetting)

		if err := saveClassSettings(c, settings); err != nil {
			log.Errorf(c, "Could not add class: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	// TODO: message of success
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func settingsAddSYHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	maxSchoolYear := getMaxSchoolYear(c)
	saveMaxSchoolYear(c, maxSchoolYear+1)

	// TODO: message of success
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func settingsAddSubjectHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	subject := r.PostForm.Get("subject")
	if subject == "" {
		renderErrorMsg(w, r, http.StatusBadRequest, "Empty subject")
		return
	}

	subjects := getAllSubjects(c, sy)
	for _, s := range subjects {
		if s == subject {
			renderErrorMsg(w, r, http.StatusBadRequest, "Subject already exists")
			return
		}
	}

	subjects = append(subjects, subject)
	if err := saveAllSubjects(c, sy, subjects); err != nil {
		log.Errorf(c, "Could not save subjects: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func settingsDeleteSubjectHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	subject := r.PostForm.Get("subject")

	subjects := getAllSubjects(c, sy)
	var newSubjects []string
	for _, s := range subjects {
		if s != subject {
			newSubjects = append(newSubjects, s)
		}
	}

	if err := saveAllSubjects(c, sy, newSubjects); err != nil {
		log.Errorf(c, "Could not save subjects: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func validSection(section string) bool {
	if len(section) != 1 {
		return false
	}

	section = strings.ToUpper(section)
	r := rune(section[0])
	if r < 'A' || r > 'Z' {
		return false
	}

	return true
}

func sectionsUntil(end string) []string {
	if !validSection(end) {
		return nil
	}

	end = strings.ToUpper(end)
	e := rune(end[0])

	sections := make([]string, 0, e-'A'+1)
	for section := 'A'; section <= e; section++ {
		sections = append(sections, string(section))
	}

	return sections
}

func settingsAccessHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	staffAccess := r.PostForm.Get("staff-access") == "on"

	if err := saveStaffAccess(c, staffAccess); err != nil {
		log.Errorf(c, "Could not save staff access: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	studentAccess := make(map[Term]bool)
	for _, term := range terms {
		access := r.PostForm.Get("student-access-"+term.Value()) == "on"
		studentAccess[term] = access
	}

	if err := saveStudentAccess(c, studentAccess); err != nil {
		log.Errorf(c, "Could not save student access: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/settings", http.StatusFound)
}
