import { act, fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/link", () => ({ default: ({ href, children, ...rest }: { href: string; children: React.ReactNode }) => <a href={href} {...rest}>{children}</a> }));
vi.mock("next/navigation", () => ({ usePathname: () => "/app/jobs", useRouter: () => ({ push: vi.fn(), replace: vi.fn() }), useSearchParams: () => new URLSearchParams() }));

import {
  Button, ButtonLink, CopyButton, DataTable, Dialog, EmptyState, ErrorState, FilterChip, IconSprite, MaskedField, Notice, Pagination, Segmented, StatusBadge, StatusLegend, Tabs, TextInput, Timestamp, ToastProvider, useToast, LiveIndicator, JOB_STATES,
} from "@clusterx/ui";

describe("StatusBadge and legend", () => {
  it("shows text plus a decorative glyph, never glyph alone", () => {
    const { container } = render(<><IconSprite /><StatusBadge state="retry_wait" /></>);
    expect(screen.getByText("Retry wait")).toBeInTheDocument();
    const svg = container.querySelector(".status svg");
    expect(svg).toHaveAttribute("aria-hidden", "true");
    expect(container.querySelector(".status")).toHaveAttribute("data-s", "retry_wait");
  });
  it("legend lists exactly the seven 1.0 states and never blocked or skipped or unschedulable", () => {
    const { container } = render(<StatusLegend />);
    expect(JOB_STATES).toHaveLength(7);
    const items = container.querySelectorAll("li .status");
    expect([...items].map((i) => i.textContent)).toEqual(["Scheduled", "Queued", "Running", "Retry wait", "Succeeded", "Failed", "Canceled"]);
    expect(container.textContent).not.toMatch(/blocked|skipped|unschedulable|dead.?letter state/i);
  });
  it("masked field is explicit text, not blank", () => {
    render(<MaskedField what="payload" />);
    expect(screen.getByText(/Hidden for viewer role/)).toBeInTheDocument();
  });
});

describe("Button", () => {
  it("pending: aria-disabled, data-pending, ignores repeat clicks", async () => {
    const onClick = vi.fn();
    render(<Button pending onClick={onClick}>Redirecting to GitHub</Button>);
    const b = screen.getByRole("button", { name: /Redirecting to GitHub/ });
    expect(b).toHaveAttribute("aria-disabled", "true");
    expect(b).toHaveAttribute("data-pending", "true");
    await userEvent.click(b);
    await userEvent.click(b);
    expect(onClick).not.toHaveBeenCalled();
  });
  it("unavailable stays focusable (aria-disabled, not disabled)", async () => {
    render(<Button unavailable>Drain</Button>);
    const b = screen.getByRole("button", { name: "Drain" });
    expect(b).not.toBeDisabled();
    b.focus();
    expect(b).toHaveFocus();
  });
  it("normal click fires once; Enter and Space activate", async () => {
    const onClick = vi.fn();
    render(<Button onClick={onClick}>Go</Button>);
    const b = screen.getByRole("button", { name: "Go" });
    await userEvent.click(b);
    b.focus();
    await userEvent.keyboard("{Enter}");
    await userEvent.keyboard(" ");
    expect(onClick).toHaveBeenCalledTimes(3);
  });
  it("external link adds the glyph and the words (external)", () => {
    render(<ButtonLink href="https://example.com" external>Source</ButtonLink>);
    const a = screen.getByRole("link", { name: /Source/ });
    expect(a).toHaveAccessibleName("Source (external)");
    expect(a).toHaveAttribute("rel", "noopener noreferrer");
  });
});

describe("Notice", () => {
  it("lead phrase carries meaning; error defaults to alert, others to status; never both", () => {
    const { rerender } = render(<Notice variant="error" lead="Failed.">Detail</Notice>);
    expect(screen.getByRole("alert")).toHaveTextContent("Failed.");
    rerender(<Notice variant="warning" lead="Stale.">x</Notice>);
    expect(screen.getByRole("status")).toHaveTextContent("Stale.");
    expect(screen.queryByRole("alert")).toBeNull();
  });
});

describe("Dialog", () => {
  function Host({ blocking }: { blocking?: boolean }) {
    const [open, setOpen] = useState(false);
    return (
      <>
        <button onClick={() => setOpen(true)}>Open</button>
        <Dialog open={open} onClose={() => setOpen(false)} title="Confirm" description="Consequence text" blocking={blocking}>body</Dialog>
        <span data-testid="state">{open ? "open" : "closed"}</span>
      </>
    );
  }
  it("is labelled by its title and described by its consequence text", async () => {
    render(<Host />);
    await userEvent.click(screen.getByText("Open"));
    const d = screen.getByRole("dialog", { name: "Confirm" });
    expect(d).toHaveAccessibleDescription("Consequence text");
    expect(d).toHaveAttribute("open");
  });
  it("close button closes; blocking dialogs have no close button and ignore cancel", async () => {
    const { unmount } = render(<Host />);
    await userEvent.click(screen.getByText("Open"));
    await userEvent.click(screen.getByRole("button", { name: "Close: Confirm" }));
    expect(screen.getByTestId("state")).toHaveTextContent("closed");
    unmount();
    render(<Host blocking />);
    await userEvent.click(screen.getByText("Open"));
    expect(screen.queryByRole("button", { name: /Close/ })).toBeNull();
    const d = screen.getByRole("dialog", { hidden: true });
    const ev = new Event("cancel", { cancelable: true });
    d.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
  });
});

describe("Tabs", () => {
  it("roving tabindex with Arrow, Home and End; panel follows selection", async () => {
    render(<Tabs label="Credentials" items={[{ id: "a", label: "API tokens", panel: "panel A" }, { id: "b", label: "Worker credentials", panel: "panel B" }, { id: "c", label: "Other", panel: "panel C" }]} />);
    const tabs = screen.getAllByRole("tab");
    expect(tabs.map((t) => t.getAttribute("tabindex"))).toEqual(["0", "-1", "-1"]);
    tabs[0]!.focus();
    await userEvent.keyboard("{ArrowRight}");
    expect(tabs[1]).toHaveFocus();
    expect(tabs[1]).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tabpanel")).toHaveTextContent("panel B");
    await userEvent.keyboard("{End}");
    expect(tabs[2]).toHaveFocus();
    await userEvent.keyboard("{Home}");
    expect(tabs[0]).toHaveFocus();
    await userEvent.keyboard("{ArrowLeft}");
    expect(tabs[2]).toHaveFocus();
  });
});

describe("DataTable and Pagination", () => {
  const rows = [{ id: "a1", n: 3 }, { id: "b2", n: 10 }];
  it("native table with caption, scoped headers, labelled cells for stacked mode, numeric alignment hook", () => {
    render(<DataTable caption="Jobs, 2 shown" rows={rows} rowKey={(r) => r.id} columns={[{ key: "id", header: "Job", primary: true, render: (r) => r.id }, { key: "n", header: "Attempts", numeric: true, render: (r) => r.n }]} />);
    const t = screen.getByRole("table", { name: "Jobs, 2 shown" });
    expect(within(t).getAllByRole("columnheader").map((h) => h.getAttribute("scope"))).toEqual(["col", "col"]);
    const cells = t.querySelectorAll("tbody td");
    expect(cells[0]).toHaveAttribute("data-primary");
    expect(cells[1]).toHaveAttribute("data-label", "Attempts");
    expect(cells[1]).toHaveClass("num");
    expect(screen.getByRole("region", { name: "Jobs, 2 shown" })).toHaveAttribute("tabindex", "0");
  });
  it("busy dims but keeps rows (aria-busy, no blanking)", () => {
    render(<DataTable busy caption="c" rows={rows} rowKey={(r) => r.id} columns={[{ key: "id", header: "Job", render: (r) => r.id }]} />);
    expect(screen.getByRole("region")).toHaveAttribute("aria-busy", "true");
    expect(screen.getAllByRole("row")).toHaveLength(3);
  });
  it("pagination: disabled states carry a visible reason and do not fire", async () => {
    const onPrev = vi.fn(), onNext = vi.fn();
    render(<Pagination hasPrev={false} hasNext={true} onPrev={onPrev} onNext={onNext} note="50 per page" />);
    const prev = screen.getByRole("button", { name: /Previous/ });
    expect(prev).toHaveAttribute("aria-disabled", "true");
    expect(prev).toHaveTextContent("(first page)");
    await userEvent.click(prev);
    expect(onPrev).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: /Next/ }));
    expect(onNext).toHaveBeenCalledOnce();
    expect(screen.getByRole("navigation", { name: "Pagination" })).toBeInTheDocument();
  });
});

describe("forms and chips", () => {
  it("TextInput links label, help and error; error sets aria-invalid", () => {
    render(<TextInput label="Handler" help="exact name" error="Required" />);
    const i = screen.getByLabelText("Handler");
    expect(i).toHaveAttribute("aria-invalid", "true");
    expect(i).toHaveAccessibleDescription(/exact name.*Required|Required/);
  });
  it("FilterChip remove button is named after the filter", async () => {
    const onRemove = vi.fn();
    render(<FilterChip label="Status: Failed" onRemove={onRemove} />);
    await userEvent.click(screen.getByRole("button", { name: "Remove Status: Failed filter" }));
    expect(onRemove).toHaveBeenCalled();
  });
  it("Segmented is a labelled aria-pressed group, not tabs", async () => {
    const onChange = vi.fn();
    render(<Segmented label="View" value="list" onChange={onChange} options={[{ value: "list", label: "List" }, { value: "graph", label: "Graph" }]} />);
    expect(screen.getByRole("group", { name: "View" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "List" })).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(screen.getByRole("button", { name: "Graph" }));
    expect(onChange).toHaveBeenCalledWith("graph");
  });
});

describe("states", () => {
  it("ErrorState chooses copy from the code, shows the request id, retry works, alert only when blocking", async () => {
    const retry = vi.fn();
    const { rerender } = render(<ErrorState error={{ code: "unavailable", status: 503, requestId: "req_abc" }} onRetry={retry} />);
    expect(screen.getByRole("heading", { name: "The service is temporarily unavailable" })).toBeInTheDocument();
    expect(screen.getByTestId("request-id")).toHaveTextContent("req_abc");
    expect(screen.queryByRole("alert")).toBeNull();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalled();
    rerender(<ErrorState blocking error={{ code: "internal" }} />);
    expect(screen.getByRole("alert")).toBeInTheDocument();
  });
  it("ErrorState never echoes a raw server message for unknown codes", () => {
    render(<ErrorState error={{ code: "totally_unknown_code" }} />);
    expect(screen.getByRole("heading", { name: "Something went wrong on the server" })).toBeInTheDocument();
  });
  it("EmptyState has title, sentence and actions", () => {
    render(<EmptyState title="No jobs yet" actions={<a href="#docs">Docs</a>}>Jobs appear here.</EmptyState>);
    expect(screen.getByRole("heading", { name: "No jobs yet" })).toBeInTheDocument();
    expect(screen.getByText("Jobs appear here.")).toBeInTheDocument();
  });
  it("LiveIndicator announces state through role=status and a label", () => {
    render(<LiveIndicator state="polling" detail="every 15 s" />);
    const s = screen.getByRole("status");
    expect(s).toHaveAccessibleName("Connection state: Polling, every 15 s");
    expect(s).toHaveAttribute("data-live", "polling");
  });
});

describe("clipboard and toast", () => {
  it("CopyButton copies, shows Copied, then returns; blocked clipboard shows manual guidance", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    render(<CopyButton text="exact text" />);
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /Copy/ })); });
    expect(writeText).toHaveBeenCalledWith("exact text");
    expect(screen.getByRole("button", { name: /Copied/ })).toBeInTheDocument();
    act(() => { vi.advanceTimersByTime(2000); });
    expect(screen.getByRole("button", { name: /Copy/ })).not.toHaveTextContent("Copied");
    writeText.mockRejectedValue(new Error("denied"));
    await act(async () => { fireEvent.click(screen.getByRole("button", { name: /Copy/ })); });
    expect(screen.getByText(/Copy blocked/)).toBeInTheDocument();
    vi.useRealTimers();
  });
  it("Toast shows one message at a time in a polite status region", async () => {
    function T() { const t = useToast(); return <button onClick={() => t.show("Saved")}>go</button>; }
    render(<ToastProvider><T /></ToastProvider>);
    await userEvent.click(screen.getByText("go"));
    expect(screen.getAllByText("Saved")).toHaveLength(1);
    await userEvent.click(screen.getByText("go"));
    expect(screen.getAllByText("Saved")).toHaveLength(1);
  });
});

describe("Timestamp", () => {
  it("renders <time datetime> with UTC in the title and an explicit zone", () => {
    const { container } = render(<Timestamp value="2026-03-02T10:42:07Z" utc />);
    const t = container.querySelector("time")!;
    expect(t).toHaveAttribute("datetime", "2026-03-02T10:42:07.000Z");
    expect(t).toHaveAttribute("title", "2026-03-02T10:42:07.000Z (UTC)");
    expect(t.textContent).toMatch(/UTC$/);
    expect(t.textContent).toContain("10:42:07");
  });
  it("invalid and empty values are explicit, never NaN", () => {
    const { container } = render(<><Timestamp value="not a date" /><Timestamp value={null} /></>);
    expect(container.textContent).toContain("Invalid time");
    expect(container.textContent).toContain("Not set");
    expect(container.textContent).not.toContain("NaN");
  });
});
