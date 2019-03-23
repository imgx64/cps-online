// Copyright 2019 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"

	"golang.org/x/net/context"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/mail"
)

func sendStudentEmails(c context.Context, ids []string, subject, body string) {
	var to []string
	for _, id := range ids {
		email := fmt.Sprintf("%s@%s", id, schoolDomain)
		to = append(to, email)
	}
	msg := &mail.Message{
		Sender:  fmt.Sprintf("Creativity Private School <noreply@%s>", schoolDomain),
		To:      to,
		Subject: subject,
		Body:    body,
	}
	if err := mail.Send(c, msg); err != nil {
		log.Errorf(c, "Couldn't send email: %v", err)
	}
}
