package recurrence

// Embed the IANA time zone database so time.LoadLocation resolves zone names (Europe/London) on every
// host, including Windows where there is no system zoneinfo. Registering it here keeps recurring events
// expanding in their own zone regardless of the machine, at the cost of a larger binary.
import _ "time/tzdata"
