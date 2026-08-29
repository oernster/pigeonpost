import donateIcon from '../assets/donate.png'
import {api} from '../api'

// DONATE_URL is the PayPal payment page the donate button opens. It is the one home for the address; no
// other surface links to it.
const DONATE_URL = 'https://www.paypal.com/ncp/payment/6QEJKCEQ3ZFZ8'

// BottomBar is the tray held at the foot of the window, built to the same shape as the title bar above so
// the two read as a matched pair. It carries the donate button at the far left, opening the payment page
// in the user's own browser rather than in the app's webview.
//
// Apart from the one link it owns it is presentational; the button has nothing to gate it, so the tray
// takes no props and needs no state from App.
export function BottomBar() {
    return (
        <footer className="titlebar bottombar">
            <div className="titlebar-actions">
                <button
                    className="icon-btn icon-btn-image"
                    data-tip="Donate to support PigeonPost"
                    aria-label="Donate to support PigeonPost"
                    onClick={() => void api.openExternal(DONATE_URL)}
                >
                    <img src={donateIcon} alt="" draggable={false}/>
                </button>
            </div>
        </footer>
    )
}
