// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	"appengine/datastore"

	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

func init() {
	http.HandleFunc("/marks", accessHandler(marksHandler))
	http.HandleFunc("/marks/save", accessHandler(marksSaveHandler))
}

// marksRow will be stored in the datastore
type marksRow struct {
	StudentID string
	Term      string
	Subject   string
	Marks     []float64
}

func getStudentMarks(c appengine.Context, id, subject string) (studentMarks, error) {
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

func storeMarksRow(c appengine.Context, id string, term Term,
	subject string, marks []float64) error {

	mr := marksRow{id, term.Value(), subject, marks}
	keyStr := fmt.Sprintf("%s|%s|%s", id, term, subject)
	key := datastore.NewKey(c, "marks", keyStr, 0, nil)
	_, err := datastore.Put(c, key, &mr)
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

func getStudentRemark(c appengine.Context, id string, term Term) (string, error) {
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

func storeRemark(c appengine.Context, id string, term Term, remark string) error {

	rr := remarksRow{id, term.Value(), remark}
	keyStr := fmt.Sprintf("%s|%s", id, term)
	key := datastore.NewKey(c, "remarks", keyStr, 0, nil)
	_, err := datastore.Put(c, key, &rr)
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
		c.Errorf("Could not parse form: %s", err)
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

	if classHasSubject(class, subject) {
		gs := getGradingSystem(class, subject)
		cols = gs.description(term)
		students, err := getStudents(c, true, classSection)
		if err != nil {
			c.Errorf("Could not store marks: %s", err)
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
			c.Errorf("Could not store marks: %s", err)
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

	data := struct {
		Term    Term
		Class   string
		Section string
		Subject string

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

		Terms:    terms,
		CG:       classGroups,
		Subjects: subjects,

		Cols:     cols,
		Students: studentRows,
	}

	if err := render(w, r, "marks", data); err != nil {
		c.Errorf("Could not render template marks: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func marksSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	f := r.PostForm
	term, err1 := parseTerm(f.Get("Term"))
	subject := f.Get("Subject")
	class, section, err2 := parseClassSection(f.Get("ClassSection"))
	if err1 != nil || subject == "" || err2 != nil {
		c.Errorf("Could not save marks: Term err: %s, subject: %q, classSection err: %s",
			err1, subject, err2)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classSection := fmt.Sprintf("%s|%s", class, section)
	// used for redirecting
	urlValues := url.Values{
		"Term":         []string{term.Value()},
		"ClassSection": []string{classSection},
		"Subject":      []string{subject},
	}
	redirectURL := fmt.Sprintf("/marks?%s", urlValues.Encode())
	// TODO: remove this line once memcache is used
	redirectURL = "/marks"

	nComplete := 0
	if classHasSubject(class, subject) {
		gs := getGradingSystem(class, subject)
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
			c.Errorf("Could not retrieve students: %s", err)
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
				if m[term][i] != v {
					marksChanged = true
					m[term][i] = v
				}

			}
			gs.evaluate(term, m) // TODO: check error
			if marksChanged {
				err := storeMarksRow(c, s.ID, term, subject, m[term])
				if err != nil {
					c.Errorf("Could not store marks: %s", err)
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
			c.Errorf("Could not get students: %s", err)
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
				c.Errorf("Could not store remark: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			nComplete++
		}
	}

	err := storeCompletion(c, classSection, term, subject, nComplete)
	if err != nil {
		c.Errorf("Could not store completion: %s", err)
	}

	// TODO: message of success/fail
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
