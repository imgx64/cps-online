// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	"appengine/datastore"

	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const studentPrefix = "S"

type studentType struct {
	ID             string
	Enabled        bool
	Name           string
	ArabicName     string
	Class          string
	Section        string
	DateOfBirth    time.Time
	Nationality    string
	CPR            string
	Passport       string
	ParentInfo     string
	EmergencyPhone string
	HealthInfo     string
	Comments       string

	// TODO: payments
}

func getStudent(r *http.Request, id string) (studentType, error) {
	c := appengine.NewContext(r)
	akey, err := getStudentsAncestor(r)
	if err != nil {
		return studentType{}, err
	}
	key := datastore.NewKey(c, "student", id, 0, akey)
	var stu studentType
	err = datastore.Get(c, key, &stu)
	if err != nil {
		return studentType{}, err
	}

	return stu, err
}

func getStudents(r *http.Request, classSection string) ([]studentType, error) {
	c := appengine.NewContext(r)
	akey, err := getStudentsAncestor(r)
	if err != nil {
		return nil, err
	}
	q := datastore.NewQuery("student").Ancestor(akey)
	if classSection != "all" {
		cs := strings.Split(classSection, "|")
		if len(cs) != 2 {
			return nil, fmt.Errorf("Invalid class and section: %s", classSection)
		}
		class := cs[0]
		section := cs[1]
		q = q.Filter("Class =", class).
			Filter("Section =", section)
	}
	q = q.Order("Class").Order("Section").Order("ID")
	var students []studentType
	_, err = q.GetAll(c, &students)
	if err != nil {
		return nil, err
	}

	return students, nil
}

func init() {
	http.HandleFunc("/students", accessHandler(studentsHandler))
	http.HandleFunc("/students/details", accessHandler(studentsDetailsHandler))
	http.HandleFunc("/students/save", accessHandler(studentsSaveHandler))
}

func studentsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	// TODO: filter: class, enabled, etc
	data, err := getStudents(r, "all")
	if err != nil {
		c.Errorf("Could not retrieve students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	if err := render(w, r, "students", data); err != nil {
		c.Errorf("Could not render template students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func studentsDetailsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var stu studentType
	var err error

	if id := r.Form.Get("id"); id == "new" {
		stu = studentType{}
		stu.Enabled = true
		stu.Nationality = "Bahrain"
		stu.DateOfBirth = time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local)
		stu.Class = "1"
		stu.Section = "A"
	} else {
		stu, err = getStudent(r, id)
		if err != nil {
			c.Errorf("Could not retrieve student details: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	data := struct {
		S  studentType
		CG []classGroup
		C  []string
	}{
		stu,
		classGroups,
		countries,
	}

	if err := render(w, r, "studentsdetails", data); err != nil {
		c.Errorf("Could not render template studentdetails: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func studentsSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	f := r.PostForm
	name := f.Get("Name")
	class, section, err1 := parseClassSection(f.Get("ClassSection"))
	dateOfBirth, err2 := time.Parse("2006-01-02", f.Get("DateOfBirth"))
	if name == "" || err1 != nil || err2 != nil {
		// TODO: message to user
		c.Errorf("Error saving student: Name: %q, Class err: %s, DateOfBirth err: %s",
			name, err1, err2)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	stu := studentType{
		ID:             f.Get("ID"),
		Enabled:        f.Get("Enabled") == "on",
		Name:           name,
		ArabicName:     f.Get("ArabicName"),
		Class:          class,
		Section:        section,
		DateOfBirth:    dateOfBirth,
		Nationality:    f.Get("Nationality"),
		CPR:            f.Get("CPR"),
		Passport:       f.Get("Passport"),
		ParentInfo:     f.Get("ParentInfo"),
		EmergencyPhone: f.Get("EmergencyPhone"),
		HealthInfo:     f.Get("HealthInfo"),
		Comments:       f.Get("Comments"),
	}

	akey, err := getStudentsAncestor(r)
	if err != nil {
		c.Errorf("Could not get Students Ancestor Key: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if stu.ID == "" {
		err := datastore.RunInTransaction(c, func(c appengine.Context) error {
			q := datastore.NewQuery("student").Ancestor(akey).
				Order("-ID").KeysOnly().Limit(1)
			keys, err := q.GetAll(c, nil)
			if err != nil {
				return err
			}

			var i int64 // Number part of the ID of the new student
			if len(keys) == 0 {
				i = 1000
			} else {
				str := keys[0].StringID()
				if !strings.HasPrefix(str, studentPrefix) {
					return fmt.Errorf("Invalid student key: %s", str)
				}
				str = strings.TrimPrefix(str, studentPrefix)
				i, err = strconv.ParseInt(str, 0, 64)
				if err != nil {
					return err
				}
				i++
			}
			id := fmt.Sprintf("%s%d", studentPrefix, i)
			stu.ID = id
			_, err = datastore.Put(c, datastore.NewKey(c, "student", id, 0, akey), &stu)
			if err != nil {
				return err
			}
			return nil
		}, nil) // end transaction
		if err != nil {
			c.Errorf("Could not create student: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	} else {
		_, err := datastore.Put(c, datastore.NewKey(c, "student", stu.ID, 0, akey), &stu)
		if err != nil {
			c.Errorf("Could not store student: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	// TODO: message of success
	http.Redirect(w, r, "/students", http.StatusFound)

}

func getStudentsAncestor(r *http.Request) (*datastore.Key, error) {
	c := appengine.NewContext(r)
	key := datastore.NewKey(c, "ancestor", "student", 0, nil)
	err := datastore.Get(c, key, &struct{}{})

	if err == datastore.ErrNoSuchEntity {
		datastore.Put(c, key, &struct{}{})
	} else if err != nil {
		return nil, err
	}
	return key, nil
}
