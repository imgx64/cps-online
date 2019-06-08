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

	"errors"
	"fmt"
	"net/http"
	"time"
)

func init() {
	http.HandleFunc("/leave/allrequests", accessHandler(leaveAllrequestsHandler))
	http.HandleFunc("/leave/myrequests", accessHandler(leaveMyrequestsHandler))
	http.HandleFunc("/leave/request", accessHandler(leaveRequestHandler))
	http.HandleFunc("/leave/request/save", accessHandler(leaveRequestSaveHandler))
}

// leaveRequest will be stored in the datastore
type leaveRequest struct {
	Key *datastore.Key `datastore:"-"`

	RequesterKey      *datastore.Key
	RequesterKeyKind  string    // used in queries
	RequesterName     string    `datastore:"-"`
	StartDate         time.Time // year, month, day
	EndDate           time.Time // year, month, day
	Type              leaveType
	Time              time.Time // hour, minute. ED only
	Term              string
	RequesterComments string `datastore:",noindex"`

	Status     leaveRequestStatus
	HRComments string `datastore:",noindex"`
}

func (lr leaveRequest) Finished() bool {
	return lr.Status != "" && lr.Status != leaveRequestPending
}

func getLeaveRequest(c context.Context, user user, keyEncoded string) (leaveRequest, error) {
	var request leaveRequest

	if keyEncoded == "" {
		var requesterKey *datastore.Key
		if user.Employee != nil {
			requesterKey = user.Employee.Key
		} else if user.Student != nil {
			requesterKey = user.Student.Key
		} else {
			return leaveRequest{}, errors.New("User is not an employee or a student")
		}

		request = leaveRequest{
			RequesterKey:     requesterKey,
			RequesterKeyKind: requesterKey.Kind(),
			RequesterName:    user.FullName(),
			StartDate:        time.Now(),
		}
	} else {
		key, err := datastore.DecodeKey(keyEncoded)
		if err != nil {
			return leaveRequest{}, err
		}

		if err = nds.Get(c, key, &request); err != nil {
			return leaveRequest{}, err
		}

		request.Key = key
		request.RequesterName = getRequesterName(c, request.RequesterKey)
	}

	request.StartDate = dateOnly(request.StartDate)
	request.EndDate = dateOnly(request.EndDate)
	request.Time = timeOnly(request.Time)
	if request.Type == LeaveOfAbsence {
		var zeroTime time.Time
		request.Time = zeroTime
	} else if request.Type == EarlyDeparture || request.Type == LateArrival {
		request.EndDate = request.StartDate
	}

	return request, nil
}

func getUserLeaveRequests(c context.Context, userKey *datastore.Key) ([]leaveRequest, error) {
	var zeroTime time.Time
	return getUserLeaveRequests2(c, userKey, "", zeroTime)
}

func getUserLeaveRequests2(c context.Context, userKey *datastore.Key,
	status leaveRequestStatus, fromDate time.Time) ([]leaveRequest, error) {

	q := datastore.NewQuery("leaverequest")
	q = q.Filter("RequesterKey =", userKey)
	if status != "" {
		q = q.Filter("Status =", status)
	}
	if !fromDate.IsZero() {
		q = q.Filter("EndDate >=", fromDate)
	} else {
		q = q.Order("StartDate")
		q = q.Order("Time")
		q = q.Order("EndDate")
	}
	// Only one inequality filter per query is supported.
	//if !toDate.IsZero() {
	//q = q.Filter("StartDate <=", toDate)
	//}

	var requests []leaveRequest
	keys, err := q.GetAll(c, &requests)
	if err == datastore.ErrNoSuchEntity {
		return []leaveRequest{}, nil
	} else if err != nil {
		return nil, err
	}

	for i, k := range keys {
		request := requests[i]
		request.Key = k
		requests[i] = request
	}

	return requests, nil
}

func searchLeaveRequests(c context.Context, status leaveRequestStatus, requesterKind string) ([]leaveRequest, error) {
	q := datastore.NewQuery("leaverequest")
	if status != "" {
		q = q.Filter("Status =", status)
	}
	if requesterKind != "" {
		q = q.Filter("RequesterKeyKind=", requesterKind)
	}
	q = q.Order("StartDate")
	q = q.Order("Time")
	q = q.Order("EndDate")

	var requests []leaveRequest
	keys, err := q.GetAll(c, &requests)
	if err == datastore.ErrNoSuchEntity {
		return []leaveRequest{}, nil
	} else if err != nil {
		return nil, err
	}

	for i, k := range keys {
		request := requests[i]
		request.Key = k
		request.RequesterName = getRequesterName(c, request.RequesterKey)
		requests[i] = request
	}

	return requests, nil
}

func getRequesterName(c context.Context, requesterKey *datastore.Key) string {
	if requesterKey.Kind() == "employee" {
		var emp employeeType
		if err := nds.Get(c, requesterKey, &emp); err != nil {
			log.Warningf(c, "Could not get employee name: %s", err)
			return ""
		}
		return emp.Name

	} else if requesterKey.Kind() == "student" {
		var stu studentType
		if err := nds.Get(c, requesterKey, &stu); err != nil {
			log.Warningf(c, "Could not get student name: %s", err)
			return ""
		}

		class, section, err := getStudentClass(c, stu.ID, getSchoolYear(c))
		if err != nil {
			log.Warningf(c, "Could not get student class and section: %s", err)
			return stu.Name
		}

		return fmt.Sprintf("%s (%s%s)", stu.Name, class, section)
	}

	log.Warningf(c, "Could not get requester name: %s", requesterKey)
	return ""
}

func saveLeaveRequest(c context.Context, request leaveRequest) error {
	if request.Key == nil {
		request.Key = datastore.NewIncompleteKey(c, "leaverequest", nil)
	}
	_, err := nds.Put(c, request.Key, &request)
	return err
}

type leaveType string

const (
	LeaveOfAbsence leaveType = "LoA"
	EarlyDeparture leaveType = "ED"
	LateArrival    leaveType = "LA"
)

var leaveTypes = []leaveType{
	LeaveOfAbsence,
	EarlyDeparture,
	LateArrival,
}

var leaveTypeStrings = map[leaveType]string{
	LeaveOfAbsence: "Leave Of Absence",
	EarlyDeparture: "Early Departure",
	LateArrival:    "Late Arrival",
}

func (lt leaveType) Value() string {
	return string(lt)
}

func (lt leaveType) String() string {
	if lt == "" {
		return ""
	}
	s, ok := leaveTypeStrings[lt]
	if !ok {
		panic(fmt.Sprintf("Invalid leaveType: %d", lt))
	}
	return s
}

type leaveRequestStatus string

const (
	leaveRequestPending  leaveRequestStatus = "P"
	leaveRequestApproved leaveRequestStatus = "A"
	leaveRequestRejected leaveRequestStatus = "R"
	leaveRequestCanceled leaveRequestStatus = "C"
)

var leaveRequestStatuses = []leaveRequestStatus{
	leaveRequestPending,
	leaveRequestApproved,
	leaveRequestRejected,
	leaveRequestCanceled,
}

var leaveRequestStatusStrings = map[leaveRequestStatus]string{
	leaveRequestPending:  "Pending",
	leaveRequestApproved: "Approved",
	leaveRequestRejected: "Rejected",
	leaveRequestCanceled: "Canceled",
}

func (lrs leaveRequestStatus) Value() string {
	return string(lrs)
}

func (lrs leaveRequestStatus) String() string {
	if lrs == "" {
		return ""
	}
	s, ok := leaveRequestStatusStrings[lrs]
	if !ok {
		panic(fmt.Sprintf("Invalid leaveRequestStatus: %d", lrs))
	}
	return s
}

func leaveAllrequestsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	status := leaveRequestStatus(r.Form.Get("Status"))
	if status == "" {
		status = leaveRequestPending
	} else if status == "all" {
		status = ""
	}

	requester := r.Form.Get("Requester")
	requesterKind := ""
	if requester == "" || requester == "employee" || requester == "student" {
		requesterKind = requester
	}

	requests, err := searchLeaveRequests(c, status, requesterKind)
	if err != nil {
		log.Errorf(c, "Could not get user leave requests: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		Statuses []leaveRequestStatus

		Status        leaveRequestStatus
		RequesterKind string

		Requests []leaveRequest
	}{
		leaveRequestStatuses,

		status,
		requesterKind,

		requests,
	}

	if err := render(w, r, "allleaverequests", data); err != nil {
		log.Errorf(c, "Could not render template allleaverequests: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func leaveMyrequestsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	user, err := getUser(c)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	requests, err := getUserLeaveRequests(c, user.Key())
	if err != nil {
		log.Errorf(c, "Could not get user leave requests: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		Requests []leaveRequest
	}{
		requests,
	}

	if err := render(w, r, "myleaverequests", data); err != nil {
		log.Errorf(c, "Could not render template myleaverequests: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func leaveRequestHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	user, err := getUser(c)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	key := r.Form.Get("key")
	request, err := getLeaveRequest(c, user, key)
	if err != nil {
		log.Errorf(c, "Could not get leave request: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	isHr, hasPermission := evalLeaveRequestPermission(request, user)
	if !hasPermission {
		log.Errorf(c, "User doesn't have permission to view leave request: %s %s", user.Email, request.Key)
		renderErrorMsg(w, r, http.StatusForbidden, "You do not have permission to view this leave request")
		return
	}

	data := struct {
		LeaveTypes []leaveType
		MinDate    time.Time
		Terms      []Term

		Request leaveRequest
		HR      bool
	}{
		leaveTypes,
		time.Now(),
		terms,

		request,
		isHr,
	}

	if err := render(w, r, "leaverequest", data); err != nil {
		log.Errorf(c, "Could not render template leaverequest: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

type leaveSaveAction string

const (
	leaveSaveSave    leaveSaveAction = "Save"
	leaveSaveCancel  leaveSaveAction = "Cancel"
	leaveSaveApprove leaveSaveAction = "Approve"
	leaveSaveReject  leaveSaveAction = "Reject"
	leaveSaveTerm    leaveSaveAction = "Save Term"
)

func leaveRequestSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	user, err := getUser(c)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	key := r.PostForm.Get("Key")
	request, err := getLeaveRequest(c, user, key)
	if err != nil {
		log.Errorf(c, "Could not get leave request: %s %s", key, err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	isHr, hasPermission := evalLeaveRequestPermission(request, user)
	if !hasPermission {
		log.Errorf(c, "User doesn't have permission to view leave request: %s %s", user.Email, request.Key)
		renderErrorMsg(w, r, http.StatusForbidden, "You do not have permission to view this leave request")
		return
	}

	term := r.PostForm.Get("Term")
	if term != "" {
		if _, err := parseTerm(term); err != nil {
			log.Errorf(c, "Invalid term: %s, %s", term, err)
			renderErrorMsg(w, r, http.StatusInternalServerError, "Invalid term")
			return
		}
	}

	action := leaveSaveAction(r.PostForm.Get("submit"))

	if request.Finished() && action != leaveSaveTerm {
		log.Errorf(c, "Can't update finished leaveRequest. Status: %s", request.Status)
		renderErrorMsg(w, r, http.StatusInternalServerError, "Can't update finished leave request")
		return
	}

	if action == leaveSaveSave && !isHr && request.Status == "" {
		// new
		var err1, err2, err3 error
		request.Status = leaveRequestPending
		request.StartDate, err1 = parseDate(r.PostForm.Get("StartDate"))
		request.EndDate, err2 = parseDate(r.PostForm.Get("EndDate"))
		request.Type = leaveType(r.PostForm.Get("Type"))
		request.Time, err3 = parseTime(r.PostForm.Get("Time"))

		request.RequesterComments = r.PostForm.Get("RequesterComments")

		if err1 != nil || err2 != nil || err3 != nil {
			log.Errorf(c, "Invalid leave request: %s %s %s", err1, err2, err3)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		request.StartDate = dateOnly(request.StartDate)
		request.EndDate = dateOnly(request.EndDate)
		request.Time = timeOnly(request.Time)
		if request.Type == LeaveOfAbsence {
			var zeroTime time.Time
			request.Time = zeroTime
			if request.EndDate.IsZero() {
				request.EndDate = request.StartDate
			}
		} else if request.Type == EarlyDeparture || request.Type == LateArrival {
			request.EndDate = request.StartDate
		} else {
			log.Errorf(c, "Invalid leave request. Type: %s", request.Type)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		if request.EndDate.Before(request.StartDate) {
			log.Errorf(c, "Invalid leave request. Dates: %s %s", request.StartDate, request.EndDate)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

	} else if request.Status == leaveRequestPending {
		if action == leaveSaveSave && !isHr {
			// update
			request.RequesterComments = r.PostForm.Get("RequesterComments")
		} else if action == leaveSaveCancel && !isHr {
			request.RequesterComments = r.PostForm.Get("RequesterComments")
			request.Status = leaveRequestCanceled
		} else if action == leaveSaveApprove && isHr {
			request.HRComments = r.PostForm.Get("HRComments")
			request.Status = leaveRequestApproved
			request.Term = term
		} else if action == leaveSaveReject && isHr {
			request.HRComments = r.PostForm.Get("HRComments")
			request.Status = leaveRequestRejected
		} else {
			log.Errorf(c, "Can't update leaveRequest. Invalid status: %s", request.Status)
			renderErrorMsg(w, r, http.StatusInternalServerError, "Can't update leave request")
			return
		}
	} else if action == leaveSaveTerm && isHr {
		request.Term = term
	} else {
		log.Errorf(c, "Can't update leaveRequest. Invalid action/isHr combination: %s %s", action, isHr)
		renderErrorMsg(w, r, http.StatusInternalServerError, "Can't update leave request")
		return
	}

	err = saveLeaveRequest(c, request)
	if err != nil {
		log.Errorf(c, "Could not save leave request: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	var redirectUrl string
	if isHr {
		redirectUrl = "/leave/allrequests"
	} else {
		redirectUrl = "/leave/myrequests"
	}

	// TODO: message of success/fail
	http.Redirect(w, r, redirectUrl, http.StatusFound)
}

func evalLeaveRequestPermission(request leaveRequest, user user) (isHr, hasPermission bool) {
	if request.RequesterKey.Equal(user.Key()) {
		// Handle case where HR is requesting a leave
		// TODO: allow edit for HR
		isHr := user.Roles.HR && request.Status != ""
		return isHr, true
	} else if user.Roles.HR {
		return true, true
	} else {
		return false, false
	}

}
