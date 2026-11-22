import donateIcon from '../assets/donate.png'
import {api} from '../api'
import type {HoverStatus} from '../hoverStatus'
import {useHoverStatus} from '../hooks/useHoverStatus'

// DONATE_URL is the PayPal payment page the donate button opens. It is the one home for the address; no
// other surface links to it.
const DONATE_URL = 'https://www.paypal.com/ncp/payment/Q2U2RRG5WV7YJ'

// HoverReadout spells out the tray's reading. A link shows its real destination with the host in bold,
// since the host is what says whose site it is; when the link's visible text names a different site the
// readout says so first. A link PigeonPost will not open says that instead. An emoji shows its name.
function HoverReadout({status}: {status: HoverStatus}) {
    if (status.kind === 'emoji') {
        return <span className="hover-status-text">{status.glyph} {status.name}</span>
    }
    if (!status.openable) {
        return (
            <span className="hover-status-text">
                <span className="hover-status-warning">PigeonPost will not open this link: </span>
                {status.before}
            </span>
        )
    }
    return (
        <span className="hover-status-text">
            {status.shown !== null && (
                <span className="hover-status-warning">Link text shows {status.shown} but goes to </span>
            )}
            {status.before}<strong>{status.host}</strong>{status.after}
        </span>
    )
}

// BottomBar is the tray held at the foot of the window, built to the same shape as the title bar above so
// the two read as a matched pair. It carries the donate button at the far left, opening the payment page
// in the user's own browser rather than in the app's webview, then the hover readout: where a link under
// the pointer in a message really goes (so it can be judged before it is clicked) or the name of an emoji.
//
// While it has a reading the tray is raised above an open dialog's backdrop, so a link hovered in the
// compose window or a popped-out message is read at full contrast rather than through the dimming.
export function BottomBar() {
    const status = useHoverStatus()
    return (
        <footer className={status === null ? 'titlebar bottombar' : 'titlebar bottombar bottombar-raised'}>
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
            <div className="hover-status" role="status" aria-live="polite">
                {status !== null && <HoverReadout status={status}/>}
            </div>
        </footer>
    )
}
