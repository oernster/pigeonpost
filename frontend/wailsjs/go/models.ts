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
	    }
	}
	export class AccountSetupRequest {
	    displayName: string;
	    email: string;
	    password: string;
	    inHost: string;
	    inPort: number;
	    inSecurity: string;
	    outHost: string;
	    outPort: number;
	    outSecurity: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountSetupRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.displayName = source["displayName"];
	        this.email = source["email"];
	        this.password = source["password"];
	        this.inHost = source["inHost"];
	        this.inPort = source["inPort"];
	        this.inSecurity = source["inSecurity"];
	        this.outHost = source["outHost"];
	        this.outPort = source["outPort"];
	        this.outSecurity = source["outSecurity"];
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
	export class ComposeRequest {
	    accountId: string;
	    to: string[];
	    cc: string[];
	    bcc: string[];
	    subject: string;
	    body: string;
	    htmlBody: string;
	    attachmentPaths: string[];
	    attachmentMessageIds: string[];
	
	    static createFrom(source: any = {}) {
	        return new ComposeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.to = source["to"];
	        this.cc = source["cc"];
	        this.bcc = source["bcc"];
	        this.subject = source["subject"];
	        this.body = source["body"];
	        this.htmlBody = source["htmlBody"];
	        this.attachmentPaths = source["attachmentPaths"];
	        this.attachmentMessageIds = source["attachmentMessageIds"];
	    }
	}
	
	export class OutboxItemDTO {
	    id: string;
	    kind: string;
	    subject: string;
	    to: string[];
	    createdMs: number;

	    static createFrom(source: any = {}) {
	        return new OutboxItemDTO(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.subject = source["subject"];
	        this.to = source["to"];
	        this.createdMs = source["createdMs"];
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
	export class MessageBodyDTO {
	    plain: string;
	    html: string;
	
	    static createFrom(source: any = {}) {
	        return new MessageBodyDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.plain = source["plain"];
	        this.html = source["html"];
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
	    snippet: string;
	
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
	export class RuleDTO {
	    id: string;
	    name: string;
	    field: string;
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
	        this.contains = source["contains"];
	        this.action = source["action"];
	    }
	}
	export class RuleRequest {
	    id: string;
	    name: string;
	    field: string;
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
	        this.contains = source["contains"];
	        this.action = source["action"];
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

}

