import {useCallback, useEffect, useState} from 'react'
import {Message, Tag, api} from '../api'
import {TAG_PALETTE, colourTagId} from '../tagColours'
import type {MessageStore} from './useMessageStore'

// TagsDeps is what tagging needs from the rest of App: the message store (a tag colour change updates the
// message wherever it appears, and the selected message drives which chips are shown) and the error sink.
export interface TagsDeps {
    store: MessageStore
    setError: (message: string) => void
}

export interface Tags {
    // tags is the fixed colour palette persisted as tags; messageTags is the set attached to the selected
    // message (the chips shown in the reader and the ticks in the Tag-with-colour menu).
    tags: Tag[]
    messageTags: Tag[]
    toggleTag: (tagId: string, assigned: boolean) => Promise<void>
    setMessageTagById: (messageId: string, tagId: string, assigned: boolean) => Promise<void>
}

// useTags owns the colour-tag palette, the selected message's tags, and toggling a tag on the open message
// (toggleTag) or any message (setMessageTagById, used by the context menu). A toggle persists the tag, then
// updates the on-screen chips and the coloured dots on every list at once. The palette is seeded once on
// mount so a colour can always be applied.
export function useTags(deps: TagsDeps): Tags {
    const {store, setError} = deps
    const {selectedMessage, setMessages, setSearchResults, setTabs} = store

    const [tags, setTags] = useState<Tag[]>([])
    const [messageTags, setMessageTags] = useState<Tag[]>([])

    // Ensure the fixed colour palette exists as tags, so a colour can be applied and its swatch shown. The
    // writes are sequential and only fill in missing colours, because SQLite is single-writer and firing
    // them at once trips "database is locked".
    useEffect(() => {
        void (async () => {
            try {
                const existing = await api.listTags()
                const have = new Set(existing.map((t) => t.id))
                for (const c of TAG_PALETTE) {
                    const id = colourTagId(c.colour)
                    if (!have.has(id)) {
                        await api.saveTag({id, name: c.name, colour: c.colour})
                    }
                }
                setTags(await api.listTags())
            } catch (e) {
                setError(String(e))
            }
        })()
    }, [])

    // Load the tags attached to the selected message whenever the selection changes.
    useEffect(() => {
        if (!selectedMessage) {
            setMessageTags([])
            return
        }
        const messageId = selectedMessage.id
        let active = true
        void api.messageTags(messageId).then((t) => {
            if (active) {
                setMessageTags(t)
            }
        }).catch((e) => setError(String(e)))
        return () => {
            active = false
        }
    }, [selectedMessage])

    // applyTagColourToLists updates a message's tag colours in every on-screen list after a tag is toggled,
    // so the coloured dots on its row appear or disappear at once rather than only after a reload. Each tag
    // id maps to a single palette colour, so at most one dot of that colour is ever present.
    const applyTagColourToLists = useCallback((messageId: string, tagId: string, assigned: boolean) => {
        const colour = tags.find((t) => t.id === tagId)?.colour
        if (!colour) {
            return
        }
        const apply = (m: Message): Message => {
            if (m.id !== messageId) {
                return m
            }
            const has = m.tagColours.includes(colour)
            if (assigned && !has) {
                return {...m, tagColours: [...m.tagColours, colour]}
            }
            if (!assigned && has) {
                return {...m, tagColours: m.tagColours.filter((c) => c !== colour)}
            }
            return m
        }
        setMessages((prev) => prev.map(apply))
        setSearchResults((prev) => prev.map(apply))
        setTabs((prev) => prev.map(apply))
    }, [tags])

    const toggleTag = useCallback(async (tagId: string, assigned: boolean) => {
        if (!selectedMessage) {
            return
        }
        try {
            await api.setMessageTag(selectedMessage.id, tagId, assigned)
            setMessageTags(await api.messageTags(selectedMessage.id))
            applyTagColourToLists(selectedMessage.id, tagId, assigned)
        } catch (e) {
            setError(String(e))
        }
    }, [selectedMessage, applyTagColourToLists])

    // setMessageTagById toggles a tag on any message (not only the selected one), used by the context
    // menu. When it targets the open message, its tag chips are refreshed too.
    const setMessageTagById = useCallback(async (messageId: string, tagId: string, assigned: boolean) => {
        try {
            await api.setMessageTag(messageId, tagId, assigned)
            if (selectedMessage?.id === messageId) {
                setMessageTags(await api.messageTags(messageId))
            }
            applyTagColourToLists(messageId, tagId, assigned)
        } catch (e) {
            setError(String(e))
        }
    }, [selectedMessage, applyTagColourToLists])

    return {tags, messageTags, toggleTag, setMessageTagById}
}
