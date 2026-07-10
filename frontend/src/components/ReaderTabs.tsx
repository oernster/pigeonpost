import {Message} from '../api'

interface ReaderTabsProps {
    tabs: Message[]
    activeMessageId: string
    onSelectTab: (message: Message) => void
    onCloseTab: (id: string) => void
}

// ReaderTabs renders the strip of open message tabs above the reader. The tab whose message is the one
// currently shown is highlighted. Clicking a tab body shows that message; its close cross is the tab's
// keyboard stop (the first stop within an open email), so closing it is one key away and the title never
// takes focus on its own.
export function ReaderTabs({tabs, activeMessageId, onSelectTab, onCloseTab}: ReaderTabsProps) {
    return (
        <div className="reader-tabs" role="tablist">
            {tabs.map((tab) => {
                const label = tab.subject || '(no subject)'
                return (
                    <div
                        key={tab.id}
                        role="tab"
                        tabIndex={-1}
                        aria-selected={tab.id === activeMessageId}
                        className={'reader-tab' + (tab.id === activeMessageId ? ' active' : '')}
                        title={label}
                        onClick={() => onSelectTab(tab)}
                    >
                        <span className="reader-tab-title">{label}</span>
                        <button
                            className="reader-tab-close"
                            aria-label={`Close ${label}`}
                            onClick={(e) => {
                                e.stopPropagation()
                                onCloseTab(tab.id)
                            }}
                        >
                            &times;
                        </button>
                    </div>
                )
            })}
        </div>
    )
}
