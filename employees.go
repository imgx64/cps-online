// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	"appengine/datastore"
	"bytes"
	"strings"

	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"reflect"
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
	if err != nil {
		return nil, err
	}
	for i, k := range keys {
		e := employees[i]
		e.ID = k.IntID()
		employees[i] = e
	}

	return employees, nil
}

func (emp *employeeType) validate(r *http.Request) error {
	if emp.ID != 0 {
		// make sure employee actually exists
		_, err := getEmployee(r, fmt.Sprint(emp.ID))
		if err != nil {
			return err
		}
	}

	if emp.Name == "" {
		return fmt.Errorf("Name is required")
	}

	if emp.Type == "" {
		return fmt.Errorf("Type is required")
	}

	if emp.Nationality == "" {
		return fmt.Errorf("Nationality is required")
	}

	intCPR, err := strconv.Atoi(emp.CPR)
	if err != nil {
		return fmt.Errorf("Invalid CPR number: %s", emp.CPR)
	}
	emp.CPR = fmt.Sprintf("%09d", intCPR)

	if emp.DateOfHiring == (time.Time{}) {
		return fmt.Errorf("Invalid Date of Hiring")
	}

	if emp.DateOfBirth == (time.Time{}) {
		return fmt.Errorf("Invalid Date of Birth")
	}

	return nil
}

func (emp *employeeType) save(r *http.Request) error {
	c := appengine.NewContext(r)
	err := emp.validate(r)
	if err != nil {
		return err
	}

	var key *datastore.Key
	if emp.ID != 0 {
		key = datastore.NewKey(c, "employee", "", emp.ID, nil)
	} else {
		key = datastore.NewIncompleteKey(c, "employee", nil)
	}
	_, err = datastore.Put(c, key, emp)
	if err != nil {
		return err
	}

	return nil
}

func getEmployeeFromEmail(r *http.Request, email string) (employeeType, error) {
	c := appengine.NewContext(r)
	q := datastore.NewQuery("employee").Filter("CPSEmail =", email).Limit(2)
	var employees []employeeType
	_, err := q.GetAll(c, &employees)
	if err != nil {
		return employeeType{}, err
	}

	if len(employees) == 0 {
		return employeeType{}, fmt.Errorf("Could not find user with email: %s", email)
	} else if len(employees) > 1 {
		return employeeType{}, fmt.Errorf("Users with duplicate emails: %s", email)
	}

	return employees[0], nil
}

func init() {
	http.HandleFunc("/employees", accessHandler(employeesHandler))
	http.HandleFunc("/employees/details", accessHandler(employeesDetailsHandler))
	http.HandleFunc("/employees/save", accessHandler(employeesSaveHandler))
	http.HandleFunc("/employees/import", accessHandler(employeesImportHandler))
	http.HandleFunc("/employees/export", accessHandler(employeesExportHandler))
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
		E     employeeType
		T     []string
		C     []string
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

	emp, err := getEmployee(r, f.Get("ID"))
	if err == datastore.ErrNoSuchEntity {
		// new employee
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

	dateOfHiring, err := time.Parse("2006-01-02", f.Get("DateOfHiring"))
	if err != nil {
		c.Errorf("Invalid Date of Hiring: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	dateOfBirth, err := time.Parse("2006-01-02", f.Get("DateOfBirth"))
	if err != nil {
		c.Errorf("Invalid Date of Birth: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: more checks
	emp.Enabled = f.Get("Enabled") == "on"
	emp.Name = f.Get("Name")
	emp.ArabicName = f.Get("ArabicName")
	emp.Type = f.Get("Type")
	emp.JobDescription = f.Get("JobDescription")
	emp.DateOfHiring = dateOfHiring
	emp.Qualifications = f.Get("Qualifications")
	emp.Nationality = f.Get("Nationality")
	emp.CPR = f.Get("CPR")
	emp.Passport = f.Get("Passport")
	emp.DateOfBirth = dateOfBirth
	emp.MobilePhone = f.Get("MobilePhone")
	emp.Address = f.Get("Address")
	emp.EmergencyPhone = f.Get("EmergencyPhone")
	emp.HealthInfo = f.Get("HealthInfo")
	emp.Comments = f.Get("Comments")

	err = emp.validate(r)
	if err != nil {
		c.Errorf("Could not store employee: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	err = emp.save(r)
	if err != nil {
		c.Errorf("Could not store employee: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/employees", http.StatusFound)

}

// used for CSV
var employeeFields = []string{
	"ID",
	"CPSEmail",
	"Roles.Admin",
	"Roles.HR",
	"Roles.Teacher",
	"Enabled",
	"Name",
	"ArabicName",
	"Type",
	"JobDescription",
	"DateOfHiring",
	"Qualifications",
	"Nationality",
	"CPR",
	"Passport",
	"DateOfBirth",
	"MobilePhone",
	"Address",
	"EmergencyPhone",
	"HealthInfo",
	"Comments",
}

// used for CSV
var employeeFieldsDesc = []string{
	"Can be empty",
	"Set by administrator",
	"Set by administrator",
	"Set by administrator",
	"Set by administrator",
	"True or False",
	"Required",
	"",
	"",
	"",
	"YYYY-MM-DD",
	"",
	"",
	"Required",
	"",
	"YYYY-MM-DD",
	"",
	"",
	"",
	"",
	"",
}

func employeesImportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	var errors []error

	message := struct {
		Msg string
	}{}

	err := r.ParseMultipartForm(1e6)
	if err != nil || r.MultipartForm == nil || len(r.MultipartForm.File["csvfile"]) != 1 {
		// nothing to import
		if err != nil {
			message.Msg = err.Error()
		}
		if err := render(w, r, "employeesimport", message); err != nil {
			c.Errorf("Could not render template employeesimport: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		return
	}

	file, err := r.MultipartForm.File["csvfile"][0].Open()
	if err != nil {
		c.Errorf("Could not open uploaded file: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	defer file.Close()

	u, err := getUser(r)
	if err != nil {
		c.Errorf("Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	isAdmin := u.Roles.Admin

	csvr := csv.NewReader(file)
	csvr.LazyQuotes = true
	csvr.TrailingComma = true
	i := 0
	for {
		i++
		record, err := csvr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}
		if i == 1 {
			// header
			if !reflect.DeepEqual(record, employeeFields) {
				message.Msg = fmt.Sprintf("Invalid file format: %q", record)
				if err := render(w, r, "employeesimport", message); err != nil {
					c.Errorf("Could not render template employeesimport: %s", err)
					renderError(w, r, http.StatusInternalServerError)
					return
				}
				return
			}
			continue
		} else if i == 2 {
			// descriptions
			continue
		}


		var emp employeeType

		var intID int64
		if record[0] == "" {
			intID = 0
		} else {
			emp, err = getEmployee(r, record[0])
			if err != nil {
				errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
				continue
			}
		}

		doh, err := time.Parse("2006-01-02", record[10])
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}

		dob, err := time.Parse("2006-01-02", record[15])
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}

		emp.ID = intID
		if isAdmin {
			emp.CPSEmail = record[1]
			emp.Roles = roles{
				Admin:   strings.EqualFold(record[2], "true"),
				HR:      strings.EqualFold(record[3], "true"),
				Teacher: strings.EqualFold(record[4], "true"),
			}
		}
		emp.Enabled = strings.EqualFold(record[5], "true")
		emp.Name = record[6]
		emp.ArabicName = record[7]
		emp.Type = record[8]
		emp.JobDescription = record[9]
		emp.DateOfHiring = doh
		emp.Qualifications = record[11]
		emp.Nationality = record[12]
		emp.CPR = record[13]
		emp.Passport = record[14]
		emp.DateOfBirth = dob
		emp.MobilePhone = record[16]
		emp.Address = record[17]
		emp.EmergencyPhone = record[18]
		emp.HealthInfo = record[19]
		emp.Comments = record[20]

		err = emp.validate(r)
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}

		err = emp.save(r)
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}
	}

	if len(errors) == 0 {
		// no errors
		http.Redirect(w, r, "/employees", http.StatusFound)
		return
	}

	msg := bytes.NewBuffer([]byte("The following errors were found: "))
	for _, err := range errors {
		fmt.Fprintf(msg, "%s,", err)
	}
	message.Msg = msg.String()
	if err := render(w, r, "employeesimport", message); err != nil {
		c.Errorf("Could not render template employeesimport: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

}

func employeesExportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	var errors []error
	var employees []employeeType
	var filename string

	r.ParseForm()
	if r.Form.Get("template") == "true" {
		filename = "Employees-template"
	} else {
		filename = fmt.Sprintf("Employees-%s", time.Now().Format("2006-01-02"))
		var err error
		employees, err = getEmployees(r, "all")
		if err != nil {
			c.Errorf("Could not retrieve employees: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "text/csv")
	// Force save as with filename
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment;filename=%s.csv", filename))

	csvw := csv.NewWriter(w)
	csvw.UseCRLF = true
	errors = append(errors, csvw.Write(employeeFields))
	errors = append(errors, csvw.Write(employeeFieldsDesc))

	for _, emp := range employees {
		var row []string

		if emp.ID == 0 {
			row = append(row, "")
		} else {
			row = append(row, fmt.Sprint(emp.ID))
		}
		row = append(row, emp.CPSEmail)
		if emp.Roles.Admin {
			row = append(row, "True")
		} else {
			row = append(row, "False")
		}
		if emp.Roles.HR {
			row = append(row, "True")
		} else {
			row = append(row, "False")
		}
		if emp.Roles.Teacher {
			row = append(row, "True")
		} else {
			row = append(row, "False")
		}
		if emp.Enabled {
			row = append(row, "True")
		} else {
			row = append(row, "False")
		}
		row = append(row, emp.Name)
		row = append(row, emp.ArabicName)
		row = append(row, emp.Type)
		row = append(row, emp.JobDescription)
		row = append(row, emp.DateOfHiring.Format("2006-01-02"))
		row = append(row, emp.Qualifications)
		row = append(row, emp.Nationality)
		row = append(row, emp.CPR)
		row = append(row, emp.Passport)
		row = append(row, emp.DateOfBirth.Format("2006-01-02"))
		row = append(row, emp.MobilePhone)
		row = append(row, emp.Address)
		row = append(row, emp.EmergencyPhone)
		row = append(row, emp.HealthInfo)
		row = append(row, emp.Comments)

		errors = append(errors, csvw.Write(row))
	}

	csvw.Flush()

	for _, err := range errors {
		if err != nil {
			c.Errorf("Error writing csv: %s", err)
		}
	}
}
