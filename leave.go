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
	RequesterComments string    `datastore:",noindex"`

	Status     leaveRequestStatus
	Finished   bool   `datastore:"-"`
	HRComments string `datastore:",noindex"`
}

func getleaveRequest(c context.Context, user user, keyEncoded string) (leaveRequest, error) {
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
			RequesterName:    user.Name,
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
	} else if request.Type == EarlyDeparture {
		request.EndDate = request.StartDate
	}
	request.Finished = request.Status != "" && request.Status != leaveRequestPending

	return request, nil
}

func getRequesterName(c context.Context, requesterKey *datastore.Key) string {
	if requesterKey.Kind() == "employee" {
		var emp employeeType
		if err := nds.Get(c, requesterKey, &emp); err != nil {
			// Ignoring error
			return ""
		}
		return emp.Name

	} else if requesterKey.Kind() == "student" {
		var stu studentType
		if err := nds.Get(c, requesterKey, &stu); err != nil {
			// Ignoring error
			return ""
		}

		class, section, err := getStudentClass(c, stu.ID, getSchoolYear(c))
		if err != nil {
			// Ignoring error
			return stu.Name
		}
		return fmt.Sprintf("%s (%s%s)", stu.Name, class, section)
	}

	return ""
}

type leaveType string

const (
	LeaveOfAbsence leaveType = "LoA"
	EarlyDeparture leaveType = "ED"
)

var leaveTypes = []leaveType{
	LeaveOfAbsence,
	EarlyDeparture,
}

var leaveTypeStrings = map[leaveType]string{
	LeaveOfAbsence: "Leave Of Absence",
	EarlyDeparture: "Early Departure",
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

	data := struct {
	}{}

	if err := render(w, r, "allleaverequests", data); err != nil {
		log.Errorf(c, "Could not render template allrequests: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func leaveMyrequestsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	data := struct {
	}{}

	if err := render(w, r, "myleaverequests", data); err != nil {
		log.Errorf(c, "Could not render template allrequests: %s", err)
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
	request, err := getleaveRequest(c, user, key)
	if err != nil {
		log.Errorf(c, "Could not get leave request: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	isHr, hasPermission := evalleaveRequestPermission(request, user)
	if !hasPermission {
		log.Errorf(c, "User doesn't have permission to view leave request: %s %s", user.Email, request.Key)
		renderErrorMsg(w, r, http.StatusForbidden, "You do not have permission to view this leave request")
		return
	}

	data := struct {
		LeaveTypes []leaveType
		MinDate    time.Time

		Request leaveRequest
		HR           bool
	}{
		leaveTypes,
		time.Now(),

		request,
		isHr,
	}

	if err := render(w, r, "leaverequest", data); err != nil {
		log.Errorf(c, "Could not render template allrequests: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func leaveRequestSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	_ = c
	redirectURL := "/leave/myrequests"
	redirectURL = "/leave/allrequests"

	// TODO: message of success/fail
	http.Redirect(w, r, redirectURL, http.StatusFound)
}

func evalleaveRequestPermission(request leaveRequest, user user) (isHr, hasPermission bool) {
	if request.RequesterKey.Equal(user.Key) {
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
