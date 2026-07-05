// tz centralises the small amount of IANA-timezone maths the calendar needs: converting between an
// absolute instant and the wall-clock time in a named zone, so an event's form shows and stores its times
// in the event's own zone rather than the browser's.

// browserZone returns the user's current IANA zone, used as the default zone for a new event.
export function browserZone(): string {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
}

// FALLBACK_ZONES is used only when the runtime lacks Intl.supportedValuesOf; every modern browser (and
// the WebView2 that hosts the app) returns the full IANA list, so this short list is a safety net.
const FALLBACK_ZONES: string[] = [
    'UTC',
    'Europe/London', 'Europe/Paris', 'Europe/Berlin', 'Europe/Madrid', 'Europe/Athens', 'Europe/Moscow',
    'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles', 'America/Sao_Paulo',
    'Asia/Dubai', 'Asia/Kolkata', 'Asia/Singapore', 'Asia/Shanghai', 'Asia/Tokyo',
    'Australia/Sydney', 'Pacific/Auckland',
]

// allZones returns every IANA zone the runtime knows, or the fallback list when the API is unavailable.
function allZones(): string[] {
    const intl = Intl as unknown as {supportedValuesOf?: (key: string) => string[]}
    if (typeof intl.supportedValuesOf === 'function') {
        try {
            return intl.supportedValuesOf('timeZone')
        } catch {
            return FALLBACK_ZONES
        }
    }
    return FALLBACK_ZONES
}

// zoneOptions returns the full list of zones for the picker, with the browser zone first (then UTC) for
// quick access and the event's current zone folded in, so an unusual imported zone still shows selected.
export function zoneOptions(current: string): string[] {
    return Array.from(new Set([browserZone(), 'UTC', current, ...allZones()].filter(Boolean)))
}

function pad(n: number): string {
    return n < 10 ? '0' + n : String(n)
}

interface CalendarParts {
    year: number
    month: number
    day: number
    hour: number
    minute: number
    second: number
}

// zoneParts renders an instant in a zone into its numeric calendar parts.
function zoneParts(instant: Date, zone: string): CalendarParts {
    const parts = new Intl.DateTimeFormat('en-US', {
        timeZone: zone, hourCycle: 'h23',
        year: 'numeric', month: '2-digit', day: '2-digit',
        hour: '2-digit', minute: '2-digit', second: '2-digit',
    }).formatToParts(instant)
    const get = (t: string) => Number(parts.find((p) => p.type === t)?.value)
    return {year: get('year'), month: get('month'), day: get('day'), hour: get('hour'), minute: get('minute'), second: get('second')}
}

// instantToZonedWall formats an ISO instant as the datetime-local wall value (YYYY-MM-DDTHH:mm) in zone.
export function instantToZonedWall(iso: string, zone: string): string {
    const p = zoneParts(new Date(iso), zone)
    return `${p.year}-${pad(p.month)}-${pad(p.day)}T${pad(p.hour)}:${pad(p.minute)}`
}

// zonedWallToISO interprets a datetime-local wall value (no zone) as a time in zone and returns the
// absolute UTC instant. The zone offset is measured at that wall time, so daylight saving is handled.
export function zonedWallToISO(wall: string, zone: string): string {
    const asUTC = new Date(wall + ':00Z').getTime()
    if (Number.isNaN(asUTC)) return ''
    const p = zoneParts(new Date(asUTC), zone)
    const rendered = Date.UTC(p.year, p.month - 1, p.day, p.hour, p.minute, p.second)
    const offset = rendered - asUTC
    return new Date(asUTC - offset).toISOString()
}
