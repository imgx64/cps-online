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
	"strings"
)

func init() {
	http.HandleFunc("/settings", accessHandler(settingsHandler))
	http.HandleFunc("/settings/save", accessHandler(settingsSaveHandler))
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
		log.Warningf(c, "Could not get school year: %s\nUsing defaults instead", err)
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
		sy = "2014-2015"
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

type maxSection struct {
	Class   string
	Section string
}

type maxSectionSetting struct {
	Value []maxSection
}

func getMaxSections(c context.Context) []maxSection {

	key := datastore.NewKey(c, "settings", "sections", 0, nil)

	setting := maxSectionSetting{}
	if err := nds.Get(c, key, &setting); err != nil {
		log.Warningf(c, "Could not get max sections: %s\n. Returning empty slice instead", err)
		return []maxSection{}
	}

	return setting.Value
}

func saveMaxSections(c context.Context, maxSections []maxSection) error {

	key := datastore.NewKey(c, "settings", "sections", 0, nil)
	_, err := nds.Put(c, key, &maxSectionSetting{maxSections})
	if err != nil {
		return err
	}
	return nil
}

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sectionChoices := sectionsUntil("Z")

	schoolYears := getSchoolYears(c)
	sy := getSchoolYear(c)

	maxSections := getMaxSections(c)

	maxSchoolYear := getMaxSchoolYear(c)
	nextSchoolYear := fmt.Sprintf("%d-%d", maxSchoolYear+1, maxSchoolYear+2)

	data := struct {
		SectionChoices []string

		SchoolYears []string
		SY          string

		MaxSections []maxSection

		NextSchoolYear string
	}{
		sectionChoices,

		schoolYears,
		sy,

		maxSections,

		nextSchoolYear,
	}

	if err := render(w, r, "settings", data); err != nil {
		log.Errorf(c, "Could not render template upload: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func settingsSaveHandler(w http.ResponseWriter, r *http.Request) {
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

	maxSections := getMaxSections(c)
	for i, maxSection := range maxSections {
		section := r.Form.Get("sections-" + maxSection.Class)
		if !validSection(section) {
			continue
		}
		maxSection.Section = section
		maxSections[i] = maxSection
	}

	if err := saveMaxSections(c, maxSections); err != nil {
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
		maxSections := getMaxSections(c)

		for _, maxSection := range maxSections {
			if class == maxSection.Class {
				log.Errorf(c, "Class already exists: %s", class)
				renderErrorMsg(w, r, http.StatusInternalServerError, fmt.Sprintf("Class already exists: %s", class))
				return
			}
		}

		newMaxSection := maxSection{
			Class:   class,
			Section: "A",
		}
		maxSections = append(maxSections, newMaxSection)

		if err := saveMaxSections(c, maxSections); err != nil {
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
