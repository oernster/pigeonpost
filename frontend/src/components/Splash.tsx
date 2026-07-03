import icon from '../assets/pigeonpost.png'

interface SplashProps {
    version: string
    author: string
    fading: boolean
}

export function Splash({version, author, fading}: SplashProps) {
    return (
        <div className={'splash' + (fading ? ' fading' : '')}>
            <img className="splash-icon" src={icon} alt="PigeonPost"/>
            <div className="splash-name">PigeonPost</div>
            {author && <div className="splash-author">by {author}</div>}
            {version && <div className="splash-version">v{version}</div>}
        </div>
    )
}
