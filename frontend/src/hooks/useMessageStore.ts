import {Dispatch, SetStateAction, useCallback, useState} from 'react'
import type {Message} from '../api'

// MessageStore is the coupled core of the mail views: the folder listing, the search results and the open
// reader tabs, plus the one active message shown in the reader. The three lists are kept as one unit on
// purpose, because almost every message action (read, flag, delete, move, tag) has to update a message
// wherever it appears; splitting them into separate hooks would scatter that update and let the lists drift
// out of step.
export interface MessageStore {
    messages: Message[]
    setMessages: Dispatch<SetStateAction<Message[]>>
    searchResults: Message[]
    setSearchResults: Dispatch<SetStateAction<Message[]>>
    tabs: Message[]
    setTabs: Dispatch<SetStateAction<Message[]>>
    selectedMessage: Message | null
    setSelectedMessage: Dispatch<SetStateAction<Message | null>>
    // applyToAllLists maps the folder list, the search results, the reader tabs and the active message
    // through fn, so an in-place field change (marking read, starring) shows at once wherever the message is.
    applyToAllLists: (fn: (m: Message) => Message) => void
    // removeFromAllLists drops every message whose id is in ids from all three lists, and clears the active
    // message when it was one of them.
    removeFromAllLists: (ids: Set<string>) => void
}

export function useMessageStore(): MessageStore {
    const [messages, setMessages] = useState<Message[]>([])
    const [searchResults, setSearchResults] = useState<Message[]>([])
    const [tabs, setTabs] = useState<Message[]>([])
    const [selectedMessage, setSelectedMessage] = useState<Message | null>(null)

    const applyToAllLists = useCallback((fn: (m: Message) => Message) => {
        setMessages((prev) => prev.map(fn))
        setSearchResults((prev) => prev.map(fn))
        setTabs((prev) => prev.map(fn))
        setSelectedMessage((prev) => (prev ? fn(prev) : prev))
    }, [])

    const removeFromAllLists = useCallback((ids: Set<string>) => {
        setMessages((prev) => prev.filter((m) => !ids.has(m.id)))
        setSearchResults((prev) => prev.filter((m) => !ids.has(m.id)))
        setTabs((prev) => prev.filter((m) => !ids.has(m.id)))
        setSelectedMessage((prev) => (prev && ids.has(prev.id) ? null : prev))
    }, [])

    return {
        messages, setMessages,
        searchResults, setSearchResults,
        tabs, setTabs,
        selectedMessage, setSelectedMessage,
        applyToAllLists, removeFromAllLists,
    }
}
