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

	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
)

func init() {
	http.HandleFunc("/marks", accessHandler(marksHandler))
	http.HandleFunc("/marks/save", accessHandler(marksSaveHandler))
	http.HandleFunc("/marks/export", accessHandler(marksExportHandler))
	http.HandleFunc("/marks/import", accessHandler(marksImportHandler))
}

// marksRow will be stored in the datastore
type marksRow struct {
	StudentID string
	Term      string
	Subject   string
	Marks     []float64
}

func getStudentMarks(c context.Context, id, subject string) (studentMarks, error) {
	q := datastore.NewQuery("marks").Filter("StudentID =", id).Filter("Subject =", subject)
	var rows []marksRow
	_, err := q.GetAll(c, &rows)
	if err != nil {
		return nil, err
	}

	marks := make(studentMarks)
	for _, row := range rows {
		termValue, err := parseTerm(row.Term)
		if err != nil {
			return nil, err
		}
		marks[termValue] = row.Marks
	}

	return marks, nil
}

func storeMarksRow(c context.Context, id string, term Term,
	subject string, marks []float64) error {

	mr := marksRow{id, term.Value(), subject, marks}
	keyStr := fmt.Sprintf("%s|%s|%s", id, term, subject)
	key := datastore.NewKey(c, "marks", keyStr, 0, nil)
	_, err := nds.Put(c, key, &mr)
	if err != nil {
		return err
	}

	return nil
}

// remarksRow will be stored in the datastore
type remarksRow struct {
	StudentID string
	Term      string
	Remark    string
}

func getStudentRemark(c context.Context, id string, term Term) (string, error) {
	q := datastore.NewQuery("remarks").Filter("StudentID =", id).
		Filter("Term =", term.Value())
	var remarks []remarksRow
	_, err := q.GetAll(c, &remarks)
	if err != nil {
		return "", err
	}

	if len(remarks) == 0 {
		return "", nil
	}

	return remarks[0].Remark, nil
}

func storeRemark(c context.Context, id string, term Term, remark string) error {

	rr := remarksRow{id, term.Value(), remark}
	keyStr := fmt.Sprintf("%s|%s", id, term)
	key := datastore.NewKey(c, "remarks", keyStr, 0, nil)
	_, err := nds.Put(c, key, &rr)
	if err != nil {
		return err
	}

	return nil
}

type studentRow struct {
	ID     string
	Name   string
	Marks  []float64
	Remark string
}

func marksHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	term, err := parseTerm(r.Form.Get("Term"))
	if err != nil {
		term = Term{}
	}

	classSection := r.Form.Get("ClassSection")
	class, section, err := parseClassSection(classSection)
	if err != nil {
		class = ""
		section = ""
	}

	subject := r.Form.Get("Subject")

	var cols []colDescription
	var studentRows []studentRow

	subjectDisplayName := displayName(subject, class, term)

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
			allowAccess, err = isTeacherAssigned(c, classSection, subject, emp.ID)
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

		sorted := r.Form.Get("sort") == "true"

		if gs := getGradingSystem(c, class, subject); gs != nil {
			cols = gs.description(term)
			students, err := getStudentsSorted(c, true, classSection, sorted)
			if err != nil {
				log.Errorf(c, "Could not get students: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}

			if subjectDisplayName == "" {
				students = []studentType{}
			}

			for _, s := range students {
				m, err := getStudentMarks(c, s.ID, subject)
				if err != nil {
					// TODO: report error
					continue
				}
				gs.evaluate(term, m) // TODO: check error
				studentRows = append(studentRows, studentRow{s.ID, s.Name, m[term], ""})
			}
		} else if subject == "Remarks" {
			cols = []colDescription{{Name: "Remarks"}}
			students, err := getStudents(c, true, classSection)
			if err != nil {
				log.Errorf(c, "Could not get students: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}

			for _, s := range students {
				rem, err := getStudentRemark(c, s.ID, term)
				if err != nil {
					// TODO: report error
					continue
				}
				studentRows = append(studentRows, studentRow{s.ID, s.Name, nil, rem})
			}
		}
	}

	classGroups := getClassGroups(c)

	data := struct {
		Term    Term
		Class   string
		Section string
		Subject string

		SubjectDisplayName string

		Terms    []Term
		CG       []classGroup
		Subjects []string

		Cols     []colDescription
		Students []studentRow
	}{
		Term:    term,
		Class:   class,
		Section: section,
		Subject: subject,

		SubjectDisplayName: subjectDisplayName,

		Terms:    terms,
		CG:       classGroups,
		Subjects: subjects,

		Cols:     cols,
		Students: studentRows,
	}

	if err := render(w, r, "marks", data); err != nil {
		log.Errorf(c, "Could not render template marks: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func marksSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	f := r.PostForm
	term, err1 := parseTerm(f.Get("Term"))
	subject := f.Get("Subject")
	classSection := f.Get("ClassSection")
	class, _, err2 := parseClassSection(classSection)
	if err1 != nil || subject == "" || err2 != nil {
		log.Errorf(c, "Could not save marks: Term err: %s, subject: %q, classSection err: %s",
			err1, subject, err2)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// used for redirecting
	urlValues := url.Values{
		"Term":         []string{term.Value()},
		"ClassSection": []string{classSection},
		"Subject":      []string{subject},
	}
	redirectURL := fmt.Sprintf("/marks?%s", urlValues.Encode())

	nComplete := 0
	if gs := getGradingSystem(c, class, subject); gs != nil {
		cols := gs.description(term)
		hasEditable := false
		for _, col := range cols {
			if col.Editable {
				hasEditable = true
			}
		}
		if !hasEditable {
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		students, err := getStudents(c, true, classSection)
		if err != nil {
			log.Errorf(c, "Could not retrieve students: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		for _, s := range students {
			m, err := getStudentMarks(c, s.ID, subject)
			if err != nil {
				// TODO: report error
				continue
			}
			gs.evaluate(term, m) // TODO: check error

			marksChanged := false
			for i, col := range cols {
				v, err := strconv.ParseFloat(f.Get(fmt.Sprintf("%s|%d", s.ID, i)), 64)
				if err != nil || v > col.Max {
					// invalid or empty marks
					v = negZero
				}
				if m[term][i] != v || math.Signbit(m[term][i]) != math.Signbit(v) {
					marksChanged = true
					m[term][i] = v
				}

			}
			gs.evaluate(term, m) // TODO: check error
			if marksChanged {
				err := storeMarksRow(c, s.ID, term, subject, m[term])
				if err != nil {
					log.Errorf(c, "Could not store marks: %s", err)
					renderError(w, r, http.StatusInternalServerError)
					return
				}
			}
			if gs.ready(term, m) {
				nComplete++
			}
		}
	} else if subject == "Remarks" {
		students, err := getStudents(c, true, classSection)
		if err != nil {
			log.Errorf(c, "Could not get students: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		for _, s := range students {
			remarksName := fmt.Sprintf("%s|0", s.ID)
			remark := f.Get(remarksName)
			if remark == "" {
				// no remark to update
				continue
			}

			err := storeRemark(c, s.ID, term, remark)
			if err != nil {
				log.Errorf(c, "Could not store remark: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			nComplete++
		}
	}

	err := storeCompletion(c, classSection, term, subject, nComplete)
	if err != nil {
		log.Errorf(c, "Could not store completion: %s", err)
	}

	// TODO: message of success/fail
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func marksExportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	term, err := parseTerm(r.Form.Get("Term"))
	if err != nil {
		term = Term{}
	}

	classSection := r.Form.Get("ClassSection")
	class, _, err := parseClassSection(classSection)
	if err != nil {
		class = ""
	}

	subject := r.Form.Get("Subject")

	var cols []colDescription
	var studentRows []studentRow

	if gs := getGradingSystem(c, class, subject); gs != nil {
		cols = gs.description(term)
		students, err := getStudents(c, true, classSection)
		if err != nil {
			log.Errorf(c, "Could not get students: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		for _, s := range students {
			m, err := getStudentMarks(c, s.ID, subject)
			if err != nil {
				// TODO: report error
				continue
			}
			gs.evaluate(term, m) // TODO: check error
			studentRows = append(studentRows, studentRow{s.ID, s.Name, m[term], ""})
		}
	} else if subject == "Remarks" {
		cols = []colDescription{{Name: "Remarks"}}
		students, err := getStudents(c, true, classSection)
		if err != nil {
			log.Errorf(c, "Could not get students: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		for _, s := range students {
			rem, err := getStudentRemark(c, s.ID, term)
			if err != nil {
				// TODO: report error
				continue
			}
			studentRows = append(studentRows, studentRow{s.ID, s.Name, nil, rem})
		}
	}

	filename := fmt.Sprintf("%s-%s-%s", term, classSection, subject)

	w.Header().Set("Content-Type", "text/csv")
	// Force save as with filename
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment;filename=Marks-%s.csv", filename))

	var errors []error
	csvw := csv.NewWriter(w)
	csvw.UseCRLF = true

	fieldNames := []string{filename, "Student Name"}
	fieldMax := []string{"Do not modify this column", ""}
	for _, col := range cols {
		fieldNames = append(fieldNames, col.Name)
		fieldMax = append(fieldMax, formatMark(col.Max))
	}
	errors = append(errors, csvw.Write(fieldNames))
	errors = append(errors, csvw.Write(fieldMax))

	for _, sr := range studentRows {
		var row []string
		row = append(row, sr.ID)
		row = append(row, sr.Name)
		if len(sr.Marks) > 0 {
			for _, mark := range sr.Marks {
				row = append(row, formatMark(mark))
			}
		} else {
			row = append(row, sr.Remark)
		}

		errors = append(errors, csvw.Write(row))
	}

	csvw.Flush()

	for _, err := range errors {
		if err != nil {
			log.Errorf(c, "Error writing csv: %s", err)
		}
	}

}

func marksImportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	err := r.ParseMultipartForm(1e6)
	if err != nil {
		log.Errorf(c, "Could not parse multipart form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	} else if r.MultipartForm == nil || len(r.MultipartForm.File["csvfile"]) != 1 {
		log.Errorf(c, "No file uploaded: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	f := url.Values(r.MultipartForm.Value)
	term, err1 := parseTerm(f.Get("Term"))
	subject := f.Get("Subject")
	classSection := f.Get("ClassSection")
	class, _, err2 := parseClassSection(classSection)
	if err1 != nil || subject == "" || err2 != nil {
		log.Errorf(c, "Could not import marks: Term err: %s, subject: %q, classSection err: %s",
			err1, subject, err2)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// used for redirecting
	urlValues := url.Values{
		"Term":         []string{term.Value()},
		"ClassSection": []string{classSection},
		"Subject":      []string{subject},
	}
	redirectURL := fmt.Sprintf("/marks?%s", urlValues.Encode())

	file, err := r.MultipartForm.File["csvfile"][0].Open()
	if err != nil {
		log.Errorf(c, "Could not open uploaded file: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// used for validating the first two rows
	filename := fmt.Sprintf("%s-%s-%s", term, classSection, subject)
	fieldNames := []string{filename, "Student Name"}
	fieldMax := []string{"Do not modify this column", ""}
	var cols []colDescription
	if gs := getGradingSystem(c, class, subject); gs != nil {
		cols = gs.description(term)
		for _, col := range cols {
			fieldNames = append(fieldNames, col.Name)
			fieldMax = append(fieldMax, formatMark(col.Max))
		}
	} else if subject == "Remarks" {
		fieldNames = append(fieldNames, "Remarks")
		fieldMax = append(fieldMax, formatMark(0))
	}

	csvr := csv.NewReader(file)
	csvr.LazyQuotes = true
	csvr.TrailingComma = true
	csvMarks := make(map[string][]string)
	var errors []error
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
			if !reflect.DeepEqual(record, fieldNames) {
				log.Errorf(c, "Invalid file header: %q. Expected: %q", record, fieldNames)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			continue
		}
		if i == 2 {
			for i, recordStr := range record {
				recordNum, recordErr := strconv.ParseFloat(recordStr, 64)
				expectedStr := fieldMax[i]
				expectedNum, expectedErr := strconv.ParseFloat(expectedStr, 64)
				if recordErr == nil && expectedErr == nil {
					if recordNum != expectedNum {
						log.Errorf(c, "Invalid file header: %q. Expected: %q", record, fieldMax)
						renderError(w, r, http.StatusInternalServerError)
						return
					}
				} else {
					// not numbers
					if recordStr != expectedStr {
						log.Errorf(c, "Invalid file header: %q. Expected: %q", record, fieldMax)
						renderError(w, r, http.StatusInternalServerError)
						return
					}
				}
			}
			continue
		}
		csvMarks[record[0]] = record[1:]
	}

	nComplete := 0
	if gs := getGradingSystem(c, class, subject); gs != nil {
		cols := gs.description(term)
		hasEditable := false
		for _, col := range cols {
			if col.Editable {
				hasEditable = true
			}
		}
		if !hasEditable {
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		students, err := getStudents(c, true, classSection)
		if err != nil {
			log.Errorf(c, "Could not retrieve students: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		for _, s := range students {
			marksRecord, ok := csvMarks[s.ID]
			if !ok {
				log.Errorf(c, "Student not found in class: %s", s.ID)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			delete(csvMarks, s.ID)
			if marksRecord[0] != s.Name {
				log.Errorf(c, "Student ID does not match name in csv: %s, %s", s.ID, marksRecord[0])
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			marksRecord = marksRecord[1:]
			m, err := getStudentMarks(c, s.ID, subject)
			if err != nil {
				log.Errorf(c, "Could not get student marks: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			gs.evaluate(term, m) // TODO: check error

			marksChanged := false
			for i, col := range cols {
				v, err := strconv.ParseFloat(marksRecord[i], 64)
				if err != nil || v > col.Max {
					// invalid or empty marks
					v = negZero
				}
				if m[term][i] != v {
					marksChanged = true
					m[term][i] = v
				}

			}
			gs.evaluate(term, m) // TODO: check error
			if marksChanged {
				err := storeMarksRow(c, s.ID, term, subject, m[term])
				if err != nil {
					log.Errorf(c, "Could not store marks: %s", err)
					renderError(w, r, http.StatusInternalServerError)
					return
				}
			}
			if gs.ready(term, m) {
				nComplete++
			}
		}
	} else if subject == "Remarks" {
		students, err := getStudents(c, true, classSection)
		if err != nil {
			log.Errorf(c, "Could not get students: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		for _, s := range students {
			marksRecord, ok := csvMarks[s.ID]
			if !ok {
				log.Errorf(c, "Student not found in class: %s", s.ID)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			delete(csvMarks, s.ID)
			if marksRecord[0] != s.Name {
				log.Errorf(c, "Student ID does not match name in csv: %s, %s", s.ID, marksRecord[0])
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			marksRecord = marksRecord[1:]
			remark := marksRecord[0]
			if remark == "" {
				// no remark to update
				continue
			}

			err = storeRemark(c, s.ID, term, remark)
			if err != nil {
				log.Errorf(c, "Could not store remark: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			nComplete++
		}
	}

	err = storeCompletion(c, classSection, term, subject, nComplete)
	if err != nil {
		log.Errorf(c, "Could not store completion: %s", err)
	}

	// TODO: message of success/fail
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
