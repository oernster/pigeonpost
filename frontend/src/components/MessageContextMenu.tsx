import {ReactNode, useEffect, useLayoutEffect, useRef, useState} from 'react'
import {api, Folder, Message, Tag} from '../api'
import {TAG_PALETTE, colourTagId} from '../tagColours'
import {isOutboxMessage} from '../outbox'

interface MessageContextMenuProps {
    message: Message
    x: number
    y: number
    folders: Folder[]
    tags: Tag[]
    onClose: () => void
    onReply: (message: Message) => void
    onReplyAll: (message: Message) => void
    onForward: (message: Message) => void
    onSetRead: (message: Message, read: boolean) => void
    onToggleFlag: (message: Message) => void
    onMove: (message: Message, destFolderId: string) => void
    onCopy: (message: Message, destFolderId: string) => void
    // canMoveCopy is false for POP3 accounts, which have a single inbox and no server-side move/copy.
    canMoveCopy: boolean
    onSetTag: (messageId: string, tagId: string, assigned: boolean) => void
    onOpenInNewTab: (message: Message) => void
    onSaveAs: (message: Message) => void
    onPrint: (message: Message) => void
    onAttachToNew: (message: Message) => void
    onDelete: (message: Message) => void
    onDeletePermanent: (message: Message) => void
    // onCancelSend discards a queued outbox item; the menu offers only this for an outbox row.
    onCancelSend: (message: Message) => void
}

// Keep the menu at least this far inside the viewport edges when clamping its position.
const MENU_MARGIN = 8
// Approximate width of the menu plus one flyout. When the menu opens within this distance of the right
// edge, flyouts open leftwards instead so they stay on screen.
const SUBMENU_REACH = 400

// SubMenu is a menu row whose flyout opens on hover (or when focus is within, for keyboard and
// click-to-focus), rather than replacing the menu on click. The flyout direction is set by the parent
// menu's `flip` class so it stays on screen near the right edge.
function SubMenu({label, scroll, children}: {label: string; scroll?: boolean; children: ReactNode}) {
    return (
        <div className="context-sub-wrap">
            <button className="context-item" role="menuitem" aria-haspopup="menu">
                <span className="context-item-label">{label}</span>
                <span className="context-chevron">&#9656;</span>
            </button>
            <div className={'context-submenu' + (scroll ? ' scroll' : '')} role="menu">
                {children}
            </div>
        </div>
    )
}

export function MessageContextMenu(props: MessageContextMenuProps) {
    const {message, folders, onClose} = props
    const ref = useRef<HTMLDivElement>(null)
    const [pos, setPos] = useState({x: props.x, y: props.y})
    const [assigned, setAssigned] = useState<Set<string>>(new Set())

    // Fetch the tags already on this message so the Tag flyout can show which are set.
    useEffect(() => {
        let active = true
        void api.messageTags(message.id)
            .then((ts) => {
                if (active) {
                    setAssigned(new Set(ts.map((t) => t.id)))
                }
            })
            .catch(() => undefined)
        return () => {
            active = false
        }
    }, [message.id])

    // Dismiss on an outside click or the Escape key.
    useEffect(() => {
        const onDown = (e: MouseEvent) => {
            if (ref.current && !ref.current.contains(e.target as Node)) {
                onClose()
            }
        }
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') {
                onClose()
            }
        }
        document.addEventListener('mousedown', onDown)
        document.addEventListener('keydown', onKey)
        return () => {
            document.removeEventListener('mousedown', onDown)
            document.removeEventListener('keydown', onKey)
        }
    }, [onClose])

    // After the first render, nudge the menu back inside the viewport if the cursor was near an edge.
    // Flyouts are absolutely positioned, so they do not change the menu's own size and this runs once.
    useLayoutEffect(() => {
        const el = ref.current
        if (!el) {
            return
        }
        const rect = el.getBoundingClientRect()
        let nx = props.x
        let ny = props.y
        if (nx + rect.width > window.innerWidth - MENU_MARGIN) {
            nx = Math.max(MENU_MARGIN, window.innerWidth - rect.width - MENU_MARGIN)
        }
        if (ny + rect.height > window.innerHeight - MENU_MARGIN) {
            ny = Math.max(MENU_MARGIN, window.innerHeight - rect.height - MENU_MARGIN)
        }
        if (nx !== pos.x || ny !== pos.y) {
            setPos({x: nx, y: ny})
        }
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [props.x, props.y])

    // act runs a menu action and then closes the menu.
    const act = (fn: () => void) => () => {
        fn()
        onClose()
    }

    const movable = folders.filter((f) => f.id !== message.folderId)
    const repliesAll = ((message.to?.length ?? 0) + (message.cc?.length ?? 0)) > 0
    const flipX = pos.x > window.innerWidth - SUBMENU_REACH

    const colourRow = (
        <div className="context-colour-row" role="group" aria-label="Tag colour">
            {TAG_PALETTE.map((c) => {
                const id = colourTagId(c.colour)
                const isOn = assigned.has(id)
                return (
                    <button
                        key={id}
                        className={'context-colour' + (isOn ? ' selected' : '')}
                        role="menuitemcheckbox"
                        aria-checked={isOn}
                        title={c.name}
                        style={{backgroundColor: c.colour}}
                        onClick={() => {
                            props.onSetTag(message.id, id, !isOn)
                            setAssigned((prev) => {
                                const next = new Set(prev)
                                if (isOn) {
                                    next.delete(id)
                                } else {
                                    next.add(id)
                                }
                                return next
                            })
                        }}
                    >
                        {isOn ? '✓' : ''}
                    </button>
                )
            })}
        </div>
    )

    return (
        <div
            ref={ref}
            className={'context-menu' + (flipX ? ' flip' : '')}
            role="menu"
            style={{left: pos.x, top: pos.y}}
            onClick={(e) => e.stopPropagation()}
        >
            {isOutboxMessage(message) ? (
                <button
                    className="context-item danger"
                    role="menuitem"
                    onClick={act(() => props.onCancelSend(message))}
                >
                    Cancel send
                </button>
            ) : (
                <>
                    <button className="context-item" role="menuitem" onClick={act(() => props.onOpenInNewTab(message))}>
                        Open in new tab
                    </button>
                    <div className="context-sep"/>
                    <button className="context-item" role="menuitem" onClick={act(() => props.onReply(message))}>
                        Reply
                    </button>
                    {repliesAll && (
                        <button className="context-item" role="menuitem" onClick={act(() => props.onReplyAll(message))}>
                            Reply to all
                        </button>
                    )}
                    <button className="context-item" role="menuitem" onClick={act(() => props.onForward(message))}>
                        Forward
                    </button>
                    <button className="context-item" role="menuitem" onClick={act(() => props.onAttachToNew(message))}>
                        Attach to new message
                    </button>
                    <div className="context-sep"/>
                    <button className="context-item" role="menuitem" onClick={act(() => props.onSaveAs(message))}>
                        Save as...
                    </button>
                    <button className="context-item" role="menuitem" onClick={act(() => props.onPrint(message))}>
                        Print...
                    </button>
                    <div className="context-sep"/>
                    <SubMenu label="Mark">
                        <button
                            className="context-item"
                            role="menuitemradio"
                            aria-checked={message.read}
                            onClick={act(() => props.onSetRead(message, true))}
                        >
                            <span className="tag-check">{message.read ? '✓' : ''}</span>
                            <span className="context-item-label">Mark as read</span>
                        </button>
                        <button
                            className="context-item"
                            role="menuitemradio"
                            aria-checked={!message.read}
                            onClick={act(() => props.onSetRead(message, false))}
                        >
                            <span className="tag-check">{!message.read ? '✓' : ''}</span>
                            <span className="context-item-label">Mark as unread</span>
                        </button>
                        <div className="context-sep"/>
                        <button className="context-item" role="menuitem" onClick={act(() => props.onToggleFlag(message))}>
                            {message.flagged ? 'Remove star' : 'Add star'}
                        </button>
                        <div className="context-sep"/>
                        <SubMenu label="Tag with colour">
                            {colourRow}
                        </SubMenu>
                    </SubMenu>
                    {props.canMoveCopy && movable.length > 0 && (
                        <>
                            <div className="context-sep"/>
                            <SubMenu label="Move to" scroll>
                                {movable.map((f) => (
                                    <button key={f.id} className="context-item" role="menuitem"
                                            onClick={act(() => props.onMove(message, f.id))}>
                                        {f.name}
                                    </button>
                                ))}
                            </SubMenu>
                            <SubMenu label="Copy to" scroll>
                                {movable.map((f) => (
                                    <button key={f.id} className="context-item" role="menuitem"
                                            onClick={act(() => props.onCopy(message, f.id))}>
                                        {f.name}
                                    </button>
                                ))}
                            </SubMenu>
                        </>
                    )}
                    <div className="context-sep"/>
                    <button className="context-item danger" role="menuitem" onClick={act(() => props.onDelete(message))}>
                        Delete
                    </button>
                    <button className="context-item danger" role="menuitem" onClick={act(() => props.onDeletePermanent(message))}>
                        Delete permanently
                    </button>
                </>
            )}
        </div>
    )
}
