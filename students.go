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

	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

const studentPrefix = "cps"

const schoolDomain = "cps-bh.com"

type studentType struct {
	ID             string
	Name           string
	ArabicName     string
	Gender         string
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
}

func getStudent(c context.Context, id string) (studentType, error) {
	akey, err := getStudentsAncestor(c)
	if err != nil {
		return studentType{}, err
	}
	key := datastore.NewKey(c, "student", id, 0, akey)
	var stu studentType
	err = nds.Get(c, key, &stu)
	if err != nil {
		return studentType{}, err
	}

	return stu, nil
}

func getStudents(c context.Context, classSection string) ([]studentType, error) {
	return getStudentsSorted(c, classSection, false)
}

func getStudentsSorted(c context.Context, classSection string, sorted bool) ([]studentType, error) {
	akey, err := getStudentsAncestor(c)
	if err != nil {
		return nil, err
	}
	q := datastore.NewQuery("student").Ancestor(akey)

	var class, section string
	if classSection != "all" && classSection != "" {
		cs := strings.Split(classSection, "|")
		if len(cs) != 2 {
			return nil, fmt.Errorf("Invalid class and section: %s", classSection)
		}
		class = cs[0]
		section = cs[1]
	}

	if classSection != "all" {
		q = q.Filter("Class =", class)
		q = q.Filter("Section =", section)
	}

	if class != "" {
		q = q.Order("Class")
	}
	if section != "" {
		q = q.Order("Section")
	}

	if sorted {
		q = q.Order("Name")
	} else {
		q = q.Order("ID")
	}

	var students []studentType
	_, err = q.GetAll(c, &students)
	if err != nil {
		return nil, err
	}

	return students, nil
}

func getStudentsCount(c context.Context, classSection string) (int, error) {
	akey, err := getStudentsAncestor(c)
	if err != nil {
		return -1, err
	}
	q := datastore.NewQuery("student").Ancestor(akey)
	if classSection == "" {
		return 0, nil
	}
	if classSection == "all" {
		classSection = "|"
	}
	cs := strings.Split(classSection, "|")
	if len(cs) != 2 {
		return -1, fmt.Errorf("Invalid class and section: %s", classSection)
	}
	class := cs[0]
	section := cs[1]
	if class != "" {
		q = q.Filter("Class =", class)
		if section != "" {
			q = q.Filter("Section =", section)
		}
	}

	n, err := q.Count(c)
	if err != nil {
		return -1, err
	}

	return n, nil
}

func (stu *studentType) validate(c context.Context) error {
	stu.ID = strings.ToLower(stu.ID)
	if stu.ID != "" && !strings.HasPrefix(stu.ID, studentPrefix) {
		return fmt.Errorf("Invalid student ID: %s", stu.ID)
	}

	if stu.Name == "" {
		return fmt.Errorf("Name is required")
	}

	if len(stu.Gender) > 0 {
		switch stu.Gender[0] {
		case 'M', 'm':
			stu.Gender = "M"
		case 'F', 'f':
			stu.Gender = "F"
		default:
			stu.Gender = ""
		}
	}

	if stu.Class != "" {
		// FIXME
		sections := getClassSections(c, getSchoolYear(c))
		classSections, ok := sections[stu.Class]
		if !ok {
			return fmt.Errorf("Invalid class and section: %s %s", stu.Class, stu.Section)
		}

		found := false
		for _, section := range classSections {
			if section == stu.Section {
				found = true
				break
			}
		}

		if !found {
			return fmt.Errorf("Invalid class and section: %s %s", stu.Class, stu.Section)
		}

	}

	intCPR, err := strconv.Atoi(stu.CPR)
	if err == nil {
		stu.CPR = fmt.Sprintf("%09d", intCPR)
	}

	return nil
}

func (stu *studentType) save(c context.Context) error {
	if err := stu.validate(c); err != nil {
		return err
	}

	akey, err := getStudentsAncestor(c)
	if err != nil {
		return fmt.Errorf("Could not get Students Ancestor Key: %s", err)
	}

	if stu.ID == "" {
		err := nds.RunInTransaction(c, func(c context.Context) error {
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
			_, err = nds.Put(c, datastore.NewKey(c, "student", id, 0, akey), stu)
			if err != nil {
				return err
			}
			return nil
		}, nil) // end transaction
		if err != nil {
			return fmt.Errorf("Could not create student: %s", err)
		}
	} else {
		_, err := nds.Put(c, datastore.NewKey(c, "student", stu.ID, 0, akey), stu)
		if err != nil {
			return err
		}
	}
	return nil
}

var studentsAncestorLock sync.Mutex
var studentsAncestor *datastore.Key

func getStudentsAncestor(c context.Context) (*datastore.Key, error) {
	studentsAncestorLock.Lock()
	defer studentsAncestorLock.Unlock()

	if studentsAncestor != nil {
		return studentsAncestor, nil
	}

	key := datastore.NewKey(c, "ancestor", "student", 0, nil)
	err := nds.Get(c, key, &struct{}{})

	if err == datastore.ErrNoSuchEntity {
		nds.Put(c, key, &struct{}{})
	} else if err != nil {
		return nil, err
	}
	studentsAncestor = key
	return key, nil
}

func getStudentFromEmail(c context.Context, email string) (studentType, error) {
	email = strings.ToLower(email)
	parts := strings.Split(email, "@")
	if len(parts) != 2 || parts[1] != schoolDomain {
		return studentType{}, fmt.Errorf("Invalid email: %s", email)
	}
	user := parts[0]
	if !strings.HasPrefix(user, studentPrefix) {
		return studentType{}, fmt.Errorf("Not a student")
	}
	stu, err := getStudent(c, user)
	if err != nil {
		return studentType{}, err
	}

	return stu, nil
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
	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classSection := r.Form.Get("classsection")

	students, err := getStudents(c, classSection)
	if err != nil {
		log.Errorf(c, "Could not retrieve students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// FIXME
	classGroups := getClassGroups(c, getSchoolYear(c))

	data := struct {
		S []studentType

		CG []classGroup

		ClassSection string
	}{
		students,

		classGroups,

		classSection,
	}

	if err := render(w, r, "students", data); err != nil {
		log.Errorf(c, "Could not render template students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func studentsDetailsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var stu studentType
	var err error

	if id := r.Form.Get("id"); id == "new" {
		stu = studentType{}
		stu.Nationality = "Bahrain"
		stu.DateOfBirth = time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local)
	} else {
		stu, err = getStudent(c, id)
		if err != nil {
			log.Errorf(c, "Could not retrieve student details: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	// FIXME
	classGroups := getClassGroups(c, getSchoolYear(c))

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
		log.Errorf(c, "Could not render template studentdetails: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func studentsSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	f := r.PostForm
	name := f.Get("Name")
	if name == "" {
		// TODO: message to user
		log.Errorf(c, "Error saving student: Name: %q", name)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	class, section, err := parseClassSection(f.Get("ClassSection"))
	if err != nil {
		class, section = "", ""
	}

	dateOfBirth, err := time.Parse("2006-01-02", f.Get("DateOfBirth"))
	if err != nil {
		dateOfBirth = time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local)
	}

	stu := studentType{
		ID:             f.Get("ID"),
		Name:           name,
		ArabicName:     f.Get("ArabicName"),
		Gender:         f.Get("Gender"),
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

	err = stu.validate(c)
	if err != nil {
		log.Errorf(c, "Invalid student details: %s", err)
		renderErrorMsg(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	err = stu.save(c)
	if err != nil {
		// TODO: message to user
		log.Errorf(c, "Could not store student: %s", err)
		renderErrorMsg(w, r, http.StatusInternalServerError, err.Error())
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/students", http.StatusFound)

}

// used for CSV
var studentFields = []string{
	"StudentID",
	"Name",
	"ArabicName",
	"Gender",
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
	"M or F",
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
		Msg string
	}{}

	err := r.ParseMultipartForm(1e6)
	if err != nil || r.MultipartForm == nil || len(r.MultipartForm.File["csvfile"]) != 1 {
		// nothing to import
		if err != nil {
			message.Msg = err.Error()
		}
		if err := render(w, r, "studentsimport", message); err != nil {
			log.Errorf(c, "Could not render template studentsimport: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		return
	}

	file, err := r.MultipartForm.File["csvfile"][0].Open()
	if err != nil {
		log.Errorf(c, "Could not open uploaded file: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	defer file.Close()

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
			if !reflect.DeepEqual(record, studentFields) {
				message.Msg = fmt.Sprintf("Invalid file format: %q", record)
				if err := render(w, r, "studentsimport", message); err != nil {
					log.Errorf(c, "Could not render template studentsimport: %s", err)
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

		dob, err := time.Parse("2006-01-02", record[7])
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}
		stu := studentType{
			ID:             record[0],
			Name:           record[2],
			ArabicName:     record[3],
			Gender:         record[4],
			Class:          record[5],
			Section:        record[6],
			DateOfBirth:    dob,
			Nationality:    record[8],
			CPR:            record[9],
			Passport:       record[10],
			ParentInfo:     record[11],
			EmergencyPhone: record[12],
			HealthInfo:     record[13],
			Comments:       record[14],
		}

		err = stu.validate(c)
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}

		err = stu.save(c)
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}
	}

	if len(errors) == 0 {
		// no errors
		http.Redirect(w, r, "/students", http.StatusFound)
		return
	}

	msg := bytes.NewBufferString("The following errors were found: ")
	for _, err := range errors {
		fmt.Fprintf(msg, "%s,", err)
	}
	message.Msg = msg.String()
	if err := render(w, r, "studentsimport", message); err != nil {
		log.Errorf(c, "Could not render template studentsimport: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

}

func studentsExportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	var errors []error
	var students []studentType
	var filename string

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	if r.Form.Get("template") == "true" {
		filename = "Students-template"
	} else {
		filename = fmt.Sprintf("Students-%s", time.Now().Format("2006-01-02"))
		var err error
		classSection := r.Form.Get("classsection")

		students, err = getStudents(c, classSection)
		if err != nil {
			log.Errorf(c, "Could not retrieve students: %s", err)
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
		row = append(row, stu.Name)
		row = append(row, stu.ArabicName)
		row = append(row, stu.Gender)
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
			log.Errorf(c, "Error writing csv: %s", err)
		}
	}
}
