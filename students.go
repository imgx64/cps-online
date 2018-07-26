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
	ID  string
	Key *datastore.Key `datastore:"-"`

	Name           string
	ArabicName     string
	Gender         string
	DateOfBirth    time.Time
	Nationality    string
	Stream         string
	CPR            string
	Passport       string
	ParentInfo     string
	EmergencyPhone string
	HealthInfo     string
	Comments       string
}

type studentClass struct {
	ID      string
	Name    string
	SY      string
	Class   string
	Section string
}

func getStudent(c context.Context, id string) (studentType, error) {
	akey, err := findStudentsAncestor(c)
	if err != nil {
		return studentType{}, err
	}
	key := datastore.NewKey(c, "student", id, 0, akey)
	var stu studentType
	err = nds.Get(c, key, &stu)
	stu.Key = key
	if err != nil {
		return studentType{}, err
	}

	return stu, nil
}

func getStudentMulti(c context.Context, ids []string) ([]studentType, error) {
	akey, err := findStudentsAncestor(c)
	if err != nil {
		return nil, err
	}
	var keys []*datastore.Key
	for _, id := range ids {
		keys = append(keys, datastore.NewKey(c, "student", id, 0, akey))
	}

	stus := make([]studentType, len(keys))
	err = nds.GetMulti(c, keys, stus)
	if err != nil {
		return nil, err
	}
	for i, stu := range stus {
		stu.Key = keys[i]
		stus[i] = stu
	}

	return stus, nil
}

func findStudents(c context.Context, sy, classSection string) ([]studentClass, error) {
	return findStudentsSorted(c, sy, classSection, false)
}

func findStudentsSorted(c context.Context, sy, classSection string, sorted bool) ([]studentClass, error) {
	if classSection == "|" || classSection == "" {
		return getUnassignedStudents(c, sy)
	}

	q := datastore.NewQuery("studentclass")
	q = q.Filter("SY =", sy)

	var class, section string
	if classSection != "all" {
		cs := strings.Split(classSection, "|")
		if len(cs) != 2 {
			return nil, fmt.Errorf("Invalid class and section: %s", classSection)
		}
		class = cs[0]
		section = cs[1]

		q = q.Filter("Class =", class)
		if section != "" {
			q = q.Filter("Section =", section)
		}

		q = q.Order("Class")
		q = q.Order("Section")
	}

	if sorted {
		q = q.Order("Name")
	} else {
		q = q.Order("ID")
	}

	var students []studentClass
	_, err := q.GetAll(c, &students)
	if err != nil {
		return nil, err
	}

	return students, nil
}

func getUnassignedStudents(c context.Context, sy string) ([]studentClass, error) {
	akey, err := findStudentsAncestor(c)
	if err != nil {
		return nil, err
	}
	q := datastore.NewQuery("student").Ancestor(akey)

	var allStudents []studentType
	_, err = q.GetAll(c, &allStudents)
	if err != nil {
		return nil, err
	}

	assignedStudents, err := findStudentsSorted(c, sy, "all", false)
	if err != nil {
		return nil, err
	}

	assignedStudentIds := make(map[string]bool)
	for _, stu := range assignedStudents {
		assignedStudentIds[stu.ID] = true
	}

	var unassignedStudents []studentClass
	for _, stu := range allStudents {
		if assignedStudentIds[stu.ID] {
			// Student is assigned a class
			continue
		}
		unassignedStudents = append(unassignedStudents, studentClass{
			stu.ID,
			stu.Name,
			sy,
			"",
			"",
		})
	}

	return unassignedStudents, nil
}

func findStudentsCount(c context.Context, sy, classSection string) (int, error) {
	if classSection == "|" || classSection == "" {
		unassignedStudents, err := getUnassignedStudents(c, sy)
		if err != nil {
			return 0, err
		}
		return len(unassignedStudents), nil
	}

	q := datastore.NewQuery("studentclass")
	q = q.Filter("SY =", sy)

	var class, section string
	if classSection != "all" {
		cs := strings.Split(classSection, "|")
		if len(cs) != 2 {
			return 0, fmt.Errorf("Invalid class and section: %s", classSection)
		}
		class = cs[0]
		section = cs[1]

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

	akey, err := findStudentsAncestor(c)
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
			stu.Key = datastore.NewKey(c, "student", id, 0, akey)
			_, err = nds.Put(c, stu.Key, stu)
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

func findStudentsAncestor(c context.Context) (*datastore.Key, error) {
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

func getStudentClass(c context.Context, id, sy string) (class, section string, err error) {
	keyStr := fmt.Sprintf("%s|%s", id, sy)
	key := datastore.NewKey(c, "studentclass", keyStr, 0, nil)

	var sc studentClass
	err = nds.Get(c, key, &sc)
	if err == datastore.ErrNoSuchEntity {
		return "", "", nil
	} else if err != nil {
		return "", "", err
	}

	return sc.Class, sc.Section, nil

}

func saveStudentClass(c context.Context, id, name, sy, class, section string) error {
	if class == "" || section == "" {
		return deleteStudentClass(c, id, sy)
	}

	sections := getClassSections(c, sy)
	classSections, ok := sections[class]
	if !ok {
		return fmt.Errorf("Invalid class and section: %s %s", class, section)
	}

	found := false
	for _, section := range classSections {
		if section == section {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("Invalid class and section: %s %s", class, section)
	}

	keyStr := fmt.Sprintf("%s|%s", id, sy)
	key := datastore.NewKey(c, "studentclass", keyStr, 0, nil)
	_, err := nds.Put(c, key, &studentClass{
		id,
		name,
		sy,
		class,
		section,
	})
	if err != nil {
		return err
	}

	return nil

}

func deleteStudentClass(c context.Context, id, sy string) error {
	keyStr := fmt.Sprintf("%s|%s", id, sy)
	key := datastore.NewKey(c, "studentclass", keyStr, 0, nil)
	err := nds.Delete(c, key)
	if err != nil {
		return err
	}

	return nil
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

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classSection := r.Form.Get("classsection")

	students, err := findStudents(c, sy, classSection)
	if err != nil {
		log.Errorf(c, "Could not retrieve students: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classGroups := getClassGroups(c, sy)

	data := struct {
		S []studentClass

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

	schoolYears := getSchoolYears(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var stu studentType
	var studentClasses map[string]string
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

		studentClasses = make(map[string]string)
		for _, sy := range schoolYears {
			class, section, err := getStudentClass(c, id, sy)
			if err != nil {
				log.Errorf(c, "Could not get student class: %s", err)
				continue
			}
			if class != "" && section != "" {
				studentClasses[sy] = class + "|" + section
			}
		}
	}

	classGroups := make(map[string][]classGroup)
	for _, sy := range schoolYears {
		classGroups[sy] = getClassGroups(c, sy)
	}

	data := struct {
		S  studentType
		SC map[string]string

		SYs []string
		CGs map[string][]classGroup
		C   []string
	}{
		stu,
		studentClasses,

		schoolYears,
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

	dateOfBirth, err := time.Parse("2006-01-02", f.Get("DateOfBirth"))
	if err != nil {
		dateOfBirth = time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local)
	}

	stu := studentType{
		ID:             f.Get("ID"),
		Name:           name,
		ArabicName:     f.Get("ArabicName"),
		Gender:         f.Get("Gender"),
		DateOfBirth:    dateOfBirth,
		Nationality:    f.Get("Nationality"),
		Stream:         f.Get("Stream"),
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

	// Save class assignments
	for _, sy := range getSchoolYears(c) {
		class, section, err := parseClassSection(f.Get("ClassSection-" + sy))
		if err == nil {
			err := saveStudentClass(c, stu.ID, stu.Name, sy, class, section)
			if err != nil {
				log.Errorf(c, "Could not save student class: %s", err)
			}
		} else {
			err := deleteStudentClass(c, stu.ID, sy)
			if err != nil {
				log.Errorf(c, "Could not delete student class: %s", err)
			}
		}
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
	"Stream",
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
	"Required",
	"",
	"M or F",
	"",
	"",
	"YYYY-MM-DD",
	"",
	"",
	"",
	"",
	"",
	"",
	"",
	"",
}

func studentsImportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	var errors []error

	message := struct {
		Msg string
	}{}

	if r.Method != "POST" {
		if err := render(w, r, "studentsimport", message); err != nil {
			log.Errorf(c, "Could not render template studentsimport: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		return
	}

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

		dob, err := time.Parse("2006-01-02", record[6])
		if err != nil {
			errors = append(errors, fmt.Errorf("Error in row %d: %s", i, err))
			continue
		}
		stu := studentType{
			ID:             record[0],
			Name:           record[1],
			ArabicName:     record[2],
			Gender:         record[3],
			DateOfBirth:    dob,
			Nationality:    record[7],
			Stream:         record[8],
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

		class := record[4]
		section := record[5]
		err = saveStudentClass(c, stu.ID, stu.Name, sy, class, section)
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

	sy := getSchoolYear(c)

	var errors []error
	var students []studentType
	var studentClasses []studentClass
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

		studentClasses, err = findStudents(c, sy, classSection)
		if err != nil {
			log.Errorf(c, "Could not retrieve students: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		var studentIds []string
		for _, stu := range studentClasses {
			studentIds = append(studentIds, stu.ID)
		}
		students, err = getStudentMulti(c, studentIds)
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

	for i, stu := range students {
		stuClass := studentClasses[i]
		var row []string
		row = append(row, stu.ID)
		row = append(row, stu.Name)
		row = append(row, stu.ArabicName)
		row = append(row, stu.Gender)
		row = append(row, stuClass.Class)
		row = append(row, stuClass.Section)
		row = append(row, stu.DateOfBirth.Format("2006-01-02"))
		row = append(row, stu.Nationality)
		row = append(row, stu.Stream)
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
