// Copyright 2018 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	_ "github.com/qedus/nds"
	_ "golang.org/x/net/context"
	"google.golang.org/appengine"
	_ "google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

	_ "errors"
	"fmt"
	"net/http"
	_ "net/url"
	_ "strings"
	"time"
)

func init() {
	http.HandleFunc("/leave/allrequests", accessHandler(leaveAllrequestsHandler))
	http.HandleFunc("/leave/myrequests", accessHandler(leaveMyrequestsHandler))
	http.HandleFunc("/leave/request", accessHandler(leaveRequestHandler))
	http.HandleFunc("/leave/request/save", accessHandler(leaveRequestSaveHandler))
}

// LeaveRequest will be stored in the datastore
type LeaveRequest struct {
	ID string `datastore:"-"`

	RequesterId       string
	FromDate          time.Time // year, month, day
	ToDate            time.Time // year, month, day
	Type              leaveRequestType
	Time              time.Time // hour, minute. ED only
	RequesterComments string    `datastore:",noindex"`

	Status        leaveRequestStatus
	AdminComments string `datastore:",noindex"`
}

type leaveRequestType string

const (
	leaveRequestLeaveOfAbsence leaveRequestType = "LoA"
	leaveRequestEarlyDeparture leaveRequestType = "ED"
)

var leaveRequestTypeStrings = map[leaveRequestType]string{
	leaveRequestLeaveOfAbsence: "Leave Of Absence",
	leaveRequestEarlyDeparture: "Early Departure",
}

func (lrt leaveRequestType) String() string {
	s, ok := leaveRequestTypeStrings[lrt]
	if !ok {
		panic(fmt.Sprintf("Invalid leaveRequestType: %d", lrt))
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

func (lrs leaveRequestStatus) String() string {
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

	data := struct {
	}{}

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
