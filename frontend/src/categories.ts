// categories is the pure catalogue of event categories: the short lowercase value stored on an event
// (the primary iCalendar CATEGORIES value), a human label for the picker and an emoji shown beside the
// event in the calendar. It carries no React or Wails coupling so it is unit-testable in isolation.

// CategoryOption is one selectable event category: the stored value, its picker label and its emoji.
export interface CategoryOption {
    value: string
    label: string
    emoji: string
}

// EVENT_CATEGORIES is the offered set, in picker order. The value is what persists on the event; the
// label and emoji are presentation. This is named data (not a magic literal): the single source the
// picker and the calendar tile both read.
export const EVENT_CATEGORIES: CategoryOption[] = [
    {value: 'work', label: 'Work', emoji: '💼'},
    {value: 'meeting', label: 'Meeting', emoji: '👥'},
    {value: 'personal', label: 'Personal', emoji: '🏠'},
    {value: 'meal', label: 'Meal', emoji: '🍽️'},
    {value: 'travel', label: 'Travel', emoji: '🚗'},
    {value: 'health', label: 'Health', emoji: '🏥'},
    {value: 'education', label: 'Education', emoji: '📚'},
    {value: 'celebration', label: 'Celebration', emoji: '🎉'},
    {value: 'reminder', label: 'Reminder', emoji: '🎯'},
]

// categoryEmoji returns the emoji for a category value, matching case-insensitively and ignoring
// surrounding whitespace, or an empty string when the value is empty or unrecognised. The empty result
// is the signal the calendar uses to show no emoji at all.
export function categoryEmoji(category: string): string {
    const key = category.trim().toLowerCase()
    if (key === '') {
        return ''
    }
    const match = EVENT_CATEGORIES.find((c) => c.value === key)
    return match ? match.emoji : ''
}
