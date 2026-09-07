// The artwork for the title bar and the folder list. Every glyph is imported here and nowhere else, so
// this module is the one home for the mapping from a name to a picture: a component asks for icons.inbox
// rather than carrying a path of its own.
//
// The files are generated from the masters in assets/ at the repo root by `go run ./tools/genicons`,
// which the build runs before it builds. Both the masters and the derived glyphs are committed, as the
// About dialog's pigeonpost.png is: the front end imports each one, so a missing file fails the build
// rather than shipping a bar with a gap in it. Adding a glyph therefore means adding its master, running
// the generator and committing both.
import addAccount from './assets/icons/add-account.png'
import archive from './assets/icons/archive.png'
import calendar from './assets/icons/calendar.png'
import compose from './assets/icons/compose.png'
import contacts from './assets/icons/contacts.png'
import darkMode from './assets/icons/dark-mode.png'
import deletedItems from './assets/icons/deleted-items.png'
import drafts from './assets/icons/drafts.png'
import edit from './assets/icons/edit.png'
import file from './assets/icons/file.png'
import help from './assets/icons/help.png'
import inbox from './assets/icons/inbox.png'
import junk from './assets/icons/junk.png'
import lightMode from './assets/icons/light-mode.png'
import mail from './assets/icons/mail.png'
import outbox from './assets/icons/outbox.png'
import rules from './assets/icons/rules.png'
import sentMessages from './assets/icons/sent-messages.png'
import snooze from './assets/icons/snooze.png'
import sync from './assets/icons/sync.png'
import template from './assets/icons/template.png'
import view from './assets/icons/view.png'

export const icons = {
    addAccount,
    archive,
    calendar,
    compose,
    contacts,
    // darkMode is the moon: the control that switches INTO dark mode, so it is the one shown while the
    // app is light. lightMode is its sun counterpart. Naming them after the mode they move to keeps the
    // toggle readable at the call site.
    darkMode,
    deletedItems,
    drafts,
    edit,
    // file is the folder mark, standing for the File menu in the tray and for every ordinary folder and
    // subfolder in the folder list.
    file,
    help,
    inbox,
    junk,
    lightMode,
    mail,
    outbox,
    rules,
    sentMessages,
    snooze,
    sync,
    template,
    view,
} as const

// folderIcon maps a folder's kind to the picture its row carries. Every named kind has a drawing of its
// own; icons.file is the ordinary folder mark, standing for custom folders and their subfolders.
export const folderIcon: Record<string, string> = {
    inbox: icons.inbox,
    sent: icons.sentMessages,
    drafts: icons.drafts,
    trash: icons.deletedItems,
    junk: icons.junk,
    archive: icons.archive,
    outbox: icons.outbox,
    custom: icons.file,
}
