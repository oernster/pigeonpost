export namespace main {
	
	export class StateDTO {
	    mode: string;
	    installed: boolean;
	    installedVersion: string;
	    thisVersion: string;
	    installDir: string;
	    launchOnBoot: boolean;
	    upgradeAvailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StateDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.installed = source["installed"];
	        this.installedVersion = source["installedVersion"];
	        this.thisVersion = source["thisVersion"];
	        this.installDir = source["installDir"];
	        this.launchOnBoot = source["launchOnBoot"];
	        this.upgradeAvailable = source["upgradeAvailable"];
	    }
	}

}

