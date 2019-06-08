// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"fmt"
	htmltemplate "html/template"
	"math"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

var errorDescriptions = map[int]string{
	http.StatusNotFound:            "The page you requested was not found.",
	http.StatusInternalServerError: "Internal server Error. Please try again later.",
	http.StatusForbidden:           "You not authorized to access this page.",
}

func renderError(w http.ResponseWriter, r *http.Request, code int) {
	description, ok := errorDescriptions[code]
	if !ok {
		description = "Unknown error."
	}
	renderErrorMsg(w, r, code, description)
}

func renderErrorMsg(w http.ResponseWriter, r *http.Request, code int, message string) {
	data := struct {
		Code        int
		Description string
	}{code, message}

	w.WriteHeader(code)
	if err := render(w, r, "error", data); err != nil {
		c := appengine.NewContext(r)
		log.Errorf(c, "Error occured while handling error: %s", err)
	}
}

func render(w http.ResponseWriter, r *http.Request,
	template string, data interface{}) error {
	c := appengine.NewContext(r)
	sy := getSchoolYear(c)

	baseTemplate := filepath.Join("template", "base.html")
	templateFile := filepath.Join("template", template+".html")
	tmpl, err := htmltemplate.New("base.html").Funcs(funcMap).
		ParseFiles(baseTemplate, templateFile)
	if err != nil {
		return err
	}

	user, err := getUser(c)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", user.Email)
	}

	var links []link
	for _, page := range pages {
		if canAccess(user.Roles, page.URL) {
			if r.URL.Path == page.URL {
				page.Active = true
			}
			links = append(links, page)
		}
	}

	tmplData := struct {
		Username   string
		SchoolYear string
		Links      []link
		Data       interface{}
	}{
		user.Name,
		sy,
		links,
		data,
	}

	if err := tmpl.Execute(w, tmplData); err != nil {
		return err
	}

	return nil
}

var funcMap = htmltemplate.FuncMap{
	"formatDate":      formatDate,
	"formatDateHuman": formatDateHuman,
	"formatTime":      formatTime,
	"formatTimeHuman": formatTimeHuman,
	"equal": func(item1, item2 interface{}) bool {
		return reflect.DeepEqual(item1, item2)
	},
	"mark":      formatMark,
	"markTrim":  formatMarkTrim,
	"markTrim3": formatMarkTrim3,
	"cut": func(s string) string {
		if len(s) < 20 {
			return s
		}
		return s[:20] + "..."
	},
	"hyphens": func(s string) string {
		s = strings.Replace(s, " ", "-", -1)
		s = strings.Replace(s, ".", "-", -1)
		return s
	},
	"increment": func(i int) int {
		return i + 1
	},
	"decrement": func(i int) int {
		return i - 1
	},
	"classSection": func(classSection string) (string, error) {
		class, section, err := parseClassSection(classSection)
		if err != nil {
			return "", err
		}
		return class + section, nil
	},
	"mapInt64Get": func(m map[int64]string, key int64) string {
		return m[key]
	},
	"maxAndWeight": maxAndWeight,
	"last": func(i, len int) bool {
		return i == len-1
	},
	"parseTerm": parseTerm,
}

func formatMark(mark float64) string {
	if math.IsNaN(mark) {
		return ""
	}
	return fmt.Sprintf("%.2f", mark)
}

func formatMarkTrim(mark float64) string {
	if mark == 0 {
		return "0"
	}
	markStr := formatMark(mark)
	markStr = strings.Trim(markStr, "0")
	return strings.TrimRight(markStr, ".")
}

func formatMarkTrim3(mark float64) string {
	if math.IsNaN(mark) {
		return ""
	}
	if mark == 0 {
		return "0"
	}
	markStr := fmt.Sprintf("%.3f", mark)
	markStr = strings.Trim(markStr, "0")
	return strings.TrimRight(markStr, ".")
}

func maxAndWeight(max, weight float64) string {
	maxStr := formatMarkTrim(max)
	if math.IsNaN(weight) {
		return maxStr
	}
	return fmt.Sprintf("%s (%s)", maxStr, formatMarkTrim(weight))
}

func formatDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02")
}

func formatDateHuman(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("02/01/2006")
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("15:04")
}

func formatTimeHuman(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("03:04PM")
}

func parseDate(s string) (time.Time, error) {
	if s == "" {
		var zeroTime time.Time
		return zeroTime, nil
	}
	return time.Parse("2006-01-02", s)
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		var zeroTime time.Time
		return zeroTime, nil
	}
	return time.Parse("15:04", s)
}

func dateOnly(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func timeOnly(t time.Time) time.Time {
	if t.IsZero() {
		return t
	}
	// 1 nsec to make IsZero false
	return time.Date(0, 0, 0, t.Hour(), t.Minute(), 0, 1, t.Location())
}

var countries = []string{
	"Afghanistan",
	"Akrotiri",
	"Albania",
	"Algeria",
	"Andorra",
	"Angola",
	"Anguilla",
	"Antarctica",
	"Argentina",
	"Armenia",
	"Aruba",
	"Australia",
	"Austria",
	"Azerbaijan",
	"Bahamas, The",
	"Bahrain",
	"Bangladesh",
	"Barbados",
	"Belarus",
	"Belgium",
	"Belize",
	"Benin",
	"Bermuda",
	"Bhutan",
	"Bolivia",
	"Bosnia and Herzegovina",
	"Botswana",
	"Bouvet Island",
	"Brazil",
	"Brunei",
	"Bulgaria",
	"Burkina Faso",
	"Burma",
	"Burundi",
	"Cambodia",
	"Cameroon",
	"Canada",
	"Cape Verde",
	"Cayman Islands",
	"Chad",
	"Chile",
	"China",
	"Christmas Island",
	"Clipperton Island",
	"Cocos (Keeling) Islands",
	"Colombia",
	"Comoros",
	"Congo",
	"Cook Islands",
	"Coral Sea Islands",
	"Costa Rica",
	"Cote d'Ivoire",
	"Croatia",
	"Cuba",
	"Cyprus",
	"Czech Republic",
	"Denmark",
	"Dhekelia",
	"Djibouti",
	"Dominica",
	"Dominican Republic",
	"Ecuador",
	"Egypt",
	"El Salvador",
	"Eritrea",
	"Estonia",
	"Ethiopia",
	"Europa Island",
	"Falkland Islands",
	"Faroe Islands",
	"Fiji",
	"Finland",
	"France",
	"Gabon",
	"Gambia, The",
	"Georgia",
	"Germany",
	"Ghana",
	"Gibraltar",
	"Glorioso Islands",
	"Greece",
	"Greenland",
	"Grenada",
	"Guadeloupe",
	"Guam",
	"Guatemala",
	"Guernsey",
	"Guinea",
	"Guinea-Bissau",
	"Guyana",
	"Haiti",
	"Honduras",
	"Hong Kong",
	"Hungary",
	"Iceland",
	"India",
	"Indonesia",
	"Iran",
	"Iraq",
	"Ireland",
	"Isle of Man",
	"Israel",
	"Italy",
	"Jamaica",
	"Jan Mayen",
	"Japan",
	"Jersey",
	"Jordan",
	"Kazakhstan",
	"Kenya",
	"Kiribati",
	"Korea, North",
	"Korea, South",
	"Kuwait",
	"Kyrgyzstan",
	"Laos",
	"Latvia",
	"Lebanon",
	"Lesotho",
	"Liberia",
	"Libya",
	"Liechtenstein",
	"Lithuania",
	"Luxembourg",
	"Macau",
	"Macedonia",
	"Madagascar",
	"Malawi",
	"Malaysia",
	"Maldives",
	"Mali",
	"Malta",
	"Marshall Islands",
	"Martinique",
	"Mauritania",
	"Mauritius",
	"Mayotte",
	"Mexico",
	"Moldova",
	"Monaco",
	"Mongolia",
	"Montserrat",
	"Morocco",
	"Mozambique",
	"Namibia",
	"Nauru",
	"Navassa Island",
	"Nepal",
	"Netherlands",
	"New Caledonia",
	"New Zealand",
	"Nicaragua",
	"Niger",
	"Nigeria",
	"Niue",
	"Norfolk Island",
	"Northern Mariana Islands",
	"Norway",
	"Oman",
	"Pakistan",
	"Palau",
	"Palestine",
	"Panama",
	"Papua New Guinea",
	"Paracel Islands",
	"Paraguay",
	"Peru",
	"Philippines",
	"Pitcairn Islands",
	"Poland",
	"Portugal",
	"Puerto Rico",
	"Qatar",
	"Reunion",
	"Romania",
	"Russia",
	"Rwanda",
	"Samoa",
	"San Marino",
	"Saudi Arabia",
	"Senegal",
	"Serbia and Montenegro",
	"Seychelles",
	"Sierra Leone",
	"Singapore",
	"Slovakia",
	"Slovenia",
	"Solomon Islands",
	"Somalia",
	"South Africa",
	"Spain",
	"Spratly Islands",
	"Sri Lanka",
	"Sudan",
	"Suriname",
	"Svalbard",
	"Swaziland",
	"Sweden",
	"Switzerland",
	"Syria",
	"Taiwan",
	"Tajikistan",
	"Tanzania",
	"Thailand",
	"Timor-Leste",
	"Togo",
	"Tokelau",
	"Tonga",
	"Trinidad and Tobago",
	"Tromelin Island",
	"Tunisia",
	"Turkey",
	"Turkmenistan",
	"Tuvalu",
	"Uganda",
	"Ukraine",
	"United Arab Emirates",
	"United Kingdom",
	"United States",
	"Uruguay",
	"Uzbekistan",
	"Vanuatu",
	"Venezuela",
	"Vietnam",
	"Virgin Islands",
	"Wake Island",
	"Wallis and Futuna",
	"Yemen",
	"Zambia",
	"Zimbabwe",
}
