export namespace main {
	
	export class DeviceInfo {
	    latest_current: number;
	    unit: string;
	    last_raw_response: string;
	    last_update: number;
	    device_id: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.latest_current = source["latest_current"];
	        this.unit = source["unit"];
	        this.last_raw_response = source["last_raw_response"];
	        this.last_update = source["last_update"];
	        this.device_id = source["device_id"];
	    }
	}

}

