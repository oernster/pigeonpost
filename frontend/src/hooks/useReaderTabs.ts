import {Dispatch, RefObject, SetStateAction, useCallback, useEffect, useRef, useState} from 'react'
import type {Message} from '../api'
import type {MessageStore} from './useMessageStore'

// READING_PANE_KEY is the localStorage key the reading-pane preference persists under.
const READING_PANE_KEY = 'pigeonpost.readingPane'

// ReaderTabsDeps is what the reading-pane and reader-tab handlers need from the rest of App: the message
// store, whose reader tabs and active message openInNewTab and closeTab mutate.
export interface ReaderTabsDeps {
    store: MessageStore
}

export interface ReaderTabs {
    // previewEnabled is the right-hand reading pane; readingFull is the full-width reader shown when the
    // pane is off and a message is opened. setReadingFull is exposed because navigating away (selecting an
    // account, a folder or another row) drops the full-width reader back to the list.
    previewEnabled: boolean
    readingFull: boolean
    setReadingFull: Dispatch<SetStateAction<boolean>>
    readerBodyRef: RefObject<HTMLDivElement>
    readerSinkRef: RefObject<HTMLSpanElement>
    selectMessage: (message: Message) => void
    openInNewTab: (message: Message, fromKeyboard?: boolean) => void
    closeTab: (id: string) => void
    togglePreview: () => void
    // popoutOpen shows the selected message in its own dialog over the app (the Thunderbird-style
    // open, fired by double-click or Enter on a row); openPopout and closePopout drive it.
    popoutOpen: boolean
    openPopout: (message: Message) => void
    closePopout: () => void
}

// useReaderTabs owns the reading-pane mode and the reader-tab interaction: whether the preview pane is on,
// whether a message is open full-width, and the focus choreography when an email is opened or closed. The
// reader tabs and the active message themselves live in the message store (they are part of the coupled
// core); this hook drives how they are shown. The preference is persisted so it survives a restart.
export function useReaderTabs(deps: ReaderTabsDeps): ReaderTabs {
    const {store} = deps
    const {setTabs, setSelectedMessage} = store

    const [previewEnabled, setPreviewEnabled] = useState<boolean>(() => {
        try {
            return localStorage.getItem(READING_PANE_KEY) !== 'off'
        } catch {
            return true
        }
    })
    const [readingFull, setReadingFull] = useState<boolean>(false)
    // popoutOpen renders the selected message's reader inside a modal dialog over the app.
    const [popoutOpen, setPopoutOpen] = useState<boolean>(false)
    // emailOpenTick bumps each time an email is opened; the effect below then moves focus onto the opened
    // email's close cross. readerBodyRef points at the reader's scrollable body (a keyboard scroll stop).
    const [emailOpenTick, setEmailOpenTick] = useState<number>(0)
    // listReturnTick bumps when an email is closed, so the effect below returns focus to the message list.
    const [listReturnTick, setListReturnTick] = useState<number>(0)
    const readerBodyRef = useRef<HTMLDivElement>(null)
    // readerSinkRef is a neutral anchor at the top of the full-width reader; openSourceRef records whether
    // the last open was by keyboard or mouse.
    const readerSinkRef = useRef<HTMLSpanElement>(null)
    const openSourceRef = useRef<'keyboard' | 'mouse'>('keyboard')

    // Persist the reading-pane preference so it survives a restart.
    useEffect(() => {
        try {
            localStorage.setItem(READING_PANE_KEY, previewEnabled ? 'on' : 'off')
        } catch {
            // A storage failure just means the preference is not remembered; the UI still works.
        }
    }, [previewEnabled])

    // selectMessage highlights a message. With the reading pane on it shows in the preview (and the view
    // effect marks it read); with the pane off it only highlights the row and stays on the list.
    const selectMessage = useCallback((message: Message) => {
        setSelectedMessage(message)
        setReadingFull(false)
        setPopoutOpen(false)
    }, [])

    // openInNewTab pins a message as a reader tab (if not already open) and shows it. With the reading
    // pane off this opens the message full-width (readingFull); with it on the tab appears in the pane.
    const openInNewTab = useCallback((message: Message, fromKeyboard = true) => {
        setTabs((prev) => (prev.some((t) => t.id === message.id) ? prev : [...prev, message]))
        setSelectedMessage(message)
        setReadingFull(true)
        // Record how the email was opened and signal the focus effect below.
        openSourceRef.current = fromKeyboard ? 'keyboard' : 'mouse'
        setEmailOpenTick((n) => n + 1)
    }, [])

    // When an email is opened (emailOpenTick bumped), move focus into the reader so the keyboard does not
    // fall back to the start of the ring (which made the next Tab land on File). Focus lands on the active
    // tab's close cross, the first stop within an open email, so it can be shut with one key; a following
    // Tab moves on to the toolbar then the body.
    useEffect(() => {
        if (emailOpenTick === 0) {
            return
        }
        document.querySelector<HTMLElement>('.reader-tab.active .reader-tab-close')?.focus()
    }, [emailOpenTick])

    // After an email is closed, put focus on the topmost message header so the keyboard returns to the list
    // rather than being stranded on the now-gone reader controls.
    useEffect(() => {
        if (listReturnTick === 0) {
            return
        }
        document.querySelector<HTMLElement>('.message-list .message-row')?.focus()
    }, [listReturnTick])

    // openPopout shows a message in its own dialog over the app (the Thunderbird-style open): the
    // message becomes the selection, so its body loads and mark-on-view applies, and the popout flag
    // renders the reader inside a modal. The focus effect below lands on the dialog's close cross.
    const openPopout = useCallback((message: Message) => {
        setSelectedMessage(message)
        setReadingFull(false)
        setPopoutOpen(true)
    }, [])

    // closePopout shuts the dialog and returns focus to the message list.
    const closePopout = useCallback(() => {
        setPopoutOpen(false)
        setListReturnTick((n) => n + 1)
    }, [])

    // When the popout opens, focus its close cross (the first stop within the dialog) so it can be
    // shut with one key; Tab then walks the reader's own controls inside the dialog's focus trap.
    useEffect(() => {
        if (!popoutOpen) {
            return
        }
        document.querySelector<HTMLElement>('.message-popout .modal-close')?.focus()
    }, [popoutOpen])

    // togglePreview flips the reading pane and returns to the list, so toggling never strands the user in
    // the full-width reader.
    const togglePreview = useCallback(() => {
        setPreviewEnabled((v) => !v)
        setReadingFull(false)
    }, [])

    // closeTab removes a tab; if it was the message on screen, selection moves to the neighbouring tab
    // (or clears when none remain).
    const closeTab = useCallback((id: string) => {
        setTabs((prev) => {
            const idx = prev.findIndex((t) => t.id === id)
            const next = prev.filter((t) => t.id !== id)
            setSelectedMessage((sel) => (sel?.id === id ? (next[Math.min(idx, next.length - 1)] ?? null) : sel))
            return next
        })
        // Closing an email returns to the message list; the effect keyed on listReturnTick lands focus on
        // the topmost header once the list has re-rendered.
        setReadingFull(false)
        setListReturnTick((n) => n + 1)
    }, [])

    return {
        previewEnabled, readingFull, setReadingFull,
        readerBodyRef, readerSinkRef,
        selectMessage, openInNewTab, closeTab, togglePreview,
        popoutOpen, openPopout, closePopout,
    }
}
