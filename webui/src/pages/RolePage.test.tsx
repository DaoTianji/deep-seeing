import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { RolePage } from "./RolePage";

vi.mock("../api", () => ({
  api: {
    activeRole: vi.fn().mockResolvedValue({ active: false, mode: "observe" }),
    roles: vi.fn().mockResolvedValue({
      roles: [{ id: "role-1", display_name: "测试角色", kind: "character", subject_class: "fictional", status: "draft", version: 1 }],
      mode: "observe",
    }),
    role: vi.fn().mockResolvedValue({ role: { id: "role-1", display_name: "测试角色", kind: "character", subject_class: "fictional", status: "draft", version: 1 }, sources: null, claims: null, sessions: null, worldlines: null }),
    roleInitializations: vi.fn().mockResolvedValue({ runs: null, mode: "observe", coverage_limited: true }),
  },
  streamChat: vi.fn(),
}));

describe("RolePage", () => {
  it("renders the library when empty API collections are encoded as null", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(<QueryClientProvider client={client}><RolePage /></QueryClientProvider>);

    expect(await screen.findByText("角色人生剧场")).toBeInTheDocument();
    expect(await screen.findAllByText("测试角色")).not.toHaveLength(0);
  });
});
