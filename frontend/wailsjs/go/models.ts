export namespace main {
	
	export class CreditDTO {
	    name: string;
	    licence: string;
	
	    static createFrom(source: any = {}) {
	        return new CreditDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.licence = source["licence"];
	    }
	}
	export class AboutDTO {
	    name: string;
	    tagline: string;
	    version: string;
	    author: string;
	    copyright: string;
	    licence: string;
	    credits: CreditDTO[];
	
	    static createFrom(source: any = {}) {
	        return new AboutDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.tagline = source["tagline"];
	        this.version = source["version"];
	        this.author = source["author"];
	        this.copyright = source["copyright"];
	        this.licence = source["licence"];
	        this.credits = this.convertValues(source["credits"], CreditDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AddressDTO {
	    name: string;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new AddressDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.address = source["address"];
	    }
	}
	export class AccountDTO {
	    id: string;
	    displayName: string;
	    email: string;
	    protocol: string;
	    inHost: string;
	    inPort: number;
	    inSecurity: string;
	    outHost: string;
	    outPort: number;
	    outSecurity: string;
	    signature: string;
	    auth: string;
	    identities: AddressDTO[];
	
	    static createFrom(source: any = {}) {
	        return new AccountDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.email = source["email"];
	        this.protocol = source["protocol"];
	        this.inHost = source["inHost"];
	        this.inPort = source["inPort"];
	        this.inSecurity = source["inSecurity"];
	        this.outHost = source["outHost"];
	        this.outPort = source["outPort"];
	        this.outSecurity = source["outSecurity"];
	        this.signature = source["signature"];
	        this.auth = source["auth"];
	        this.identities = this.convertValues(source["identities"], AddressDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class IdentityInput {
	    name: string;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new IdentityInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.address = source["address"];
	    }
	}
	export class AccountProfileRequest {
	    email: string;
	    displayName: string;
	    signature: string;
	    identities: IdentityInput[];
	
	    static createFrom(source: any = {}) {
	        return new AccountProfileRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.email = source["email"];
	        this.displayName = source["displayName"];
	        this.signature = source["signature"];
	        this.identities = this.convertValues(source["identities"], IdentityInput);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AccountSetupRequest {
	    displayName: string;
	    email: string;
	    password: string;
	    protocol: string;
	    inHost: string;
	    inPort: number;
	    inSecurity: string;
	    outHost: string;
	    outPort: number;
	    outSecurity: string;
	    signature: string;
	    identities: IdentityInput[];
	
	    static createFrom(source: any = {}) {
	        return new AccountSetupRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.displayName = source["displayName"];
	        this.email = source["email"];
	        this.password = source["password"];
	        this.protocol = source["protocol"];
	        this.inHost = source["inHost"];
	        this.inPort = source["inPort"];
	        this.inSecurity = source["inSecurity"];
	        this.outHost = source["outHost"];
	        this.outPort = source["outPort"];
	        this.outSecurity = source["outSecurity"];
	        this.signature = source["signature"];
	        this.identities = this.convertValues(source["identities"], IdentityInput);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class AttachmentDTO {
	    index: number;
	    filename: string;
	    contentType: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new AttachmentDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.filename = source["filename"];
	        this.contentType = source["contentType"];
	        this.size = source["size"];
	    }
	}
	export class AttendeeDTO {
	    address: string;
	    commonName: string;
	    role: string;
	    status: string;
	    rsvp: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AttendeeDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.commonName = source["commonName"];
	        this.role = source["role"];
	        this.status = source["status"];
	        this.rsvp = source["rsvp"];
	    }
	}
	export class BulkResultDTO {
	    ids: string[];
	    failed: number;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new BulkResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ids = source["ids"];
	        this.failed = source["failed"];
	        this.error = source["error"];
	    }
	}
	export class CalDAVAccountDTO {
	    id: string;
	    displayName: string;
	    baseUrl: string;
	    username: string;
	
	    static createFrom(source: any = {}) {
	        return new CalDAVAccountDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.baseUrl = source["baseUrl"];
	        this.username = source["username"];
	    }
	}
	export class CalendarDTO {
	    id: string;
	    name: string;
	    colour: string;
	
	    static createFrom(source: any = {}) {
	        return new CalendarDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.colour = source["colour"];
	    }
	}
	export class CalendarRequest {
	    id: string;
	    name: string;
	    colour: string;
	
	    static createFrom(source: any = {}) {
	        return new CalendarRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.colour = source["colour"];
	    }
	}
	export class ComposeRequest {
	    accountId: string;
	    from: string;
	    to: string[];
	    cc: string[];
	    bcc: string[];
	    subject: string;
	    body: string;
	    htmlBody: string;
	    attachmentPaths: string[];
	    attachmentMessageIds: string[];
	    holdSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new ComposeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.cc = source["cc"];
	        this.bcc = source["bcc"];
	        this.subject = source["subject"];
	        this.body = source["body"];
	        this.htmlBody = source["htmlBody"];
	        this.attachmentPaths = source["attachmentPaths"];
	        this.attachmentMessageIds = source["attachmentMessageIds"];
	        this.holdSeconds = source["holdSeconds"];
	    }
	}
	export class ContactAddressDTO {
	    label: string;
	    street: string;
	    locality: string;
	    region: string;
	    postalCode: string;
	    country: string;
	
	    static createFrom(source: any = {}) {
	        return new ContactAddressDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.street = source["street"];
	        this.locality = source["locality"];
	        this.region = source["region"];
	        this.postalCode = source["postalCode"];
	        this.country = source["country"];
	    }
	}
	export class ContactPhoneDTO {
	    label: string;
	    number: string;
	
	    static createFrom(source: any = {}) {
	        return new ContactPhoneDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.number = source["number"];
	    }
	}
	export class ContactEmailDTO {
	    label: string;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new ContactEmailDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.address = source["address"];
	    }
	}
	export class ContactDTO {
	    id: string;
	    uid: string;
	    formattedName: string;
	    givenName: string;
	    familyName: string;
	    organization: string;
	    title: string;
	    note: string;
	    birthday: string;
	    emails: ContactEmailDTO[];
	    phones: ContactPhoneDTO[];
	    addresses: ContactAddressDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ContactDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.uid = source["uid"];
	        this.formattedName = source["formattedName"];
	        this.givenName = source["givenName"];
	        this.familyName = source["familyName"];
	        this.organization = source["organization"];
	        this.title = source["title"];
	        this.note = source["note"];
	        this.birthday = source["birthday"];
	        this.emails = this.convertValues(source["emails"], ContactEmailDTO);
	        this.phones = this.convertValues(source["phones"], ContactPhoneDTO);
	        this.addresses = this.convertValues(source["addresses"], ContactAddressDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ContactGroupDTO {
	    id: string;
	    name: string;
	    members: string[];
	
	    static createFrom(source: any = {}) {
	        return new ContactGroupDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.members = source["members"];
	    }
	}
	export class ContactGroupRequest {
	    id: string;
	    name: string;
	    members: string[];
	
	    static createFrom(source: any = {}) {
	        return new ContactGroupRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.members = source["members"];
	    }
	}
	
	export class ContactRequest {
	    id: string;
	    uid: string;
	    formattedName: string;
	    givenName: string;
	    familyName: string;
	    organization: string;
	    title: string;
	    note: string;
	    birthday: string;
	    emails: ContactEmailDTO[];
	    phones: ContactPhoneDTO[];
	    addresses: ContactAddressDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ContactRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.uid = source["uid"];
	        this.formattedName = source["formattedName"];
	        this.givenName = source["givenName"];
	        this.familyName = source["familyName"];
	        this.organization = source["organization"];
	        this.title = source["title"];
	        this.note = source["note"];
	        this.birthday = source["birthday"];
	        this.emails = this.convertValues(source["emails"], ContactEmailDTO);
	        this.phones = this.convertValues(source["phones"], ContactPhoneDTO);
	        this.addresses = this.convertValues(source["addresses"], ContactAddressDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class DraftRecoveryDTO {
	    present: boolean;
	    accountId: string;
	    to: string;
	    cc: string;
	    bcc: string;
	    subject: string;
	    bodyHtml: string;
	    savedMs: number;
	
	    static createFrom(source: any = {}) {
	        return new DraftRecoveryDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.present = source["present"];
	        this.accountId = source["accountId"];
	        this.to = source["to"];
	        this.cc = source["cc"];
	        this.bcc = source["bcc"];
	        this.subject = source["subject"];
	        this.bodyHtml = source["bodyHtml"];
	        this.savedMs = source["savedMs"];
	    }
	}
	export class DraftRecoveryRequest {
	    accountId: string;
	    to: string;
	    cc: string;
	    bcc: string;
	    subject: string;
	    bodyHtml: string;
	
	    static createFrom(source: any = {}) {
	        return new DraftRecoveryRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.to = source["to"];
	        this.cc = source["cc"];
	        this.bcc = source["bcc"];
	        this.subject = source["subject"];
	        this.bodyHtml = source["bodyHtml"];
	    }
	}
	export class EmailView {
	    subject: string;
	    from: string;
	    to: string;
	    date: string;
	    html: string;
	    plain: string;
	
	    static createFrom(source: any = {}) {
	        return new EmailView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subject = source["subject"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.date = source["date"];
	        this.html = source["html"];
	        this.plain = source["plain"];
	    }
	}
	export class OrganizerDTO {
	    address: string;
	    commonName: string;
	
	    static createFrom(source: any = {}) {
	        return new OrganizerDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.address = source["address"];
	        this.commonName = source["commonName"];
	    }
	}
	export class EventDTO {
	    id: string;
	    uid: string;
	    calendarId: string;
	    summary: string;
	    description: string;
	    location: string;
	    category: string;
	    start: string;
	    end: string;
	    allDay: boolean;
	    recurrence: string;
	    timeZone: string;
	    reminders: number[];
	    extra: string;
	    organizer: OrganizerDTO;
	    attendees: AttendeeDTO[];
	
	    static createFrom(source: any = {}) {
	        return new EventDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.uid = source["uid"];
	        this.calendarId = source["calendarId"];
	        this.summary = source["summary"];
	        this.description = source["description"];
	        this.location = source["location"];
	        this.category = source["category"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.allDay = source["allDay"];
	        this.recurrence = source["recurrence"];
	        this.timeZone = source["timeZone"];
	        this.reminders = source["reminders"];
	        this.extra = source["extra"];
	        this.organizer = this.convertValues(source["organizer"], OrganizerDTO);
	        this.attendees = this.convertValues(source["attendees"], AttendeeDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EventInstanceDTO {
	    event: EventDTO;
	    start: string;
	    end: string;
	    recurrenceId: string;
	
	    static createFrom(source: any = {}) {
	        return new EventInstanceDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.event = this.convertValues(source["event"], EventDTO);
	        this.start = source["start"];
	        this.end = source["end"];
	        this.recurrenceId = source["recurrenceId"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EventRequest {
	    id: string;
	    uid: string;
	    calendarId: string;
	    summary: string;
	    description: string;
	    location: string;
	    category: string;
	    start: string;
	    end: string;
	    allDay: boolean;
	    recurrence: string;
	    timeZone: string;
	    reminders: number[];
	    extra: string;
	    organizer: OrganizerDTO;
	    attendees: AttendeeDTO[];
	
	    static createFrom(source: any = {}) {
	        return new EventRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.uid = source["uid"];
	        this.calendarId = source["calendarId"];
	        this.summary = source["summary"];
	        this.description = source["description"];
	        this.location = source["location"];
	        this.category = source["category"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.allDay = source["allDay"];
	        this.recurrence = source["recurrence"];
	        this.timeZone = source["timeZone"];
	        this.reminders = source["reminders"];
	        this.extra = source["extra"];
	        this.organizer = this.convertValues(source["organizer"], OrganizerDTO);
	        this.attendees = this.convertValues(source["attendees"], AttendeeDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FolderDTO {
	    id: string;
	    accountId: string;
	    path: string;
	    name: string;
	    kind: string;
	    unread: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new FolderDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.path = source["path"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.unread = source["unread"];
	        this.total = source["total"];
	    }
	}
	
	export class InvitationDTO {
	    method: string;
	    event: EventDTO;
	    me: string;
	    myStatus: string;
	    organizer: OrganizerDTO;
	    attendees: AttendeeDTO[];
	
	    static createFrom(source: any = {}) {
	        return new InvitationDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.method = source["method"];
	        this.event = this.convertValues(source["event"], EventDTO);
	        this.me = source["me"];
	        this.myStatus = source["myStatus"];
	        this.organizer = this.convertValues(source["organizer"], OrganizerDTO);
	        this.attendees = this.convertValues(source["attendees"], AttendeeDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MessageBodyDTO {
	    plain: string;
	    html: string;
	    hasInvite: boolean;
	    attachments: AttachmentDTO[];
	
	    static createFrom(source: any = {}) {
	        return new MessageBodyDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.plain = source["plain"];
	        this.html = source["html"];
	        this.hasInvite = source["hasInvite"];
	        this.attachments = this.convertValues(source["attachments"], AttachmentDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MessageDTO {
	    id: string;
	    folderId: string;
	    subject: string;
	    fromName: string;
	    fromAddress: string;
	    to: AddressDTO[];
	    cc: AddressDTO[];
	    date: string;
	    size: number;
	    read: boolean;
	    flagged: boolean;
	    hasAttachments: boolean;
	    answered: boolean;
	    forwarded: boolean;
	    snippet: string;
	    tagColours: string[];
	
	    static createFrom(source: any = {}) {
	        return new MessageDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.folderId = source["folderId"];
	        this.subject = source["subject"];
	        this.fromName = source["fromName"];
	        this.fromAddress = source["fromAddress"];
	        this.to = this.convertValues(source["to"], AddressDTO);
	        this.cc = this.convertValues(source["cc"], AddressDTO);
	        this.date = source["date"];
	        this.size = source["size"];
	        this.read = source["read"];
	        this.flagged = source["flagged"];
	        this.hasAttachments = source["hasAttachments"];
	        this.answered = source["answered"];
	        this.forwarded = source["forwarded"];
	        this.snippet = source["snippet"];
	        this.tagColours = source["tagColours"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MessagePageDTO {
	    messages: MessageDTO[];
	    hasMore: boolean;
	    nextCursorDateMs: number;
	    nextCursorId: string;
	
	    static createFrom(source: any = {}) {
	        return new MessagePageDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = this.convertValues(source["messages"], MessageDTO);
	        this.hasMore = source["hasMore"];
	        this.nextCursorDateMs = source["nextCursorDateMs"];
	        this.nextCursorId = source["nextCursorId"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class OutboxItemDTO {
	    id: string;
	    accountId: string;
	    kind: string;
	    subject: string;
	    to: string[];
	    body: string;
	    createdMs: number;
	    holdMs: number;
	    failed: boolean;
	    failure: string;
	
	    static createFrom(source: any = {}) {
	        return new OutboxItemDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.kind = source["kind"];
	        this.subject = source["subject"];
	        this.to = source["to"];
	        this.body = source["body"];
	        this.createdMs = source["createdMs"];
	        this.holdMs = source["holdMs"];
	        this.failed = source["failed"];
	        this.failure = source["failure"];
	    }
	}
	export class RuleDTO {
	    id: string;
	    name: string;
	    field: string;
	    operator: string;
	    contains: string;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new RuleDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.field = source["field"];
	        this.operator = source["operator"];
	        this.contains = source["contains"];
	        this.action = source["action"];
	    }
	}
	export class RuleRequest {
	    id: string;
	    name: string;
	    field: string;
	    operator: string;
	    contains: string;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new RuleRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.field = source["field"];
	        this.operator = source["operator"];
	        this.contains = source["contains"];
	        this.action = source["action"];
	    }
	}
	export class SearchHitDTO {
	    message: MessageDTO;
	    snippet: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchHitDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = this.convertValues(source["message"], MessageDTO);
	        this.snippet = source["snippet"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SearchResultDTO {
	    hits: SearchHitDTO[];
	    degraded: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SearchResultDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hits = this.convertValues(source["hits"], SearchHitDTO);
	        this.degraded = source["degraded"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TagDTO {
	    id: string;
	    name: string;
	    colour: string;
	
	    static createFrom(source: any = {}) {
	        return new TagDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.colour = source["colour"];
	    }
	}
	export class TagRequest {
	    id: string;
	    name: string;
	    colour: string;
	
	    static createFrom(source: any = {}) {
	        return new TagRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.colour = source["colour"];
	    }
	}
	export class TemplateDTO {
	    id: string;
	    name: string;
	    subject: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new TemplateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.subject = source["subject"];
	        this.body = source["body"];
	    }
	}
	export class TemplateRequest {
	    id: string;
	    name: string;
	    subject: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new TemplateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.subject = source["subject"];
	        this.body = source["body"];
	    }
	}
	export class ThreadDTO {
	    subject: string;
	    count: number;
	    unreadCount: number;
	    messages: MessageDTO[];
	
	    static createFrom(source: any = {}) {
	        return new ThreadDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subject = source["subject"];
	        this.count = source["count"];
	        this.unreadCount = source["unreadCount"];
	        this.messages = this.convertValues(source["messages"], MessageDTO);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UnreadCountsDTO {
	    total: number;
	    byAccount: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new UnreadCountsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.byAccount = source["byAccount"];
	    }
	}

}

