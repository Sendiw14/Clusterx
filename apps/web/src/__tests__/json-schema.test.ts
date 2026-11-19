import { describe, expect, it } from "vitest";
import { checkSchema } from "@/lib/json-schema";

const hash = { type: "object", additionalProperties: false, required: ["data"], properties: { data: { type: "string", maxLength: 4 } } };
const sleep = { type: "object", additionalProperties: false, required: ["ms"], properties: { ms: { type: "integer", minimum: 0, maximum: 60000 } } };

describe("checkSchema (the catalogue subset)", () => {
  it("accepts valid payloads", () => {
    expect(checkSchema(hash, { data: "abc" }).issues).toEqual([]);
    expect(checkSchema(sleep, { ms: 10 }).issues).toEqual([]);
  });
  it("names the JSON pointer of every violation", () => {
    expect(checkSchema(hash, {}).issues).toEqual([{ path: "/data", message: "is required" }]);
    expect(checkSchema(hash, { data: "toolong" }).issues[0]).toMatchObject({ path: "/data" });
    expect(checkSchema(hash, { data: "a", extra: 1 }).issues).toEqual([{ path: "/extra", message: "is not an allowed property" }]);
    expect(checkSchema(sleep, { ms: 70000 }).issues[0]).toMatchObject({ path: "/ms", message: "must be at most 60000" });
    expect(checkSchema(sleep, { ms: 1.5 }).issues[0]).toMatchObject({ path: "/ms", message: "must be integer" });
    expect(checkSchema(sleep, "x").issues[0]).toMatchObject({ path: "", message: "must be object" });
  });
  it("checks arrays, enums and patterns, and reports keywords it cannot check", () => {
    const s = { type: "object", properties: { a: { type: "array", items: { enum: ["x", "y"] }, maxItems: 2 }, b: { type: "string", pattern: "^[a-z]+$" } }, oneOf: [{}] };
    const r = checkSchema(s, { a: ["x", "z", "y"], b: "A" });
    expect(r.issues.map((i) => i.path).sort()).toEqual(["/a", "/a/1", "/b"]);
    expect(r.unchecked).toEqual(["oneOf"]);
  });
});
