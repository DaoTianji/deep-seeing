let activeReader = "";
export const READER_CHANGED = "reading-reader-changed";
export const READER_STORAGE_KEY = "reading-session-change";
export function setActiveReader(id: string) { activeReader = id; }
export function readerHeaders(): Record<string,string> { return activeReader ? {"X-Reading-Profile":activeReader} : {}; }
export function readerCacheKey(book: string, id=activeReader) { return id ? `reading-${id}-${book}-branch` : `reading-${book}-branch`; }
export function checkReaderResponse(status:number) {
 if(status===401||status===409) window.dispatchEvent(new Event(READER_CHANGED));
}
export function announceReaderChange() {
 try { localStorage.setItem(READER_STORAGE_KEY,crypto.randomUUID()); } catch { /* Cookie-based login still works when local storage is disabled. */ }
}
