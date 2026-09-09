// The Guide's text, held apart from the dialog that draws it so the component stays a renderer and the
// words stay one readable document. Two jobs, in this order. It NAMES the furniture, each entry carrying
// the REAL picture the title bar, the folder list or the foot strip actually draws, so a control that
// replaced a text label can be identified by someone who has just met it. Then it states the handful of
// rules the windows cannot say for themselves: what is held locally, what only happens while the app is
// running and what cannot be undone.
//
// Every entry carries the real icon, pulled through the same icons module the surfaces themselves use.
// Never a description in words where the control is a picture; never a similar-looking emoji standing in
// for one. A guide showing something other than the icon is worse than no guide.
//
// It is deliberately short. Anything a control says for itself (its tooltip, its own label) is left to
// the control; a guide nobody finishes explains nothing.
import donateIcon from '../assets/donate.png'
import {folderIcon, icons} from '../icons'

// GuideIconEntry is one named piece of furniture: its picture, what it is called and what it does.
export interface GuideIconEntry {
    icon: string
    name: string
    text: string
}

// GuideRule is one of the rules behind the behaviour: a claim in bold, then what it means in practice.
export interface GuideRule {
    title: string
    text: string
}

// GuideSection is one block of the document. A section carries icon entries, rules or plain paragraphs;
// the dialog draws whichever are present, in that order.
export interface GuideSection {
    heading: string
    intro?: string
    entries?: GuideIconEntry[]
    rules?: GuideRule[]
    paragraphs?: string[]
}

export const guideSections: GuideSection[] = [
    {
        heading: 'The bar along the top',
        intro: 'The menus and the working controls read left to right; the theme toggle and Help are held at the far end. Hover any of them to see its name.',
        entries: [
            {icon: icons.file, name: 'File', text: 'save the open message as a file or print it.'},
            {icon: icons.edit, name: 'Edit', text: 'undo and redo the mail actions, cut, copy and paste messages between folders, delete, search, filter rules and message templates. Each entry names what it will unwind.'},
            {icon: icons.view, name: 'View', text: 'conversation view, the unified mailbox, the reading pane (F8) and whether remote images load by default.'},
            {icon: icons.rules, name: 'Filter rules', text: 'the rules that sort arriving mail, with Export and Import to carry a set between installations.'},
            {icon: icons.template, name: 'Message templates', text: 'reusable subjects, bodies and attachments, inserted from the compose window.'},
            {icon: icons.mail, name: 'Mail', text: 'everything you can do to the message you are on: respond, snooze, tag, move, copy, junk, plus Add account and Sync.'},
            {icon: icons.compose, name: 'Compose', text: 'a new message (Ctrl+N). Disabled until an account is selected.'},
            {icon: icons.addAccount, name: 'Add account', text: 'the two-step setup wizard.'},
            {icon: icons.sync, name: 'Sync', text: 'fetch now for the account you are in (F9). It greys out while a sync is running.'},
            {icon: icons.contacts, name: 'Contacts', text: 'the address book, with vCard and CSV import and export.'},
            {icon: icons.calendar, name: 'Calendar', text: 'month, week and day views, reminders, invitations and ICS import and export.'},
            {icon: icons.darkMode, name: 'Light or dark', text: 'the toggle shows the mode it will switch INTO, so the moon appears while the app is light.'},
            {icon: icons.help, name: 'Help', text: 'this guide, About, the licence and Check for Updates.'},
        ],
    },
    {
        heading: 'The folder list',
        intro: 'One folder each holds the roles below; every other folder and every subfolder carries the plain folder mark. Unread counts badge the folder, the account and the total.',
        entries: [
            {icon: folderIcon.inbox, name: 'Inbox', text: 'arriving mail for the account you are in.'},
            {icon: folderIcon.sent, name: 'Sent', text: 'what you have sent, including replies to invitations.'},
            {icon: folderIcon.drafts, name: 'Drafts', text: 'saved drafts; open one to carry on writing exactly where you left it.'},
            {icon: folderIcon.outbox, name: 'Outbox', text: 'messages waiting to leave: a scheduled send or anything written while offline. Cancel send is on the Mail menu.'},
            {icon: folderIcon.archive, name: 'Archive', text: 'out of the way rather than deleted. It is the one folder that never badges, because archiving is how a message stops asking for attention.'},
            {icon: folderIcon.junk, name: 'Junk', text: 'marked junk here or by a rule. Not junk moves a message back and tells the server it was wrong.'},
            {icon: folderIcon.trash, name: 'Trash', text: 'deleted messages. Delete permanently skips it entirely.'},
            {icon: folderIcon.custom, name: 'Your own folders', text: 'the plus beside the Folders heading makes one; New subfolder on a folder makes one inside it. Drag a folder to nest it or reorder it; drag messages onto any folder to move them.'},
            {icon: icons.mail, name: 'All inboxes', text: 'appears above the account picker when the unified mailbox is on: every account in one list, each row dotted with its account colour. A reply is sent from the account that row belongs to.'},
            {icon: icons.snooze, name: 'Snoozed', text: 'appears while anything is snoozed: what is hidden, when each is due back and an Unsnooze.'},
        ],
    },
    {
        heading: 'The strip along the foot',
        entries: [
            {icon: donateIcon, name: 'Buy the author a drink', text: 'opens a donation page in your own browser. Nothing in PigeonPost is held back behind it.'},
        ],
    },
    {
        heading: 'Rules behind the behaviour',
        rules: [
            {
                title: 'Your mail is held on this machine.',
                text: 'Folders and message summaries are cached to a local database and read offline; a body is fetched when you first open it, then cached too. Passwords live in the operating system keychain, never in that database.',
            },
            {
                title: 'A message is shown, never run.',
                text: 'HTML renders in a sandboxed frame that keeps the styling the sender wrote while running no scripts and fetching nothing remote. Remote images stay blocked until you load them, per message or through the View menu; links open in your browser rather than inside the app.',
            },
            {
                title: 'Delete permanently means exactly that.',
                text: 'It is removed on the server, never cached and not recoverable; ordinary Delete moves the message to Trash, where it can still be fetched back.',
            },
            {
                title: 'Rules act on mail that arrives after they exist.',
                text: 'Unattended, a rule runs on the Inbox as mail comes in, so adding one never reaches back over the mail you already have. The Now button on a rule is how you ask it to: it says what it found and acts only once you agree.',
            },
            {
                title: 'A scheduled thing needs the app.',
                text: 'A snoozed message, a send later and a calendar reminder each fire while PigeonPost is running, else at the next launch after their time. Nothing is lost by closing the window; it simply waits.',
            },
            {
                title: 'The only thing fetched that is not your mail is the version check.',
                text: 'PigeonPost asks GitHub whether a newer release has been published, shortly after launch and once a day; Help > Check for Updates asks on demand. The request carries nothing about you or your mail and a version you would rather not hear about again can be skipped.',
            },
        ],
    },
    {
        heading: 'Keyboard',
        paragraphs: [
            'Tab and Shift+Tab move through the window, wrapping at both ends; Up and Down walk the message list and Enter opens what is focused. A teal ring marks the focused control.',
            'Ctrl+N composes, Ctrl+R and Ctrl+Shift+R reply, Ctrl+L forwards, Ctrl+K jumps to the search box, F8 shows or hides the reading pane and F9 syncs. Every accelerator is printed beside its menu entry, so the menus are the reference.',
            'After ten seconds without input the window settles back on the Inbox of the account you are in, so it always resumes from a known place. It never does so while a dialog is open, while you are typing in a field or while a message is open in the reader.',
        ],
    },
]
