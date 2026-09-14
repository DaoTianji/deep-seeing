import {cleanup,render,screen} from "@testing-library/react";
import {afterEach,expect,it,vi} from "vitest";
import {ReadingExample} from "./ReadingExample";
afterEach(()=>{cleanup();vi.unstubAllGlobals()});
it("labels the historical sample and never generates or saves a visitor record",()=>{const fetch=vi.fn();vi.stubGlobal("fetch",fetch);render(<ReadingExample/>);expect(screen.getByText(/不是现场生成/)).toBeInTheDocument();expect(screen.getByRole("heading",{name:"五百法郎的清晨"})).toBeInTheDocument();expect(screen.getByRole("heading",{name:/原著故事线/})).toBeInTheDocument();expect(screen.getByRole("link",{name:/自己尝试/})).toHaveAttribute("href","/reading?book=necklace&mode=story");expect(fetch).not.toHaveBeenCalled()});
