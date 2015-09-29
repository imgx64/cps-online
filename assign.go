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
	"strconv"

	"net/http"
)

func init() {
	http.HandleFunc("/assign", accessHandler(assignHandler))
	http.HandleFunc("/assign/save", accessHandler(assignSaveHandler))
}

type assignType struct {
	SY           string
	ClassSection string
	Subject      string
	Teacher      int64
}

func getAllAssignments(c context.Context, sy string) ([]assignType, error) {
	q := datastore.NewQuery("assign")
	q = q.Filter("SY =", sy)
	q = q.Order("ClassSection")
	q = q.Order("Subject")
	var assigns []assignType
	_, err := q.GetAll(c, &assigns)
	if err != nil {
		return nil, err
	}

	return assigns, nil
}

func isTeacherAssigned(c context.Context, sy, classSection, subject string, teacher int64) (bool, error) {
	q := datastore.NewQuery("assign")
	q = q.Filter("SY =", sy)
	q = q.Filter("ClassSection =", classSection)
	q = q.Filter("Subject =", subject)
	q = q.Filter("Teacher =", teacher)
	q = q.KeysOnly().Limit(1)
	count, err := q.Count(c)
	if err != nil {
		return false, err
	}

	return count >= 1, nil
}

func getTeacherAssignments(c context.Context, sy string, teacher int64) ([]assignType, error) {
	q := datastore.NewQuery("assign")
	q = q.Filter("SY =", sy)
	q = q.Filter("Teacher =", teacher)
	q = q.Order("ClassSection")
	q = q.Order("Subject")
	var assigns []assignType
	_, err := q.GetAll(c, &assigns)
	if err != nil {
		return nil, err
	}

	return assigns, nil
}

func (at assignType) save(c context.Context) error {
	q := datastore.NewQuery("assign")
	q = q.Filter("SY =", at.SY)
	q = q.Filter("ClassSection =", at.ClassSection)
	q = q.Filter("Subject =", at.Subject)
	q = q.Filter("Teacher =", at.Teacher)
	q = q.KeysOnly().Limit(1)

	var key *datastore.Key
	keys, err := q.GetAll(c, nil)
	if err == datastore.ErrNoSuchEntity || len(keys) == 0 {
		key = datastore.NewIncompleteKey(c, "assign", nil)
	} else if err != nil {
		return err
	} else {
		key = keys[0]
	}

	_, err = nds.Put(c, key, &at)
	if err != nil {
		return err
	}

	return nil
}

func (at assignType) delete(c context.Context) error {
	q := datastore.NewQuery("assign")
	q = q.Filter("SY =", at.SY)
	q = q.Filter("ClassSection =", at.ClassSection)
	q = q.Filter("Subject =", at.Subject)
	q = q.Filter("Teacher =", at.Teacher)
	q = q.KeysOnly().Limit(1)

	var key *datastore.Key
	keys, err := q.GetAll(c, nil)
	if err == datastore.ErrNoSuchEntity || len(keys) == 0 {
		key = datastore.NewIncompleteKey(c, "assign", nil)
	} else if err != nil {
		return err
	} else {
		key = keys[0]
	}

	err = nds.Delete(c, key)
	if err != nil {
		return err
	}

	return nil
}

func assignHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	assigns, err := getAllAssignments(c, sy)
	if err != nil {
		log.Errorf(c, "Could not get assignments: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	teachers, err := getEmployees(c, true, "Teacher")
	if err != nil {
		log.Errorf(c, "Could not get teachers: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	teachersMap := make(map[int64]string)
	for _, teacher := range teachers {
		teachersMap[teacher.ID] = teacher.Name
	}

	classGroups := getClassGroups(c, sy)

	subjects := getAllSubjects(c, sy)
	subjects = append(subjects, "Behavior", "Remarks")

	data := struct {
		CG       []classGroup
		Subjects []string
		Teachers []employeeType

		TeachersMap map[int64]string

		Assigns []assignType
	}{
		CG:       classGroups,
		Subjects: subjects,
		Teachers: teachers,

		TeachersMap: teachersMap,

		Assigns: assigns,
	}

	if err = render(w, r, "assign", data); err != nil {
		log.Errorf(c, "Could not render template assign: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func assignSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classSection := r.Form.Get("classSection")
	if classSection == "" {
		log.Errorf(c, "No classSection submitted")
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	_, _, err := parseClassSection(classSection)
	if err != nil {
		log.Errorf(c, "Invalid classSection")
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	subject := r.Form.Get("subject")
	if subject == "" {
		log.Errorf(c, "No subject submitted")
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	teacher, err := strconv.ParseInt(r.Form.Get("teacher"), 10, 64)
	if err != nil {
		log.Errorf(c, "Invalid teacher")
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	delet := r.Form.Get("delete") != ""

	assign := assignType{
		SY:           sy,
		ClassSection: classSection,
		Subject:      subject,
		Teacher:      teacher,
	}

	if delet {
		if err := assign.delete(c); err != nil {
			log.Errorf(c, "Could not save assignment: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	} else {
		if err := assign.save(c); err != nil {
			log.Errorf(c, "Could not delete assignment: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	// TODO: message of success
	http.Redirect(w, r, "/assign", http.StatusFound)
}
