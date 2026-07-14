import {Dispatch, SetStateAction} from 'react'
import brandIcon from '../assets/pigeonpost.png'

export interface WelcomeScreenProps {
    setSettingUp: Dispatch<SetStateAction<boolean>>
}

// WelcomeScreen is the no-accounts empty state: the brand image and a card inviting the user to add their
// first mail account. App shows it once the splash has cleared and there are still no accounts.
export function WelcomeScreen({setSettingUp}: WelcomeScreenProps) {
    return (
        <div className="empty-state welcome">
            <img className="welcome-brand" src={brandIcon} alt="" aria-hidden="true"/>
            <div className="empty-card">
                <h2>Welcome to PigeonPost</h2>
                <p>Add your mail account and you are in: PigeonPost syncs it straight away; your mail
                    stays on your server.</p>
                <button className="btn primary" onClick={() => setSettingUp(true)}>Add account</button>
            </div>
        </div>
    )
}
