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
	http.HandleFunc("/settings/addstream", accessHandler(settingsAddStreamHandler))
	http.HandleFunc("/settings/access", accessHandler(settingsAccessHandler))
	http.HandleFunc("/gradinggroups/details", accessHandler(gradingGroupsDetailsHandler))
	http.HandleFunc("/gradinggroups/save", accessHandler(gradingGroupsSaveHandler))
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
	Class            string
	MaxSection       string
	LetterSystem     string
	QuarterWeight    float64
	IgnoreInTotalGPA bool
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

type streamsSettings struct {
	Value []string
}

func getAllStreams(c context.Context, sy string) []string {
	key := datastore.NewKey(c, "settings", "streams-"+sy, 0, nil)

	setting := streamsSettings{}
	if err := nds.Get(c, key, &setting); err != nil {
		log.Warningf(c, "Could not get streams: %s\n. Returning empty slice instead", err)
		return []string{}
	}

	return setting.Value
}

func saveAllStreams(c context.Context, sy string, streams []string) error {
	key := datastore.NewKey(c, "settings", "streams-"+sy, 0, nil)
	_, err := nds.Put(c, key, &streamsSettings{streams})
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

	streams := getAllStreams(c, sy)

	gradingGroups := getGradingGroups(c, sy)

	data := struct {
		SectionChoices      []string
		LetterSystemChoices []string
		Terms               []Term

		StaffAccess   bool
		StudentAccess map[Term]bool

		SchoolYears []string
		SY          string

		ClassSettings []classSetting
		Streams       []string

		Subjects      []string
		GradingGroups []string

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
		streams,

		subjects,
		gradingGroups,

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

		ignoreStr := r.PostForm.Get("ignore-in-total-gpa-" + classSetting.Class)
		classSetting.IgnoreInTotalGPA = ignoreStr == "on"
		settings[i] = classSetting
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

func settingsAddStreamHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	stream := r.PostForm.Get("stream")
	if stream == "" {
		renderErrorMsg(w, r, http.StatusBadRequest, "Empty subject")
		return
	}

	streams := getAllStreams(c, sy)
	for _, s := range streams {
		if s == stream {
			renderErrorMsg(w, r, http.StatusBadRequest, "Stream already exists")
			return
		}
	}

	streams = append(streams, stream)
	if err := saveAllStreams(c, sy, streams); err != nil {
		log.Errorf(c, "Could not save streams: %s", err)
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

type weeksSetting struct {
	Value int
}

func getMaxWeeks(c context.Context) int {

	key := datastore.NewKey(c, "settings", "max_weeks", 0, nil)

	setting := weeksSetting{}
	err := nds.Get(c, key, &setting)
	var maxWeeks int
	if err == nil {
		maxWeeks = setting.Value
	} else {
		maxWeeks = 0
	}

	return maxWeeks
}

func updateMaxWeeks(c context.Context) error {

	s1q := datastore.NewQuery("subjects")
	s1q = s1q.Order("-TotalWeeksS1")
	s1q = s1q.Limit(1)
	var s1 []Subject
	s1MaxWeeks := 0
	if _, err := s1q.GetAll(c, &s1); err == nil {
		if len(s1) > 0 {
			s1MaxWeeks = s1[0].TotalWeeksS1
		}
	} else {
		log.Errorf(c, "Could not get weeks: %s", err)
	}

	s2q := datastore.NewQuery("subjects")
	s2q = s2q.Order("-TotalWeeksS2")
	s2q = s2q.Limit(1)
	var s2 []Subject
	s2MaxWeeks := 0
	if _, err := s2q.GetAll(c, &s2); err == nil {
		if len(s2) > 0 {
			s2MaxWeeks = s2[0].TotalWeeksS2
		}
	} else {
		log.Errorf(c, "Could not get weeks: %s", err)
	}

	maxWeeks := 0
	if s1MaxWeeks > s2MaxWeeks {
		maxWeeks = s1MaxWeeks
	} else {
		maxWeeks = s2MaxWeeks
	}

	key := datastore.NewKey(c, "settings", "max_weeks", 0, nil)

	_, err := nds.Put(c, key, &weeksSetting{maxWeeks})
	if err != nil {
		return err
	}
	return nil
}

type GradingGroupSettings struct {
	Value []string
}

type GradingGroup struct {
	Name    string
	Columns []GradingGroupColumn
}

type GradingGroupColumn struct {
	Name string
	Max  float64
}

func getGradingGroups(c context.Context, sy string) []string {
	key := datastore.NewKey(c, "settings", "grading-groups-"+sy, 0, nil)

	setting := GradingGroupSettings{}
	err := nds.Get(c, key, &setting)
	if err != nil {
		return []string{}
	}

	return setting.Value
}

func saveGradingGroups(c context.Context, sy string, groups []string) error {
	key := datastore.NewKey(c, "settings", "grading-groups-"+sy, 0, nil)

	_, err := nds.Put(c, key, &GradingGroupSettings{groups})
	if err != nil {
		return err
	}
	return nil
}

func getGradingGroup(c context.Context, sy string, name string) (GradingGroup, error) {
	key := datastore.NewKey(c, "grading_groups", name+"-"+sy, 0, nil)

	group := GradingGroup{}
	err := nds.Get(c, key, &group)
	if err != nil {
		return group, err
	}

	return group, nil
}

func saveGradingGroup(c context.Context, sy string, group GradingGroup) error {
	key := datastore.NewKey(c, "grading_groups", group.Name+"-"+sy, 0, nil)

	_, err := nds.Put(c, key, &group)
	if err != nil {
		return err
	}

	gradingGroups := getGradingGroups(c, sy)
	found := false
	for _, existingGroup := range gradingGroups {
		if existingGroup == group.Name {
			found = true
			break
		}
	}
	if !found {
		gradingGroups = append(gradingGroups, group.Name)
		err := saveGradingGroups(c, sy, gradingGroups)
		if err != nil {
			return err
		}
	}

	return nil
}

func deleteGradingGroup(c context.Context, sy string, groupName string) error {
	key := datastore.NewKey(c, "grading_groups", groupName+"-"+sy, 0, nil)

	gradingGroups := getGradingGroups(c, sy)
	for i, existingGroup := range gradingGroups {
		if existingGroup == groupName {
			gradingGroups = append(gradingGroups[:i], gradingGroups[i+1:]...)
			err := saveGradingGroups(c, sy, gradingGroups)
			if err != nil {
				return err
			}
			break
		}
	}

	err := nds.Delete(c, key)
	if err != nil {
		return err
	}
	return nil
}

func gradingGroupsDetailsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	groupName := r.Form.Get("group")

	group := GradingGroup{}
	if groupName != "" {
		var err error
		group, err = getGradingGroup(c, sy, groupName)
		if err != nil {
			log.Errorf(c, "Could not get group %s %s: %s", sy, groupName, err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	if len(group.Columns) < 15 {
		group.Columns = append(group.Columns, make([]GradingGroupColumn, 15-len(group.Columns))...)
	}

	data := struct {
		Group GradingGroup
	}{
		group,
	}

	if err := render(w, r, "gradinggroup", data); err != nil {
		log.Errorf(c, "Could not render template gradinggroup: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func gradingGroupsSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	groupName := r.Form.Get("group")

	if r.PostForm.Get("submit") == "Delete" {

		err := deleteGradingGroup(c, sy, groupName)
		if err != nil {
			log.Errorf(c, "could not delete group %s %s: %s", sy, groupName, err)
			renderErrorMsg(w, r, http.StatusBadRequest, err.Error())
			return
		}

		// TODO: message of success
		http.Redirect(w, r, "/settings", http.StatusFound)
		return
	}

	var group GradingGroup

	group.Name = r.PostForm.Get("GroupName")

	for i := 0; ; i++ {
		_, ok := r.PostForm[fmt.Sprintf("ggc-name-%d", i)]
		if !ok {
			break
		}

		nameStr := r.PostForm.Get(fmt.Sprintf("ggc-name-%d", i))
		maxStr := r.PostForm.Get(fmt.Sprintf("ggc-max-%d", i))

		name := nameStr
		if name == "" {
			continue
		}

		max, err := strconv.ParseFloat(maxStr, 64)
		if err != nil {
			renderErrorMsg(w, r, http.StatusBadRequest,
				fmt.Sprintf("Invalid Encoded Max for %s: %s", name, maxStr))
			return
		}
		if max <= 0 {
			renderErrorMsg(w, r, http.StatusBadRequest,
				fmt.Sprintf("Invalid Encoded Max for %s: %s", name, maxStr))
			return
		}

		column := GradingGroupColumn{
			name,
			max,
		}

		group.Columns = append(group.Columns, column)
	}

	if len(group.Columns) == 0 {
		renderErrorMsg(w, r, http.StatusBadRequest, "Please add columns")
		return
	}

	err := saveGradingGroup(c, sy, group)
	if err != nil {
		log.Errorf(c, "could not save group %s %v: %s", sy, group, err)
		renderErrorMsg(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/settings", http.StatusFound)
}
