// Copyright 2019 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/mail"
)

func sendStudentEmails(c context.Context, ids []string, subject, body string) {
	var emails []string
	for _, id := range ids {
		email := fmt.Sprintf("%s@%s", id, schoolDomain)
		emails = append(emails, email)
	}
	msg := &mail.Message{
		Sender:  fmt.Sprintf("Creativity Private School <noreply@%s>", schoolDomain),
		Subject: subject,
		Body:    body,
	}

	if len(emails) == 1 {
		msg.To = emails
	} else {
		msg.Bcc = emails
	}

	if err := mail.Send(c, msg); err != nil {
		log.Errorf(c, "Couldn't send email: %v", err)
	}
}

func sendClassEmails(c context.Context, class string, subject, body string) {
	sy := getSchoolYear(c)

	if class == "" || class == "|" {
		class = "all"
	}

	var classSections []string
	if class == "all" || strings.Contains(class, "|") {
		classSections = []string{class}
	} else {
		classSections = getClassSectionsOfClass(c, sy, class)
	}

	var students []studentClass
	for _, classSection := range classSections {
		stus, err := findStudents(c, sy, classSection)
		if err != nil {
			log.Errorf(c, "Could not retrieve students: %s", err)
			continue
		}
		students = append(students, stus...)
	}

	var ids []string
	for _, stu := range students {
		ids = append(ids, stu.ID)
	}

	sendStudentEmails(c, ids, subject, body)
}
