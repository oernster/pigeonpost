import {useState} from 'react'
import type {Editor} from '@tiptap/react'

// useLinkEditor drives the inline link-editing row shared by the composer and the signature editor: whether
// the row is open and the url being typed, opening it seeded with the current selection's link, applying it
// (an empty url removes the link, a non-empty one sets it after normalising) and removing it outright. The
// normalise function is injected so each caller keeps its own url policy.
export function useLinkEditor(editor: Editor | null, normalise: (url: string) => string) {
    const [open, setOpen] = useState(false)
    const [url, setUrl] = useState('')

    const openLink = () => {
        setUrl((editor?.getAttributes('link').href as string) ?? '')
        setOpen(true)
    }

    const applyLink = () => {
        const href = normalise(url)
        if (href === '') {
            editor?.chain().focus().extendMarkRange('link').unsetLink().run()
        } else {
            editor?.chain().focus().extendMarkRange('link').setLink({href}).run()
        }
        setOpen(false)
        setUrl('')
    }

    const removeLink = () => {
        editor?.chain().focus().extendMarkRange('link').unsetLink().run()
        setOpen(false)
        setUrl('')
    }

    return {open, url, setUrl, openLink, applyLink, removeLink}
}
