export namespace main {
	
	export class ChapterInfo {
	    title: string;
	    url: string;
	    content: string;
	    raw_html: string;
	    full_page_html: string;
	
	    static createFrom(source: any = {}) {
	        return new ChapterInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.url = source["url"];
	        this.content = source["content"];
	        this.raw_html = source["raw_html"];
	        this.full_page_html = source["full_page_html"];
	    }
	}
	export class ScrapeResult {
	    page_type: string;
	    title: string;
	    author: string;
	    raw_html: string[];
	    text_content: string[];
	    full_page_html: string;
	    index_pages_html: string[];
	    chapters?: ChapterInfo[];
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ScrapeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.page_type = source["page_type"];
	        this.title = source["title"];
	        this.author = source["author"];
	        this.raw_html = source["raw_html"];
	        this.text_content = source["text_content"];
	        this.full_page_html = source["full_page_html"];
	        this.index_pages_html = source["index_pages_html"];
	        this.chapters = this.convertValues(source["chapters"], ChapterInfo);
	        this.error = source["error"];
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
	export class Settings {
	    url: string;
	    savePath: string;
	    encoding: string;
	    lineEnding: string;
	    createHtml: boolean;
	    createTxt: boolean;
	    createCombined: boolean;
	    showInFront: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.savePath = source["savePath"];
	        this.encoding = source["encoding"];
	        this.lineEnding = source["lineEnding"];
	        this.createHtml = source["createHtml"];
	        this.createTxt = source["createTxt"];
	        this.createCombined = source["createCombined"];
	        this.showInFront = source["showInFront"];
	    }
	}

}

