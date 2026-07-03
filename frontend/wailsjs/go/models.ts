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
	
	    static createFrom(source: any = {}) {
	        return new AccountDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.displayName = source["displayName"];
	        this.email = source["email"];
	        this.protocol = source["protocol"];
	    }
	}
	export class ComposeRequest {
	    accountId: string;
	    to: string[];
	    cc: string[];
	    subject: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new ComposeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.to = source["to"];
	        this.cc = source["cc"];
	        this.subject = source["subject"];
	        this.body = source["body"];
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
	export class MessageDTO {
	    id: string;
	    folderId: string;
	    subject: string;
	    fromName: string;
	    fromAddress: string;
	    date: string;
	    size: number;
	    read: boolean;
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
	        this.date = source["date"];
	        this.size = source["size"];
	        this.read = source["read"];
	        this.hasAttachments = source["hasAttachments"];
	        this.snippet = source["snippet"];
	    }
	}

}

