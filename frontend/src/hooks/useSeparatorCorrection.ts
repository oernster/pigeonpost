import {useState} from 'react'
import {detectSeparatorFix} from '../composeAddresses'

// Correction is a proposed fix for a wrong address separator: the recipient fields rewritten with their
// addresses correctly separated, plus a preview of the changed fields for the user to approve.
interface Correction {
    to: string
    cc: string
    bcc: string
    preview: string
}

interface CorrectionFields {
    to: string
    cc: string
    bcc: string
    setTo: (value: string) => void
    setCc: (value: string) => void
    setBcc: (value: string) => void
    setError: (value: string) => void
    markDirty: () => void
}

// useSeparatorCorrection owns the "wrong separator between valid addresses" safeguard for the compose window.
// On a send attempt offer scans the recipient fields; when it spots the mistake it records a proposed fix
// (pending) so the banner shows it for approval instead of sending. apply writes the corrected addresses back
// and marks the draft dirty; dismiss drops the suggestion. It is local only and never touches the server.
export function useSeparatorCorrection(
    {to, cc, bcc, setTo, setCc, setBcc, setError, markDirty}: CorrectionFields,
) {
    // pending holds a proposed fix for a wrong address separator, shown for the user to approve before the
    // message is sent.
    const [pending, setPending] = useState<Correction | null>(null)

    // separatorFix scans the recipient fields for the "wrong separator between valid addresses" mistake. When
    // it finds one, it returns the fields rewritten with the addresses correctly separated by "; " for the
    // user to approve. It returns null when nothing needs fixing, so a correctly-typed or a genuinely-invalid
    // single address is left for the normal send path (and the backend) to handle.
    const separatorFix = (): Correction | null => {
        const fixedTo = detectSeparatorFix(to)
        const fixedCc = detectSeparatorFix(cc)
        const fixedBcc = detectSeparatorFix(bcc)
        if (fixedTo === null && fixedCc === null && fixedBcc === null) return null
        const next = {to: fixedTo ?? to, cc: fixedCc ?? cc, bcc: fixedBcc ?? bcc}
        const preview = [
            {value: next.to, changed: fixedTo !== null},
            {value: next.cc, changed: fixedCc !== null},
            {value: next.bcc, changed: fixedBcc !== null},
        ].filter((field) => field.changed).map((field) => field.value).join('\n')
        return {...next, preview}
    }

    // offer runs the scan; when a fix is found it records the fix so the banner appears. It returns true when
    // a correction was offered, so the send path can stop and wait for the user to approve or dismiss it.
    const offer = (): boolean => {
        const fix = separatorFix()
        if (fix === null) return false
        setPending(fix)
        return true
    }

    // apply accepts the suggested separator fix, writing the corrected addresses back into the fields so the
    // user can review them and send.
    const apply = () => {
        if (pending === null) return
        setTo(pending.to)
        setCc(pending.cc)
        setBcc(pending.bcc)
        markDirty()
        setPending(null)
        setError('')
    }

    const dismiss = () => setPending(null)

    return {pending, offer, apply, dismiss}
}
