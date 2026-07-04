interface ModalCloseProps {
    onClose: () => void
}

// ModalClose is the window-style close cross shown in the top-right corner of every dialog. It sits
// absolutely inside the modal, so each modal just drops it in as its first child.
export function ModalClose({onClose}: ModalCloseProps) {
    // Close on mousedown so the cross works on the first click even when a native date input has focus:
    // Chromium swallows the first click after such a field while it commits and blurs. Keyboard activation
    // fires a click with detail 0, which still closes; a real mouse click (detail > 0) is already handled
    // by mousedown, so onClick ignores it and it does not fire twice.
    return (
        <button
            type="button"
            className="modal-close"
            aria-label="Close"
            title="Close"
            onMouseDown={onClose}
            onClick={(e) => {
                if (e.detail === 0) {
                    onClose()
                }
            }}
        >
            &times;
        </button>
    )
}
