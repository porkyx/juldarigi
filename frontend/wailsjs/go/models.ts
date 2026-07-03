export namespace main {

	export class MetricRank {
	    rank: number;
	    uid: string;
	    nickname: string;
	    ip: string;
	    value: number;
	    postNumber: string;
	    postTitle: string;
	    postUrl: string;

	    static createFrom(source: any = {}) {
	        return new MetricRank(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rank = source["rank"];
	        this.uid = source["uid"];
	        this.nickname = source["nickname"];
	        this.ip = source["ip"];
	        this.value = source["value"];
	        this.postNumber = source["postNumber"];
	        this.postTitle = source["postTitle"];
	        this.postUrl = source["postUrl"];
	    }
	}
	export class MetricRankings {
	    views: MetricRank[];
	    recommendations: MetricRank[];
	    comments: MetricRank[];

	    static createFrom(source: any = {}) {
	        return new MetricRankings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.views = this.convertValues(source["views"], MetricRank);
	        this.recommendations = this.convertValues(source["recommendations"], MetricRank);
	        this.comments = this.convertValues(source["comments"], MetricRank);
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
	export class SaveCaptureRequest {
	    dataUrl: string;
	    defaultFilename: string;

	    static createFrom(source: any = {}) {
	        return new SaveCaptureRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataUrl = source["dataUrl"];
	        this.defaultFilename = source["defaultFilename"];
	    }
	}
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
	    topMetrics: MetricRankings;

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
	        this.topMetrics = this.convertValues(source["topMetrics"], MetricRankings);
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

