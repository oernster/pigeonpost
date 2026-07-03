import {Message} from '../api'

interface ReaderTabsProps {
    tabs: Message[]
    activeMessageId: string
    onSelectTab: (message: Message) => void
    onCloseTab: (id: string) => void
}

// ReaderTabs renders the strip of open message tabs above the reader. The tab whose message is the one
// currently shown is highlighted. Selecting a tab shows that message; closing it removes the tab.
export function ReaderTabs({tabs, activeMessageId, onSelectTab, onCloseTab}: ReaderTabsProps) {
    return (
        <div className="reader-tabs" role="tablist">
            {tabs.map((tab) => {
                const label = tab.subject || '(no subject)'
                return (
                    <div
                        key={tab.id}
                        role="tab"
                        tabIndex={0}
                        aria-selected={tab.id === activeMessageId}
                        className={'reader-tab' + (tab.id === activeMessageId ? ' active' : '')}
                        title={label}
                        onClick={() => onSelectTab(tab)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter') {
                                e.preventDefault()
                                onSelectTab(tab)
                            }
                        }}
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
