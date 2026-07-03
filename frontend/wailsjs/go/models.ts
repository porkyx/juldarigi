export namespace main {
	
	export class ScrapeRequest {
	    url: string;
	    pages: number;
	    startDate: string;
	    endDate: string;
	
	    static createFrom(source: any = {}) {
	        return new ScrapeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.pages = source["pages"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	    }
	}
	export class UserStat {
	    uid: string;
	    nickname: string;
	    ip: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new UserStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.nickname = source["nickname"];
	        this.ip = source["ip"];
	        this.count = source["count"];
	    }
	}
	export class ScrapeResult {
	    success: boolean;
	    type: string;
	    galleryId: string;
	    galleryType: string;
	    url: string;
	    pagesScraped: number;
	    startDate?: string;
	    endDate?: string;
	    totalPosts: number;
	    uniqueUsers: number;
	    userStats: UserStat[];
	
	    static createFrom(source: any = {}) {
	        return new ScrapeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.type = source["type"];
	        this.galleryId = source["galleryId"];
	        this.galleryType = source["galleryType"];
	        this.url = source["url"];
	        this.pagesScraped = source["pagesScraped"];
	        this.startDate = source["startDate"];
	        this.endDate = source["endDate"];
	        this.totalPosts = source["totalPosts"];
	        this.uniqueUsers = source["uniqueUsers"];
	        this.userStats = this.convertValues(source["userStats"], UserStat);
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

}

