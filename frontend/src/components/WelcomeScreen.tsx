import {Dispatch, SetStateAction} from 'react'

export interface WelcomeScreenProps {
    setSettingUp: Dispatch<SetStateAction<boolean>>
}

// WelcomeScreen is the no-accounts empty state: a card inviting the user to add their first mail
// account. App shows it once the splash has cleared and there are still no accounts.
//
// It carried a large app mark in the top-left of the pane, which read as a second pigeon directly under
// the one now in the corner of the title bar. The corner holds the mark; this screen holds the card.
export function WelcomeScreen({setSettingUp}: WelcomeScreenProps) {
    return (
        <div className="empty-state">
            <div className="empty-card">
                <h2>Welcome to PigeonPost</h2>
                <p>Add your mail account and you are in: PigeonPost syncs it straight away; your mail
                    stays with your email provider.</p>
                <button className="btn primary" onClick={() => setSettingUp(true)}>Add account</button>
            </div>
        </div>
    )
}
