// The address-book half of the Wails seam: the contact and contact-group types and the calls that read,
// write, import, export and collect contacts. It lives beside api.ts rather than inside it for the reason
// the filter rules do (see apiRules): a cohesive group of calls with its own types, taken out of a module
// that was over the size limit. The api object spreads what is exported here, so callers still reach
// these through api.* and nothing else changes.
import {
    CollectContacts,
    DeleteContact,
    DeleteContactGroup,
    ExportContactsToFile,
    GetContact,
    ImportContactsFromFile,
    ListContactGroups,
    ListContacts,
    SaveContact,
    SaveContactGroup,
} from '../wailsjs/go/main/App'
import {main} from '../wailsjs/go/models'

export type Contact = main.ContactDTO
export type ContactGroup = main.ContactGroupDTO
// ContactImportResult separates records stored as new contacts from those merged into existing ones,
// so the UI can say what an import actually changed rather than implying every row was new.
export type ContactImportResult = main.ContactImportResult

export interface ContactEmailInput {
    label: string
    address: string
}

export interface ContactPhoneInput {
    label: string
    number: string
}

export interface ContactAddressInput {
    label: string
    street: string
    locality: string
    region: string
    postalCode: string
    country: string
}

export interface ContactInput {
    id: string
    uid: string
    formattedName: string
    givenName: string
    familyName: string
    organization: string
    title: string
    note: string
    birthday: string
    emails: ContactEmailInput[]
    phones: ContactPhoneInput[]
    addresses: ContactAddressInput[]
}

export interface ContactGroupInput {
    id: string
    name: string
    members: string[]
}

export const contactsApi = {
    listContacts: (): Promise<Contact[]> => ListContacts(),
    getContact: (id: string): Promise<Contact> => GetContact(id),
    saveContact: (req: ContactInput): Promise<void> => SaveContact(main.ContactRequest.createFrom(req)),
    deleteContact: (id: string): Promise<void> => DeleteContact(id),
    // collectContacts adds a minimal contact for each address not already in the address book,
    // returning how many were added; called after a successful send when auto-collect is on.
    collectContacts: (addresses: string[]): Promise<number> => CollectContacts(addresses),
    listContactGroups: (): Promise<ContactGroup[]> => ListContactGroups(),
    saveContactGroup: (req: ContactGroupInput): Promise<void> =>
        SaveContactGroup(main.ContactGroupRequest.createFrom(req)),
    deleteContactGroup: (id: string): Promise<void> => DeleteContactGroup(id),
    importContactsFromFile: (): Promise<ContactImportResult> => ImportContactsFromFile(),
    exportContactsToFile: (format: string): Promise<boolean> => ExportContactsToFile(format),
}
