import {useRef, useState} from 'react'
import {api} from '../api'
import {Suggestion, suggestionPool} from '../recipientSuggest'

// useContactPool lazily loads the address book into the suggestion pool the recipient fields share.
// Nothing is fetched until a recipient field is first touched, so a compose window whose recipients
// are never edited makes no call; the fetch happens once per compose window. A load failure leaves
// the pool empty (suggestions are a nicety, so their absence is the whole error handling).
export function useContactPool() {
    const [pool, setPool] = useState<Suggestion[]>([])
    const requested = useRef(false)

    const ensurePool = () => {
        if (requested.current) {
            return
        }
        requested.current = true
        api.listContacts()
            .then((contacts) => setPool(suggestionPool(contacts)))
            .catch(() => setPool([]))
    }

    return {pool, ensurePool}
}
