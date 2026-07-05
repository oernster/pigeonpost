package ics

// Embed the IANA time zone database so time.LoadLocation resolves zone names (Europe/London) when
// writing a TZID export, on every host including Windows where there is no system zoneinfo.
import _ "time/tzdata"
