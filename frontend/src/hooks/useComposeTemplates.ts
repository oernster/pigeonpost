import {useState} from 'react'
import type {Dispatch, SetStateAction} from 'react'
import type {Editor} from '@tiptap/react'
import {api, Template} from '../api'
import type {useComposeIntake} from './useComposeIntake'

export interface ComposeTemplatesDeps {
    editor: Editor | null
    subject: string
    setSubject: Dispatch<SetStateAction<string>>
    markDirty: () => void
    intake: ReturnType<typeof useComposeIntake>
    setError: (message: string) => void
}

// useComposeTemplates is the compose window's template picker: the templates, whether the picker is open and
// applying one (subject when empty, body at the cursor, then its files).
export function useComposeTemplates(deps: ComposeTemplatesDeps) {
    const {editor, subject, setSubject, markDirty, intake, setError} = deps
    // The message-template picker: the loaded templates and whether its dropdown is open. Templates are
    // fetched the first time the picker is opened so an unused compose window makes no call.
    const [templates, setTemplates] = useState<Template[]>([])
    const [templatePicker, setTemplatePicker] = useState(false)

    // openTemplatePicker toggles the template dropdown, loading the templates on first open so the list is
    // current without fetching until it is wanted.
    const openTemplatePicker = async () => {
        if (templatePicker) {
            setTemplatePicker(false)
            return
        }
        try {
            setTemplates(await api.listTemplates())
            setTemplatePicker(true)
        } catch (e) {
            setError(String(e))
        }
    }

    // insertTemplate applies a chosen template: it fills the subject when it is still empty (so a template
    // never overwrites a subject already typed) and inserts the template body HTML at the cursor, then marks
    // the draft dirty so the change is autosaved.
    const insertTemplate = async (t: Template) => {
        setTemplatePicker(false)
        if (subject.trim() === '' && t.subject !== '') {
            setSubject(t.subject)
        }
        if (t.body !== '') {
            editor?.chain().focus().insertContent(t.body).run()
        }
        markDirty()
        // The files come over only now, for the one template chosen: a template may carry a whole
        // message's worth of bytes, so the picker's listing describes them and this reads them.
        if ((t.attachments ?? []).length === 0) {
            return
        }
        try {
            intake.add(await api.templateFiles(t.id))
        } catch (e) {
            setError(String(e))
        }
    }

    return {templates, templatePicker, setTemplatePicker, openTemplatePicker, insertTemplate}
}
