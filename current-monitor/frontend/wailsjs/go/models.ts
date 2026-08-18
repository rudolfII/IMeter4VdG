export namespace main {
	
	export class DeviceState {
	    latest_current: number;
	    unit: string;
	    last_raw_response: string;
	    last_update: number;
	
	    static createFrom(source: any = {}) {
	        return new DeviceState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latest_current = source["latest_current"];
	        this.unit = source["unit"];
	        this.last_raw_response = source["last_raw_response"];
	        this.last_update = source["last_update"];
	    }
	}

}

