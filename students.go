// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	"appengine/datastore"

	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

const studentPrefix = "S"

const schoolDomain = "cps-bh.com"

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

func (stu *studentType) validate() error {
	if stu.ID != "" && !strings.HasPrefix(stu.ID, studentPrefix) {
		return fmt.Errorf("Invalid student ID: %s", stu.ID)
	}

	if stu.Class == "" || stu.Section == "" {
		return fmt.Errorf("Invalid class: %s %s", stu.Class, stu.Section)
	}

	if stu.DateOfBirth == (time.Time{}) {
		return fmt.Errorf("Invalid Date of Birth")
	}

	intCPR, err := strconv.Atoi(stu.CPR)
	if err != nil {
		return fmt.Errorf("Invalid CPR number: %s", stu.CPR)
	}
	stu.CPR = fmt.Sprintf("%09d", intCPR)

	return nil
}

func (stu *studentType) save(r *http.Request) error {
	c := appengine.NewContext(r)
	if err := stu.validate(); err != nil {
		return err
	}

	akey, err := getStudentsAncestor(r)
	if err != nil {
		return fmt.Errorf("Could not get Students Ancestor Key: %s", err)
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
			_, err = datastore.Put(c, datastore.NewKey(c, "student", id, 0, akey), stu)
			if err != nil {
				return err
			}
			return nil
		}, nil) // end transaction
		if err != nil {
			return fmt.Errorf("Could not create student: %s", err)
		}
	} else {
		_, err := datastore.Put(c, datastore.NewKey(c, "student", stu.ID, 0, akey), stu)
		if err != nil {
			return err
		}
	}
	return nil
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

func isStudentEmail(r *http.Request, email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[1] != schoolDomain {
		// invalid email
		return false
	}
	user := parts[0]
	if !strings.HasPrefix(user, studentPrefix) {
		// not a student
		return false
	}
	_, err := getStudent(r, user)
	if err != nil {
		// student does not exist
		return false
	}

	return true
}

func init() {
	http.HandleFunc("/students", accessHandler(studentsHandler))
	http.HandleFunc("/students/details", accessHandler(studentsDetailsHandler))
	http.HandleFunc("/students/save", accessHandler(studentsSaveHandler))
	http.HandleFunc("/students/import", accessHandler(studentsImportHandler))
	http.HandleFunc("/students/export", accessHandler(studentsExportHandler))
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

	err := stu.validate()
	if err != nil {
		// TODO: message to user
		c.Errorf("Invalid student details: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	err = stu.save(r)
	if err != nil {
		// TODO: message to user
		c.Errorf("Could not store student: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/students", http.StatusFound)

}

// used for CSV
var studentFields = []string{
	"StudentID",
	"Enabled",
	"Name",
	"ArabicName",
	"Class",
	"Section",
	"DateOfBirth",
	"Nationality",
	"CPR",
	"Passport",
	"ParentInfo",
	"EmergencyPhone",
	"HealthInfo",
	"Comments",
}

// used for CSV
var studentFieldsDesc = []string{
	"Created Automatically",
	"True or False",
	"Required",
	"",
	"Required",
	"Required",
	"YYYY-MM-DD",
	"",
	"Required",
	"",
	"",
	"",
	"",
	"",
}

func studentsImportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	var errors []error

	message := struct {
		Class string
		Msg   string
	}{}

	err := r.ParseMultipartForm(1e6)
	if err != nil || r.MultipartForm == nil || len(r.MultipartForm.File["csvfile"]) != 1 {
		// nothing to import
		if err != nil {
			message.Class = "alert"
			message.Msg = err.Error()
		}
		if err := render(w, r, "studentsimport", message); err != nil {
			c.Errorf("Could not render template studentsimport: %s", err)
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

	csvr := csv.NewReader(file)
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
			if !reflect.DeepEqual(record, studentFields) {
				message.Class = "alert"
				message.Msg = fmt.Sprintf("Invalid file format: %q", record)
				if err := render(w, r, "studentsimport", message); err != nil {
					c.Errorf("Could not render template studentsimport: %s", err)
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

		dob, err := time.Parse("2006-01-02", record[6])
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}
		stu := studentType{
			ID:             record[0],
			Enabled:        strings.EqualFold(record[1], "true"),
			Name:           record[2],
			ArabicName:     record[3],
			Class:          record[4],
			Section:        record[5],
			DateOfBirth:    dob,
			Nationality:    record[7],
			CPR:            record[8],
			Passport:       record[9],
			ParentInfo:     record[10],
			EmergencyPhone: record[11],
			HealthInfo:     record[12],
			Comments:       record[13],
		}

		err = stu.validate()
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}

		err = stu.save(r)
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}
	}
	// TODO: report errors to user
	http.Redirect(w, r, "/students", http.StatusFound)

}

func studentsExportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	var errors []error
	var students []studentType
	var filename string

	r.ParseForm()
	if r.Form.Get("class") == "empty" {
		// class=empty when downloading template
		filename = "Students-template"
	} else {
		filename = fmt.Sprintf("Students-%s", time.Now().Format("2006-01-02"))
		var err error
		students, err = getStudents(r, "all")
		if err != nil {
			c.Errorf("Could not retrieve students: %s", err)
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
	errors = append(errors, csvw.Write(studentFields))
	errors = append(errors, csvw.Write(studentFieldsDesc))

	for _, stu := range students {
		var row []string
		row = append(row, stu.ID)
		if stu.Enabled {
			row = append(row, "True")
		} else {
			row = append(row, "False")
		}
		row = append(row, stu.Name)
		row = append(row, stu.ArabicName)
		row = append(row, stu.Class)
		row = append(row, stu.Section)
		row = append(row, stu.DateOfBirth.Format("2006-01-02"))
		row = append(row, stu.Nationality)
		row = append(row, stu.CPR)
		row = append(row, stu.Passport)
		row = append(row, stu.ParentInfo)
		row = append(row, stu.EmergencyPhone)
		row = append(row, stu.HealthInfo)
		row = append(row, stu.Comments)

		errors = append(errors, csvw.Write(row))
	}

	csvw.Flush()

	for _, err := range errors {
		if err != nil {
			c.Errorf("Error writing csv: %s", err)
		}
	}
}
