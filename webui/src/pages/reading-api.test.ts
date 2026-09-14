import {afterEach,expect,it,vi} from "vitest";
import {request} from "./reading-api";
import {setActiveReader,READER_CHANGED} from "./reading-session";
afterEach(()=>{setActiveReader("");vi.unstubAllGlobals();window.history.replaceState({},"","/");});
it("binds requests to the active name and rejects stale sessions",async()=>{
 setActiveReader("reader-a");const listener=vi.fn();window.addEventListener(READER_CHANGED,listener);
 const fetcher=vi.fn().mockResolvedValue({ok:false,status:409,text:async()=>JSON.stringify({code:"reader_changed",error:"名字已变化"})});vi.stubGlobal("fetch",fetcher);
 await expect(request("/records")).rejects.toThrow("名字已变化");
 expect(fetcher.mock.calls[0][1].headers["X-Reading-Profile"]).toBe("reader-a");expect(listener).toHaveBeenCalledOnce();window.removeEventListener(READER_CHANGED,listener);
});
it("scopes all book requests without dropping character parameters",async()=>{
 window.history.replaceState({},"","/reading?book=magi");
 const fetcher=vi.fn().mockResolvedValue({ok:true,text:async()=>'{}'});vi.stubGlobal("fetch",fetcher);
 await request("/branches/example/snapshot?character=della");
 expect(fetcher.mock.calls[0][0]).toBe("/api/story/branches/example/snapshot?character=della&book=magi");
 await request("/branches",{scene:1});
 expect(fetcher.mock.calls[1][0]).toBe("/api/story/branches?book=magi");
});
it("preserves legacy endpoints without a selection",async()=>{
 const fetcher=vi.fn().mockResolvedValue({ok:true,text:async()=>'{}'});vi.stubGlobal("fetch",fetcher);
 await request("/book");
 expect(fetcher.mock.calls[0][0]).toBe("/api/story/book");
});
