import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SeriesChart, axisTicks } from "../features/chart";

describe("chart y-axis ticks (count series)", () => {
  it.each([[0, [0, 1]], [1, [0, 1]], [2, [0, 1, 2]], [3, [0, 2, 3]], [4, [0, 2, 4]], [5, [0, 3, 5]]])("max %i gives unique whole numbers", (max, want) => {
    const t = axisTicks(max, "");
    expect(t).toEqual(want);
    expect(new Set(t).size).toBe(t.length);
    expect(t.every(Number.isInteger)).toBe(true);
  });
  it("rendered labels are unique for small maxima", () => {
    for (const peak of [0, 1, 2, 3]) {
      const { container, unmount } = render(<SeriesChart title="t" stepLabel="5-minute" series={[{ label: "A", color: "red", points: [{ t: "2026-01-01T00:00:00Z", v: peak }, { t: "2026-01-01T00:05:00Z", v: 0 }] }]} />);
      const labels = [...container.querySelectorAll(".axis text")].map((e) => e.textContent);
      expect(new Set(labels).size, `peak ${peak}: ${labels}`).toBe(labels.length);
      unmount();
    }
  });
  it("seconds axis keeps half steps", () => expect(axisTicks(8, "s")).toEqual([0, 4, 8]));
});
