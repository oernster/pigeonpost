import {normalisePastedHtml} from './pastedHtml'

// richText holds the settings every rich-text surface shares: the compose window, the template editor
// and the account signature. They are one home so the three cannot drift apart.

// EDITOR_LINK_OPTIONS configures the link mark that StarterKit already registers. Passing a separate
// Link extension alongside StarterKit registers the name twice; TipTap then warns and keeps only
// one of the two configurations, so a link could still open on click. Configure it through StarterKit
// instead and there is one registration carrying these settings.
export const EDITOR_LINK_OPTIONS = {openOnClick: false, autolink: true, linkOnPaste: true}

// EDITOR_PASTE_PROPS is the editorProps fragment that cleans clipboard markup on the way in. Spread it
// into an editor's own editorProps; see pastedHtml for what it removes and why.
export const EDITOR_PASTE_PROPS = {transformPastedHTML: normalisePastedHtml}
