// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	"appengine/datastore"

	"net/http"
	"strconv"
	"time"
)

type employeeType struct {
	ID             int64 `datastore:"-"`
	CPSEmail       string
	Roles          roles
	Enabled        bool
	Name           string
	ArabicName     string
	Type           string
	JobDescription string
	DateOfHiring   time.Time
	Qualifications string
	Nationality    string
	CPR            string
	Passport       string
	DateOfBirth    time.Time
	MobilePhone    string
	Address        string
	EmergencyPhone string
	HealthInfo     string
	Comments       string

	// TODO: payments
}

var employeeTypes = []string{
	"Teacher",
	"Administrative staff",
	"Maintenance and Cleaning",
	"Other",
}

func getEmployee(r *http.Request, id string) (employeeType, error) {
	c := appengine.NewContext(r)
	intID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return employeeType{}, err
	}
	key := datastore.NewKey(c, "employee", "", intID, nil)
	var emp employeeType
	err = datastore.Get(c, key, &emp)
	if err != nil {
		return employeeType{}, err
	}
	emp.ID = intID

	return emp, err
}

func getEmployees(r *http.Request, typ string) ([]employeeType, error) {
	c := appengine.NewContext(r)
	q := datastore.NewQuery("employee")
	if typ != "all" {
		q = q.Filter("Type =", typ)
	}
	q = q.Order("Type")
	var employees []employeeType
	keys, err := q.GetAll(c, &employees)
	for i, k := range keys {
		e := employees[i]
		e.ID = k.IntID()
		employees[i] = e
	}
	if err != nil {
		return nil, err
	}

	return employees, nil
}

func init() {
	http.HandleFunc("/employees", accessHandler(employeesHandler))
	http.HandleFunc("/employees/details", accessHandler(employeesDetailsHandler))
	http.HandleFunc("/employees/save", accessHandler(employeesSaveHandler))
}

func employeesHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	// TODO: filter: type, enabled, etc
	data, err := getEmployees(r, "all")
	if err != nil {
		c.Errorf("Could not retrieve employees: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	if err := render(w, r, "employees", data); err != nil {
		c.Errorf("Could not render template employees: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func employeesDetailsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var emp employeeType
	var err error

	if id := r.Form.Get("id"); id == "new" {
		emp = employeeType{}
		emp.ID = -1
		emp.Enabled = true
		emp.Type = "Teacher"
		emp.DateOfHiring = time.Now()
		emp.Nationality = "Bahrain"
		emp.DateOfBirth = time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local)
	} else {
		emp, err = getEmployee(r, id)
		if err != nil {
			c.Errorf("Could not retrieve employee details: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	u, err := getUser(r)
	if err != nil {
		c.Errorf("Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		E employeeType
		T []string
		C []string
		Admin bool
	}{
		emp,
		employeeTypes,
		countries,
		u.Roles.Admin,
	}

	if err := render(w, r, "employeesdetails", data); err != nil {
		c.Errorf("Could not render template employeedetails: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func employeesSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	f := r.PostForm

	empExists := true
	emp, err := getEmployee(r, f.Get("ID"))
	if err == datastore.ErrNoSuchEntity {
		empExists = false
	} else if err != nil {
		c.Errorf("Could not retrieve employee details: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	u, err := getUser(r)
	if err != nil {
		c.Errorf("Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if u.Roles.Admin {
		emp.CPSEmail = f.Get("CPSEmail")
		emp.Roles = roles{
			Admin:   f.Get("AdminRole") == "on",
			HR:      f.Get("HRRole") == "on",
			Teacher: f.Get("TeacherRole") == "on",
		}
	}

	name := f.Get("Name")
	typ := f.Get("Type")
	dateOfHiring, errHiring := time.Parse("2006-01-02", f.Get("DateOfHiring"))
	nationality := f.Get("Nationality")
	cpr := f.Get("CPR")
	dateOfBirth, errBirth := time.Parse("2006-01-02", f.Get("DateOfBirth"))
	// TODO: more checks
	if name == "" || typ == "" || nationality == "" || cpr == "" ||
		errHiring != nil || errBirth != nil {
		// TODO: message to user
		c.Errorf("Error saving employee: Name: %q, Type: %q, DateOfHiring err: %s, Nationality: %q, CPR: %q, DateOfBirth err: %s",
			name, typ, errHiring, nationality, cpr, errBirth)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	emp.Enabled = f.Get("Enabled") == "on"
	emp.Name = name
	emp.ArabicName = f.Get("ArabicName")
	emp.Type = typ
	emp.JobDescription = f.Get("JobDescription")
	emp.DateOfHiring = dateOfHiring
	emp.Qualifications = f.Get("Qualifications")
	emp.Nationality = nationality
	emp.CPR = cpr
	emp.Passport = f.Get("Passport")
	emp.DateOfBirth = dateOfBirth
	emp.MobilePhone = f.Get("MobilePhone")
	emp.Address = f.Get("Address")
	emp.EmergencyPhone = f.Get("EmergencyPhone")
	emp.HealthInfo = f.Get("HealthInfo")
	emp.Comments = f.Get("Comments")

	// FIXME: update employee if already exists
	var key *datastore.Key
	if empExists {
		key = datastore.NewKey(c, "employee", "", emp.ID, nil)
	} else {
		key = datastore.NewIncompleteKey(c, "employee", nil)
	}
	_, err = datastore.Put(c, key, &emp)
	if err != nil {
		c.Errorf("Could not store employee: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/employees", http.StatusFound)

}
