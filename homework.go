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

	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func init() {
	http.HandleFunc("/homework", accessHandler(homeworkHandler))
	http.HandleFunc("/homework/save", accessHandler(homeworkSaveHandler))
	http.HandleFunc("/homework/delete", accessHandler(homeworkDeleteHandler))
	http.HandleFunc("/homeworks", accessHandler(homeworkStudentHandler))
}

// Homework will be stored in the datastore
type Homework struct {
	ID string `datastore:"-"`

	SY      string
	Class   string
	Section string
	Subject string

	Date              time.Time
	Teacher           string
	Homework          string
	HomeworkMultiline []string `datastore:"-"`
}

func getHomework(c context.Context, sy, class, section, subject string) ([]Homework, error) {
	q := datastore.NewQuery("homework")
	q = q.Filter("SY =", sy)
	q = q.Filter("Class =", class)
	q = q.Filter("Section =", section)
	q = q.Filter("Subject =", subject)

	q = q.Order("Date")

	var hws []Homework
	keys, err := q.GetAll(c, &hws)
	if err == datastore.ErrNoSuchEntity {
		return []Homework{}, nil
	} else if err != nil {
		return nil, err
	}

	for i, k := range keys {
		hw := hws[i]
		hw.ID = k.Encode()
		hw.HomeworkMultiline = strings.Split(hw.Homework, "\n")
		hws[i] = hw
	}

	return hws, nil
}

func addHomework(c context.Context, sy, class, section, subject string,
	date time.Time, teacher, homework string) error {

	if sy == "" || class == "" || section == "" || subject == "" || homework == "" {
		return errors.New("Could not save homework")
	}

	hw := Homework{"", sy, class, section, subject, date, teacher, homework, nil}
	key := datastore.NewIncompleteKey(c, "homework", nil)
	_, err := nds.Put(c, key, &hw)
	if err != nil {
		return err
	}

	return nil
}

func deleteHomework(c context.Context, id string) error {
	key, err := datastore.DecodeKey(id)
	if err != nil {
		return err
	}

	err = nds.Delete(c, key)
	if err != nil {
		return err
	}

	return nil
}

func homeworkHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classSection := r.Form.Get("ClassSection")
	class, section, err := parseClassSection(classSection)
	if err != nil {
		class = ""
		section = ""
	}

	subject := r.Form.Get("Subject")

	var hws []Homework

	if subject != "" {
		user, err := getUser(c)
		if err != nil {
			log.Errorf(c, "Could not get user: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		allowAccess := false
		if user.Roles.Admin {
			allowAccess = true
		} else if user.Roles.Teacher {
			emp, err := getEmployeeFromEmail(c, user.Email)
			if err != nil {
				log.Errorf(c, "Could not get employee: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			allowAccess, err = isTeacherAssigned(c, sy, classSection, subject, emp.ID)
			if err != nil {
				log.Errorf(c, "Could not get assignment: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
		} else {
			allowAccess = false
		}

		if !allowAccess {
			renderErrorMsg(w, r, http.StatusForbidden, "You do not have access to this class/subject")
			return
		}

		hws, err = getHomework(c, sy, class, section, subject)
		if err != nil {
			log.Errorf(c, "Could not get homework: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	subjects := getAllSubjects(c, sy)

	classGroups := getClassGroups(c, sy)

	data := struct {
		Class   string
		Section string
		Subject string

		CG       []classGroup
		Subjects []string

		Homeworks []Homework
	}{
		Class:   class,
		Section: section,
		Subject: subject,

		CG:       classGroups,
		Subjects: subjects,

		Homeworks: hws,
	}

	if err := render(w, r, "homework", data); err != nil {
		log.Errorf(c, "Could not render template homework: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func homeworkSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	f := r.PostForm

	subject := f.Get("Subject")

	classSection := f.Get("ClassSection")
	class, section, err := parseClassSection(classSection)
	if err != nil {
		log.Errorf(c, "Invalid classSection: %s", classSection)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	date, err := time.Parse("2006-01-02", f.Get("Date"))
	if err != nil {
		log.Errorf(c, "Invalid homework date: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	homework := f.Get("Homework")
	if homework == "" {
		log.Errorf(c, "Empty homework")
		renderErrorMsg(w, r, http.StatusBadRequest, "Homework is required")
		return
	}

	user, err := getUser(c)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var teacher string
	allowAccess := false
	if user.Roles.Admin {
		allowAccess = true
	} else if user.Roles.Teacher {
		emp, err := getEmployeeFromEmail(c, user.Email)
		if err != nil {
			log.Errorf(c, "Could not get employee: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		allowAccess, err = isTeacherAssigned(c, sy, classSection, subject, emp.ID)
		if err != nil {
			log.Errorf(c, "Could not get assignment: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		teacher = emp.Name
	} else {
		allowAccess = false
	}

	if !allowAccess {
		renderErrorMsg(w, r, http.StatusForbidden, "You do not have access to this class/subject")
		return
	}

	// used for redirecting
	urlValues := url.Values{
		"ClassSection": []string{classSection},
		"Subject":      []string{subject},
	}
	redirectURL := fmt.Sprintf("/homework?%s", urlValues.Encode())

	err = addHomework(c, sy, class, section, subject, date, teacher, homework)
	if err != nil {
		log.Errorf(c, "Could not save homework: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success/fail
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func homeworkDeleteHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	f := r.PostForm

	subject := f.Get("Subject")

	classSection := f.Get("ClassSection")

	homeworkId := f.Get("HomeworkID")
	if homeworkId == "" {
		log.Errorf(c, "Empty homework ID")
		renderErrorMsg(w, r, http.StatusBadRequest, "Homework is required")
		return
	}

	user, err := getUser(c)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	allowAccess := false
	if user.Roles.Admin {
		allowAccess = true
	} else if user.Roles.Teacher {
		emp, err := getEmployeeFromEmail(c, user.Email)
		if err != nil {
			log.Errorf(c, "Could not get employee: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		allowAccess, err = isTeacherAssigned(c, sy, classSection, subject, emp.ID)
		if err != nil {
			log.Errorf(c, "Could not get assignment: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	} else {
		allowAccess = false
	}

	if !allowAccess {
		renderErrorMsg(w, r, http.StatusForbidden, "You do not have access to this class/subject")
		return
	}

	// used for redirecting
	urlValues := url.Values{
		"ClassSection": []string{classSection},
		"Subject":      []string{subject},
	}
	redirectURL := fmt.Sprintf("/homework?%s", urlValues.Encode())

	err = deleteHomework(c, homeworkId)
	if err != nil {
		log.Errorf(c, "Could not save homework: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success/fail
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

type subjectHomework struct {
	Class   string
	Section string
	Subject string

	Homeworks []Homework
}

func homeworkStudentHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	user, err := getUser(c)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if user.Student == nil {
		log.Errorf(c, "User is not a student: %s", user.Email)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	stu := *user.Student

	cs, err := getStudentClass(c, stu.ID, sy)
	if err != nil {
		log.Errorf(c, "Could not get student class: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	class, section := cs.Class, cs.Section

	subjects, err := getSubjects(c, sy, class)
	if err != nil {
		log.Errorf(c, "Could not get subjects: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var subjectHomeworks []subjectHomework

	for _, subject := range subjects {
		if subjectSubject, err := getSubject(c, sy, class, subject); err == nil {
			if !subjectSubject.inStream(cs.Stream) {
				continue
			}
		}

		hws, err := getHomework(c, sy, class, section, subject)
		if err != nil {
			log.Errorf(c, "Could not get homework: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		subjectHomeworks = append(subjectHomeworks, subjectHomework{
			class, section, subject, hws,
		})
	}

	data := struct {
		Homeworks []subjectHomework
	}{
		Homeworks: subjectHomeworks,
	}

	if err := render(w, r, "homeworkstudent", data); err != nil {
		log.Errorf(c, "Could not render template homeworkstudent: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
