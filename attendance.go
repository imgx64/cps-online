// Copyright 2018 Ibrahim Ghazal. All rights reserved.
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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"time"
)

func init() {
	http.HandleFunc("/attendance", accessHandler(attendanceHandler))
	http.HandleFunc("/attendance/save", accessHandler(attendanceSaveHandler))
	http.HandleFunc("/attendance/import", accessHandler(attendanceImportHandler))
	http.HandleFunc("/attendance/export", accessHandler(attendanceExportHandler))
	http.HandleFunc("/attendance/report", accessHandler(attendanceReportHandler))
}

// Attendance will be stored in the datastore
type Attendance struct {
	Date     time.Time // year, month, day
	UserKey  *datastore.Key
	UserName string `datastore:"-"`

	From time.Time // hour, minute
	To   time.Time // hour, minute
}

func getGroupAttendances(c context.Context, date time.Time, group string) ([]Attendance, error) {
	var atts []Attendance

	if group == "employee" {
		employees, err := getEmployees(c, true, "all")
		if err != nil {
			return nil, err
		}
		for _, emp := range employees {
			var att Attendance
			att.UserKey = datastore.NewKey(c, "employee", "", emp.ID, nil)
			att.UserName = emp.Name
			atts = append(atts, att)
		}
	} else {
		return nil, errors.New(fmt.Sprintf("Unknown group: %s", group))
	}

	for i, att := range atts {
		var err error
		atts[i], err = getAttendance(c, date, att.UserKey, att.UserName)
		if err != nil {
			return nil, err
		}
	}

	return atts, nil
}

func getAttendance(c context.Context, date time.Time, userKey *datastore.Key, userName string) (Attendance, error) {
	keyStr := fmt.Sprintf("%s|%s", formatDate(date), userKey.Encode())
	key := datastore.NewKey(c, "attendance", keyStr, 0, nil)
	var attendance Attendance
	if err := nds.Get(c, key, &attendance); err != nil {
		if err == datastore.ErrNoSuchEntity {
			attendance.Date = date
			attendance.UserKey = userKey
		} else {
			return attendance, err
		}
	}
	attendance.UserName = userName
	return attendance, nil
}

func storeAttendance(c context.Context, attendance Attendance) error {
	keyStr := fmt.Sprintf("%s|%s", formatDate(attendance.Date), attendance.UserKey.Encode())
	key := datastore.NewKey(c, "attendance", keyStr, 0, nil)

	_, err := nds.Put(c, key, &attendance)
	return err
}

func attendanceHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	group := r.Form.Get("Group")
	if group == "" {
		group = "employee"
	}

	date, err := parseDate(r.Form.Get("Date"))
	if err != nil {
		log.Errorf(c, "Invalid date: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if date.IsZero() {
		date = time.Now()
	}

	attendances, err := getGroupAttendances(c, date, group)
	if err != nil {
		log.Errorf(c, "Unable to get group attendance: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		Group string
		Date  time.Time

		Attendances []Attendance
	}{
		group,
		date,

		attendances,
	}

	if err := render(w, r, "attendance", data); err != nil {
		log.Errorf(c, "Could not render template attendance: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func attendanceSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	date, err := parseDate(r.PostForm.Get("Date"))
	if err != nil {
		log.Errorf(c, "Invalid date: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	isError := false
	for i := 0; true; i++ {
		keyStr := r.PostForm.Get(fmt.Sprintf("key-%d", i))
		if keyStr == "" {
			break
		}

		key, err1 := datastore.DecodeKey(keyStr)
		from, err2 := parseTime(r.PostForm.Get(fmt.Sprintf("from-%d", i)))
		to, err3 := parseTime(r.PostForm.Get(fmt.Sprintf("to-%d", i)))
		if err1 != nil || err2 != nil || err3 != nil {
			log.Errorf(c, "Invalid attendance: %s %s %s", err1, err2, err3)
			isError = true
			continue
		}

		att := Attendance{
			Date:    date,
			UserKey: key,
			From:    from,
			To:      to,
		}
		log.Debugf(c, "%#v", att)

		err = storeAttendance(c, att)
		if err != nil {
			log.Errorf(c, "Unable to store attendance: %s", err)
			isError = true
			continue
		}
	}

	if isError {
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	urlValues := url.Values{
		"Date":  []string{r.PostForm.Get("Date")},
		"Group": []string{r.PostForm.Get("Group")},
	}
	redirectUrl := fmt.Sprintf("/attendance?%s", urlValues.Encode())
	http.Redirect(w, r, redirectUrl, http.StatusFound)
}

func attendanceImportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	err := r.ParseMultipartForm(1e6)
	if err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if r.MultipartForm == nil || len(r.MultipartForm.File["csvfile"]) != 1 {
		log.Errorf(c, "empty file")
		renderError(w, r, http.StatusInternalServerError)
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
	errorMsg := ""
	i := 0
	for {
		i++
		record, err := csvr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			var msg = fmt.Sprintf("%d: Invalid row: %s", i, err)
			log.Errorf(c, msg)
			errorMsg += msg + ", "
			continue
		}
		if i == 1 {
			// header
			if !reflect.DeepEqual(record, attendanceFields) {
				errorMsg = fmt.Sprintf("Invalid file format: %q", record)
				log.Errorf(c, errorMsg)
				break
			}
			continue
		} else if i == 2 {
			// descriptions
			continue
		}

		var att Attendance
		var err1, err2, err3, err4 error

		att.UserKey, err1 = datastore.DecodeKey(record[0])
		att.Date, err2 = parseDate(record[2])
		att.From, err3 = parseTime(record[3])
		att.To, err4 = parseTime(record[4])
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			var msg = fmt.Sprintf("%d: Invalid row: %s %s %s %s", i,
				err1, err2, err2, err4)
			log.Errorf(c, msg)
			errorMsg += msg + ", "
			continue
		}

		err = storeAttendance(c, att)
		if err != nil {
			var msg = fmt.Sprintf("%d: unable to save: %s", i, err)
			log.Errorf(c, msg)
			errorMsg += msg + ", "
			continue
		}
	}

	if errorMsg != "" {
		renderErrorMsg(w, r, http.StatusInternalServerError, errorMsg)
		return
	}

	http.Redirect(w, r, "/attendance", http.StatusFound)
}

var attendanceFields []string = []string{"", "Name", "Date", "From", "To"}

func attendanceExportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	group := r.Form.Get("Group")
	if group == "" {
		group = "employee"
	}

	date, err := parseDate(r.Form.Get("Date"))
	if err != nil {
		log.Errorf(c, "Invalid date: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if date.IsZero() {
		date = time.Now()
	}

	attendances, err := getGroupAttendances(c, date, group)
	if err != nil {
		log.Errorf(c, "Unable to get group attendance: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("Attendance-%s-%s", group, date)

	w.Header().Set("Content-Type", "text/csv")
	// Force save as with filename
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment;filename=%s.csv", filename))

	var errors []error
	csvw := csv.NewWriter(w)
	csvw.UseCRLF = true

	fieldMax := []string{"Do not modify this column", "", "yyyy-mm-dd", "24-hour format", "24-hour format"}
	errors = append(errors, csvw.Write(attendanceFields))
	errors = append(errors, csvw.Write(fieldMax))

	for _, att := range attendances {
		var row []string
		row = append(row, att.UserKey.Encode())
		row = append(row, att.UserName)
		row = append(row, formatDate(att.Date))
		row = append(row, formatTime(att.From))
		row = append(row, formatTime(att.To))
		errors = append(errors, csvw.Write(row))
	}

	csvw.Flush()

	for _, err := range errors {
		if err != nil {
			log.Errorf(c, "Error writing csv: %s", err)
		}
	}
}

func attendanceReportHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	group := r.Form.Get("Group")

	fromDate, err := parseDate(r.Form.Get("FromDate"))
	if err != nil {
		log.Errorf(c, "Invalid date: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if fromDate.IsZero() {
		fromDate = time.Now()
	}

	toDate, err := parseDate(r.Form.Get("ToDate"))
	if err != nil {
		log.Errorf(c, "Invalid toDate: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if toDate.IsZero() {
		toDate = time.Now()
	}

	duration := toDate.Sub(fromDate)
	if duration < 0 {
		renderErrorMsg(w, r, http.StatusBadRequest, "\"From\" must be before \"To\"")
		return
	}
	day := time.Hour * 24
	numDays := duration / day
	if numDays > 31 {
		renderErrorMsg(w, r, http.StatusBadRequest, "Can't show more than one month at a time")
		return
	}

	var rows [][]string

	// Row for dates
	rows = append(rows, make([]string, 2, 2+numDays))

	if group == "" {
		// Do nothing
	} else if group == "employee" {
		employees, err := getEmployees(c, true, "all")
		if err != nil {
			log.Errorf(c, "Could not get employees: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		for _, emp := range employees {
			row := make([]string, 2, 2+numDays)
			row[0] = datastore.NewKey(c, "employee", "", emp.ID, nil).Encode()
			row[1] = emp.Name
			rows = append(rows, row)
		}
	} else {
		log.Errorf(c, "Unknown group: %s", group)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	for i, row := range rows {
		var key *datastore.Key
		var leaveRequests []leaveRequest
		if i != 0 {
			key, err = datastore.DecodeKey(row[0])
			if err != nil {
				log.Errorf(c, "Could not decode key: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}

			leaveRequests, err = getUserLeaveRequests2(c, key, leaveRequestApproved, fromDate)
			if err != nil {
				log.Errorf(c, "Could not get leave requests: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
		}

		for date := fromDate; date.Before(toDate.Add(1)); date = date.Add(day) {
			if i == 0 {
				row = append(row, formatDateHuman(date))
				rows[i] = row
				continue
			}

			att, err := getAttendance(c, date, key, "")
			if err != nil {
				log.Errorf(c, "Could not get attendance: %s", err)
				renderError(w, r, http.StatusInternalServerError)
				return
			}
			cell := fmt.Sprintf("%s - %s", formatTimeHuman(att.From), formatTimeHuman(att.To))

			for _, leaveRequest := range leaveRequests {
				if leaveRequest.StartDate.Add(-1).Before(date) &&
					date.Before(leaveRequest.EndDate.Add(1)) {

					if !leaveRequest.Time.IsZero() {
						cell += " (" + string(leaveRequest.Type) + " " + formatTimeHuman(leaveRequest.Time) + ")"
					} else {
						cell += " (" + string(leaveRequest.Type) + ")"
					}
					break
				}
			}

			row = append(row, cell)
			rows[i] = row
		}
	}

	if len(rows) == 1 {
		// No data
		rows = nil
	}

	data := struct {
		LeaveTypes []leaveType
		FromDate   time.Time
		ToDate     time.Time
		Group      string

		Rows [][]string
	}{
		leaveTypes,
		fromDate,
		toDate,
		group,

		rows,
	}

	if err := render(w, r, "attendancereport", data); err != nil {
		log.Errorf(c, "Could not render template attendancereport: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
