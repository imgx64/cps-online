// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"

	"fmt"
	htmltemplate "html/template"
	"math"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type templateData struct {
	Username string
	Links    []link
	Data     interface{}
}

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
	data := struct {
		Code        int
		Description string
	}{code, description}

	w.WriteHeader(code)
	if err := render(w, r, "error", data); err != nil {
		c := appengine.NewContext(r)
		c.Errorf("Error occured while handling error: %s", err)
	}
}

func render(w http.ResponseWriter, r *http.Request,
	template string, data interface{}) error {
	c := appengine.NewContext(r)
	baseTemplate := filepath.Join("template", "base.html")
	templateFile := filepath.Join("template", template+".html")
	tmpl, err := htmltemplate.New("base.html").Funcs(funcMap).
		ParseFiles(baseTemplate, templateFile)
	if err != nil {
		return err
	}

	user, err := getUser(c)
	if err != nil {
		c.Errorf("Could not get user: %s", user.Email)
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

	tmplData := templateData{user.Name, links, data}

	if err := tmpl.Execute(w, tmplData); err != nil {
		return err
	}

	return nil
}

var funcMap = htmltemplate.FuncMap{
	"formatDate": func(t time.Time) string {
		return t.Format("2006-01-02")
	},
	"equal": func(item1, item2 interface{}) bool {
		return reflect.DeepEqual(item1, item2)
	},
	"mark": formatMark,
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
}

func formatMark(mark float64) string {
	if math.Signbit(mark) {
		// negative zero
		return ""
	}
	return fmt.Sprintf("%.2f", mark)
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
