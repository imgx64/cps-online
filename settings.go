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
}

const startYear = 2013

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

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sectionChoices := sectionsUntil("Z")

	var letterSystemChoices []string
	for key, _ := range letterSystemMap {
		letterSystemChoices = append(letterSystemChoices, key)
	}

	schoolYears := getSchoolYears(c)
	sy := getSchoolYear(c)

	settings := getClassSettings(c, sy)

	maxSchoolYear := getMaxSchoolYear(c)
	nextSchoolYear := fmt.Sprintf("%d-%d", maxSchoolYear+1, maxSchoolYear+2)

	data := struct {
		SectionChoices      []string
		LetterSystemChoices []string

		SchoolYears []string
		SY          string

		ClassSettings []classSetting

		NextSchoolYear string
	}{
		sectionChoices,
		letterSystemChoices,

		schoolYears,
		sy,

		settings,

		nextSchoolYear,
	}

	if err := render(w, r, "settings", data); err != nil {
		log.Errorf(c, "Could not render template upload: %s", err)
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

	sy := r.Form.Get("sy")
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
		section := r.Form.Get("sections-" + classSetting.Class)
		if validSection(section) {
			classSetting.MaxSection = section
			settings[i] = classSetting
		}

		ls := r.Form.Get("letter-system-" + classSetting.Class)
		if _, ok := letterSystemMap[ls]; ok {
			classSetting.LetterSystem = ls
			settings[i] = classSetting
		}

		qwStr := r.Form.Get("quarter-weight-" + classSetting.Class)
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

	if class := r.Form.Get("class"); class != "" {
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
