package ics

import (
	"time"

	goical "github.com/emersion/go-ical"
)

// This file resolves the time zone named by a property's TZID parameter on ICS import. go-ical interprets
// a TZID by calling time.LoadLocation, which only knows IANA zone names (Europe/London). Outlook and
// Exchange instead emit Windows zone names (GMT Standard Time), which LoadLocation rejects, and go-ical
// then fails the DTSTART parse so the whole event is dropped silently. normalizeZones rewrites each
// Windows name to its IANA equivalent before parsing, and strips a zone it cannot resolve so the value is
// read as floating time rather than dropping the event.

// timeZoneParamProps are the VEVENT properties whose TZID parameter names the zone their value is in.
var timeZoneParamProps = []string{
	goical.PropDateTimeStart,
	goical.PropDateTimeEnd,
	goical.PropRecurrenceID,
	goical.PropRecurrenceDates,
	goical.PropExceptionDates,
}

// normalizeZones rewrites each time-bearing property's TZID parameter so downstream parsing succeeds. A
// Windows zone name is replaced with its IANA equivalent; a name that cannot be resolved to a loadable
// IANA zone has its TZID stripped so the value is read as floating time rather than dropping the event. It
// mutates the parsed component in place, before its times are read.
func normalizeZones(e goical.Event) {
	for _, name := range timeZoneParamProps {
		props := e.Props[name]
		for i := range props {
			tzid := props[i].Params.Get(goical.PropTimezoneID)
			if tzid == "" {
				continue
			}
			if iana, ok := resolveZone(tzid); ok {
				if iana != tzid {
					props[i].Params.Set(goical.PropTimezoneID, iana)
				}
				continue
			}
			delete(props[i].Params, goical.PropTimezoneID)
		}
	}
}

// resolveZone maps a TZID to a loadable IANA zone name. It returns the TZID unchanged when it is already a
// loadable IANA name, its IANA equivalent when it is a known Windows zone name, or ("", false) when it
// cannot be resolved. The mapped name is itself checked against time.LoadLocation, so an entry that names
// a zone the embedded tzdata does not carry degrades to unresolved (floating) rather than a broken TZID.
func resolveZone(tzid string) (string, bool) {
	if tzid == "" {
		return "", false
	}
	if _, err := time.LoadLocation(tzid); err == nil {
		return tzid, true
	}
	if iana, ok := windowsToIANA[tzid]; ok {
		if _, err := time.LoadLocation(iana); err == nil {
			return iana, true
		}
	}
	return "", false
}

// windowsToIANA maps a Windows time zone name (the identifiers Outlook and Exchange write as a TZID) to
// its default IANA equivalent, from the Unicode CLDR windowsZones mapping (territory 001). It is reference
// data, extended as new Windows zones appear; a name absent here degrades to floating on import.
var windowsToIANA = map[string]string{
	"Dateline Standard Time":          "Etc/GMT+12",
	"UTC-11":                          "Etc/GMT+11",
	"Aleutian Standard Time":          "America/Adak",
	"Hawaiian Standard Time":          "Pacific/Honolulu",
	"Marquesas Standard Time":         "Pacific/Marquesas",
	"Alaskan Standard Time":           "America/Anchorage",
	"UTC-09":                          "Etc/GMT+9",
	"Pacific Standard Time (Mexico)":  "America/Tijuana",
	"UTC-08":                          "Etc/GMT+8",
	"Pacific Standard Time":           "America/Los_Angeles",
	"US Mountain Standard Time":       "America/Phoenix",
	"Mountain Standard Time (Mexico)": "America/Mazatlan",
	"Mountain Standard Time":          "America/Denver",
	"Central America Standard Time":   "America/Guatemala",
	"Central Standard Time":           "America/Chicago",
	"Easter Island Standard Time":     "Pacific/Easter",
	"Central Standard Time (Mexico)":  "America/Mexico_City",
	"Canada Central Standard Time":    "America/Regina",
	"SA Pacific Standard Time":        "America/Bogota",
	"Eastern Standard Time (Mexico)":  "America/Cancun",
	"Eastern Standard Time":           "America/New_York",
	"Haiti Standard Time":             "America/Port-au-Prince",
	"Cuba Standard Time":              "America/Havana",
	"US Eastern Standard Time":        "America/Indiana/Indianapolis",
	"Turks And Caicos Standard Time":  "America/Grand_Turk",
	"Paraguay Standard Time":          "America/Asuncion",
	"Atlantic Standard Time":          "America/Halifax",
	"Venezuela Standard Time":         "America/Caracas",
	"Central Brazilian Standard Time": "America/Cuiaba",
	"SA Western Standard Time":        "America/La_Paz",
	"Pacific SA Standard Time":        "America/Santiago",
	"Newfoundland Standard Time":      "America/St_Johns",
	"Tocantins Standard Time":         "America/Araguaina",
	"E. South America Standard Time":  "America/Sao_Paulo",
	"SA Eastern Standard Time":        "America/Cayenne",
	"Argentina Standard Time":         "America/Argentina/Buenos_Aires",
	"Greenland Standard Time":         "America/Godthab",
	"Montevideo Standard Time":        "America/Montevideo",
	"Magallanes Standard Time":        "America/Punta_Arenas",
	"Saint Pierre Standard Time":      "America/Miquelon",
	"Bahia Standard Time":             "America/Bahia",
	"UTC-02":                          "Etc/GMT+2",
	"Azores Standard Time":            "Atlantic/Azores",
	"Cape Verde Standard Time":        "Atlantic/Cape_Verde",
	"UTC":                             "Etc/UTC",
	"GMT Standard Time":               "Europe/London",
	"Greenwich Standard Time":         "Atlantic/Reykjavik",
	"Sao Tome Standard Time":          "Africa/Sao_Tome",
	"Morocco Standard Time":           "Africa/Casablanca",
	"W. Europe Standard Time":         "Europe/Berlin",
	"Central Europe Standard Time":    "Europe/Budapest",
	"Romance Standard Time":           "Europe/Paris",
	"Central European Standard Time":  "Europe/Warsaw",
	"W. Central Africa Standard Time": "Africa/Lagos",
	"Jordan Standard Time":            "Asia/Amman",
	"GTB Standard Time":               "Europe/Bucharest",
	"Middle East Standard Time":       "Asia/Beirut",
	"Egypt Standard Time":             "Africa/Cairo",
	"E. Europe Standard Time":         "Europe/Chisinau",
	"Syria Standard Time":             "Asia/Damascus",
	"West Bank Standard Time":         "Asia/Hebron",
	"South Africa Standard Time":      "Africa/Johannesburg",
	"FLE Standard Time":               "Europe/Kiev",
	"Israel Standard Time":            "Asia/Jerusalem",
	"South Sudan Standard Time":       "Africa/Juba",
	"Kaliningrad Standard Time":       "Europe/Kaliningrad",
	"Sudan Standard Time":             "Africa/Khartoum",
	"Libya Standard Time":             "Africa/Tripoli",
	"Namibia Standard Time":           "Africa/Windhoek",
	"Arabic Standard Time":            "Asia/Baghdad",
	"Turkey Standard Time":            "Europe/Istanbul",
	"Arab Standard Time":              "Asia/Riyadh",
	"Belarus Standard Time":           "Europe/Minsk",
	"Russian Standard Time":           "Europe/Moscow",
	"E. Africa Standard Time":         "Africa/Nairobi",
	"Iran Standard Time":              "Asia/Tehran",
	"Arabian Standard Time":           "Asia/Dubai",
	"Astrakhan Standard Time":         "Europe/Astrakhan",
	"Azerbaijan Standard Time":        "Asia/Baku",
	"Russia Time Zone 3":              "Europe/Samara",
	"Mauritius Standard Time":         "Indian/Mauritius",
	"Saratov Standard Time":           "Europe/Saratov",
	"Georgian Standard Time":          "Asia/Tbilisi",
	"Volgograd Standard Time":         "Europe/Volgograd",
	"Caucasus Standard Time":          "Asia/Yerevan",
	"Afghanistan Standard Time":       "Asia/Kabul",
	"West Asia Standard Time":         "Asia/Tashkent",
	"Ekaterinburg Standard Time":      "Asia/Yekaterinburg",
	"Pakistan Standard Time":          "Asia/Karachi",
	"India Standard Time":             "Asia/Kolkata",
	"Sri Lanka Standard Time":         "Asia/Colombo",
	"Nepal Standard Time":             "Asia/Kathmandu",
	"Central Asia Standard Time":      "Asia/Almaty",
	"Bangladesh Standard Time":        "Asia/Dhaka",
	"Omsk Standard Time":              "Asia/Omsk",
	"Myanmar Standard Time":           "Asia/Yangon",
	"SE Asia Standard Time":           "Asia/Bangkok",
	"Altai Standard Time":             "Asia/Barnaul",
	"W. Mongolia Standard Time":       "Asia/Hovd",
	"North Asia Standard Time":        "Asia/Krasnoyarsk",
	"N. Central Asia Standard Time":   "Asia/Novosibirsk",
	"Tomsk Standard Time":             "Asia/Tomsk",
	"China Standard Time":             "Asia/Shanghai",
	"North Asia East Standard Time":   "Asia/Irkutsk",
	"Singapore Standard Time":         "Asia/Singapore",
	"W. Australia Standard Time":      "Australia/Perth",
	"Taipei Standard Time":            "Asia/Taipei",
	"Ulaanbaatar Standard Time":       "Asia/Ulaanbaatar",
	"Aus Central W. Standard Time":    "Australia/Eucla",
	"Transbaikal Standard Time":       "Asia/Chita",
	"Tokyo Standard Time":             "Asia/Tokyo",
	"North Korea Standard Time":       "Asia/Pyongyang",
	"Korea Standard Time":             "Asia/Seoul",
	"Yakutsk Standard Time":           "Asia/Yakutsk",
	"Cen. Australia Standard Time":    "Australia/Adelaide",
	"AUS Central Standard Time":       "Australia/Darwin",
	"E. Australia Standard Time":      "Australia/Brisbane",
	"AUS Eastern Standard Time":       "Australia/Sydney",
	"West Pacific Standard Time":      "Pacific/Port_Moresby",
	"Tasmania Standard Time":          "Australia/Hobart",
	"Vladivostok Standard Time":       "Asia/Vladivostok",
	"Lord Howe Standard Time":         "Australia/Lord_Howe",
	"Bougainville Standard Time":      "Pacific/Bougainville",
	"Russia Time Zone 10":             "Asia/Srednekolymsk",
	"Magadan Standard Time":           "Asia/Magadan",
	"Norfolk Standard Time":           "Pacific/Norfolk",
	"Sakhalin Standard Time":          "Asia/Sakhalin",
	"Central Pacific Standard Time":   "Pacific/Guadalcanal",
	"Russia Time Zone 11":             "Asia/Kamchatka",
	"New Zealand Standard Time":       "Pacific/Auckland",
	"UTC+12":                          "Etc/GMT-12",
	"Fiji Standard Time":              "Pacific/Fiji",
	"Kamchatka Standard Time":         "Asia/Kamchatka",
	"Chatham Islands Standard Time":   "Pacific/Chatham",
	"UTC+13":                          "Etc/GMT-13",
	"Tonga Standard Time":             "Pacific/Tongatapu",
	"Samoa Standard Time":             "Pacific/Apia",
	"Line Islands Standard Time":      "Pacific/Kiritimati",
}
