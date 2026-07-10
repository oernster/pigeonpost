import {useState} from 'react'
import {Folder} from '../api'
import {ModalClose} from './ModalClose'
import {useBackdropDismiss} from './useBackdropDismiss'
import {moveTargets} from '../folderPaths'

interface MoveFolderDialogProps {
    folder: Folder
    // folders is the account's whole folder set, from which the valid destinations are derived.
    folders: Folder[]
    busy?: boolean
    // onMove reparents the folder under newParentId or to the top level when newParentId is empty.
    onMove: (newParentId: string) => void
    onCancel: () => void
}

// MoveFolderDialog is the modal for reparenting a custom folder. It lists the destinations the folder may
// be moved under (every other folder, excluding the folder itself, its own subtree and its current
// parent) plus a Top level entry. On confirm it moves the folder to the chosen destination. When there
// is no valid destination it says so and offers only Close.
export function MoveFolderDialog({folder, folders, busy, onMove, onCancel}: MoveFolderDialogProps) {
    const dismiss = useBackdropDismiss(onCancel)
    const targets = moveTargets(folder, folders)
    const [selected, setSelected] = useState<string>(targets.length > 0 ? targets[0].id : '')
    const canMove = targets.length > 0
    const submit = () => {
        if (canMove) {
            onMove(selected)
        }
    }
    return (
        <div className="modal-backdrop" {...dismiss}>
            <div className="modal" role="dialog" aria-label={`Move ${folder.name}`} onClick={(e) => e.stopPropagation()}>
                <ModalClose onClose={onCancel}/>
                <h2 className="modal-title">Move folder</h2>
                {canMove ? (
                    <>
                        <p className="confirm-message">Move "{folder.name}" under:</p>
                        <div className="move-target-list" role="radiogroup" aria-label="Destination folder">
                            {targets.map((t, i) => (
                                <label key={t.id || '__top__'} className="move-target">
                                    <input
                                        type="radio"
                                        name="move-target"
                                        value={t.id}
                                        checked={selected === t.id}
                                        disabled={busy}
                                        autoFocus={i === 0}
                                        onChange={() => setSelected(t.id)}
                                    />
                                    <span className="move-target-label">{t.label}</span>
                                </label>
                            ))}
                        </div>
                        <div className="modal-actions spread">
                            <button className="btn" onClick={onCancel} disabled={busy}>Cancel</button>
                            <button className="btn primary" onClick={submit} disabled={busy}>
                                {busy ? 'Working...' : 'Move here'}
                            </button>
                        </div>
                    </>
                ) : (
                    <>
                        <p className="confirm-message">
                            "{folder.name}" has nowhere else to go: there is no other folder to move it under.
                        </p>
                        <div className="modal-actions spread">
                            <button className="btn" onClick={onCancel} disabled={busy} autoFocus>Close</button>
                        </div>
                    </>
                )}
            </div>
        </div>
    )
}
