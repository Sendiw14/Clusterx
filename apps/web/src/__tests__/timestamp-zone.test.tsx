import { render } from "@testing-library/react";
import { act } from "react";
import { describe, expect, it } from "vitest";
import { ForceUtcContext, setPrefs, Timestamp } from "@clusterx/ui";

const V = "2026-03-02T10:42:07Z";

describe("Timestamp zone", () => {
  it("follows the zone preference, but a ForceUtc subtree (the demo) stays in UTC and keeps UTC in the title", () => {
    act(() => setPrefs({ timezone: "Asia/Tokyo" }));
    const a = render(<Timestamp value={V} />);
    expect(a.container.textContent).toMatch(/GMT\+9|JST/);
    a.unmount();
    const b = render(<ForceUtcContext value><Timestamp value={V} /></ForceUtcContext>);
    expect(b.container.textContent).toContain("UTC");
    expect(b.container.textContent).not.toMatch(/GMT\+9|JST/);
    expect(b.container.querySelector("time")!.getAttribute("title")).toBe("2026-03-02T10:42:07.000Z (UTC)");
    act(() => setPrefs({ timezone: "browser" }));
  });
});
