import {useState} from 'react'
import {api, Tag} from '../api'
import {ConfirmDialog} from './ConfirmDialog'

interface TagManagerModalProps {
    tags: Tag[]
    onChanged: () => void
    onClose: () => void
}

// TAG_PALETTE is the fixed set of tag colours. A small, curated palette keeps tags visually
// consistent and legible on both themes rather than allowing arbitrary colours.
const TAG_PALETTE: readonly string[] = [
    '#e05252', // red
    '#e8833a', // orange
    '#d9a300', // amber
    '#4caf6e', // green
    '#2fb3a8', // teal
    '#3d7ff2', // blue
    '#7c5cff', // violet
    '#e05299', // pink
    '#a5744b', // brown
    '#8a94a6', // slate
]

export function TagManagerModal({tags, onChanged, onClose}: TagManagerModalProps) {
    const [editingId, setEditingId] = useState('')
    const [name, setName] = useState('')
    const [colour, setColour] = useState(TAG_PALETTE[0])
    const [busy, setBusy] = useState(false)
    const [error, setError] = useState('')
    const [tagToDelete, setTagToDelete] = useState<Tag | null>(null)

    const editing = editingId !== ''

    const resetForm = () => {
        setEditingId('')
        setName('')
        setColour(TAG_PALETTE[0])
    }

    const startEdit = (tag: Tag) => {
        setEditingId(tag.id)
        setName(tag.name)
        setColour(tag.colour)
    }

    // save creates a new tag or updates the one being edited (a present id updates in place).
    const save = async () => {
        setBusy(true)
        setError('')
        try {
            await api.saveTag({id: editingId, name: name.trim(), colour})
            resetForm()
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    const confirmDelete = async () => {
        if (!tagToDelete) {
            return
        }
        setBusy(true)
        setError('')
        try {
            await api.deleteTag(tagToDelete.id)
            if (tagToDelete.id === editingId) {
                resetForm()
            }
            setTagToDelete(null)
            onChanged()
        } catch (e) {
            setError(String(e))
        } finally {
            setBusy(false)
        }
    }

    return (
        <div className="modal-backdrop" onClick={onClose}>
            <div className="modal tags" role="dialog" aria-label="Manage tags" onClick={(e) => e.stopPropagation()}>
                <h2 className="modal-title">Tags</h2>
                {error && <div className="compose-error">{error}</div>}

                {tags.length === 0 ? (
                    <p className="empty-body">No tags yet. Create one below.</p>
                ) : (
                    <ul className="tag-manager-list">
                        {tags.map((tag) => (
                            <li key={tag.id} className={'tag-manager-row' + (tag.id === editingId ? ' editing' : '')}>
                                <button className="tag-manager-edit" title="Edit tag" onClick={() => startEdit(tag)}>
                                    <span className="tag-swatch" style={{backgroundColor: tag.colour}}/>
                                    <span className="tag-manager-name">{tag.name}</span>
                                </button>
                                <button
                                    className="account-action delete"
                                    aria-label={`Delete tag ${tag.name}`}
                                    title="Delete tag"
                                    onClick={() => setTagToDelete(tag)}
                                >
                                    &times;
                                </button>
                            </li>
                        ))}
                    </ul>
                )}

                <div className="tag-create">
                    <div className="tag-form-label">{editing ? 'Edit tag' : 'New tag'}</div>
                    <input
                        className="tag-name-input"
                        value={name}
                        onChange={(e) => setName(e.target.value)}
                        placeholder="Tag name"
                    />
                    <div className="tag-palette" role="radiogroup" aria-label="Tag colour">
                        {TAG_PALETTE.map((c) => (
                            <button
                                key={c}
                                type="button"
                                className={'tag-palette-swatch' + (c === colour ? ' selected' : '')}
                                style={{backgroundColor: c}}
                                role="radio"
                                aria-checked={c === colour}
                                aria-label={`Colour ${c}`}
                                onClick={() => setColour(c)}
                            />
                        ))}
                    </div>
                    <div className="tag-form-actions">
                        {editing && (
                            <button className="btn" onClick={resetForm} disabled={busy}>Cancel</button>
                        )}
                        <button className="btn primary" onClick={() => void save()} disabled={busy || name.trim() === ''}>
                            {editing ? 'Save changes' : 'Add tag'}
                        </button>
                    </div>
                </div>

                <div className="modal-actions">
                    <button className="btn" onClick={onClose} disabled={busy}>Close</button>
                </div>
            </div>

            {tagToDelete && (
                <ConfirmDialog
                    title="Delete tag"
                    message={`Delete the tag "${tagToDelete.name}"? It is removed from every message it is attached to.`}
                    confirmLabel="Delete tag"
                    busy={busy}
                    onConfirm={() => void confirmDelete()}
                    onCancel={() => setTagToDelete(null)}
                />
            )}
        </div>
    )
}
