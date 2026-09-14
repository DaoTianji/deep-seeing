import {cleanup, render, screen} from "@testing-library/react";
import {afterEach, expect, it} from "vitest";
import {CompanionCost} from "./CompanionCost";

afterEach(cleanup);
it("shows complete gateway receipts without estimating money",()=>{
 render(<CompanionCost metrics={{duration_ms:1250,model_calls:2,reported_calls:2,usage:{prompt_tokens:30,completion_tokens:10,total_tokens:40}}}/>);
 expect(screen.getByText(/1.3 秒 · 2 次模型调用/)).toBeInTheDocument();
 expect(screen.getByText(/合计 40 Token/)).toBeInTheDocument();
});
it("does not label missing or partial receipts as a full total",()=>{
 render(<CompanionCost metrics={{duration_ms:50,model_calls:3,reported_calls:1,usage:{prompt_tokens:30,completion_tokens:10,total_tokens:40}}}/>);
 expect(screen.getByText(/Token 未完整返回（1\/3 次）/)).toBeInTheDocument();
 expect(screen.queryByText(/合计 40/)).not.toBeInTheDocument();
});
it("keeps old records unknown and zero-call cache events distinct",()=>{
 const {rerender,container}=render(<CompanionCost/>);expect(container).toBeEmptyDOMElement();
 rerender(<CompanionCost metrics={{duration_ms:2,model_calls:0,reported_calls:0}}/>);
 expect(screen.getByText("本次未调用模型。")).toBeInTheDocument();
});
