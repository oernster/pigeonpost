// ErrorBar is the main window's error surface: the red banner under the toolbar that reports a failed
// action (a move that could not complete, a sync failure). Errors persist until the user dismisses them
// or a later action replaces them, so the bar carries an explicit dismiss control; without one the only
// way to clear a stale error was to trigger another action that happened to succeed.
export function ErrorBar({message, onDismiss}: {message: string, onDismiss: () => void}) {
    if (!message) {
        return null
    }
    return (
        <div className="error-bar" role="alert">
            <span className="error-bar-text">{message}</span>
            <button
                type="button"
                className="error-bar-dismiss"
                aria-label="Dismiss error"
                data-tip="Dismiss"
                onClick={onDismiss}
            >
                ×
            </button>
        </div>
    )
}
