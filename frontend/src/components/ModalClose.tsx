interface ModalCloseProps {
    onClose: () => void
}

// ModalClose is the window-style close cross shown in the top-right corner of every dialog. It sits
// absolutely inside the modal, so each modal just drops it in as its first child.
export function ModalClose({onClose}: ModalCloseProps) {
    return (
        <button
            type="button"
            className="modal-close"
            aria-label="Close"
            title="Close"
            onClick={onClose}
        >
            &times;
        </button>
    )
}
