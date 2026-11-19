/**
 * A small JSON Schema checker for the handler catalogue (type, enum, const, required, properties, additionalProperties,
 * items, numeric/string/array bounds, pattern). Why not Ajv: it compiles schemas with `new Function`, which the console's
 * CSP forbids (no 'unsafe-eval'). The server validates authoritatively (422 with the same field pointers); this gives early
 * feedback. Keywords outside the subset are reported in `unchecked` so the UI can say the check was partial.
 */
export interface SchemaIssue { path: string; message: string }
export interface SchemaResult { issues: SchemaIssue[]; unchecked: string[] }

type S = Record<string, unknown>;
const KNOWN = new Set(["type", "enum", "const", "required", "properties", "additionalProperties", "items", "minimum", "maximum", "exclusiveMinimum", "exclusiveMaximum", "minLength", "maxLength", "pattern", "minItems", "maxItems", "uniqueItems", "description", "title", "default", "examples", "$schema", "$id", "format"]);

const typeOf = (v: unknown): string => (v === null ? "null" : Array.isArray(v) ? "array" : Number.isInteger(v) ? "integer" : typeof v);
const isType = (v: unknown, t: string) => (t === "number" ? typeof v === "number" : typeOf(v) === t);

export function checkSchema(schema: unknown, value: unknown): SchemaResult {
  const issues: SchemaIssue[] = [];
  const unchecked = new Set<string>();
  const walk = (s: unknown, v: unknown, path: string) => {
    if (typeof s !== "object" || s === null) return;
    const sc = s as S;
    for (const k of Object.keys(sc)) if (!KNOWN.has(k)) unchecked.add(k);
    const bad = (m: string) => issues.push({ path, message: m });
    if (sc.type !== undefined) {
      const ts = Array.isArray(sc.type) ? (sc.type as string[]) : [sc.type as string];
      if (!ts.some((t) => isType(v, t))) return bad(`must be ${ts.join(" or ")}`);
    }
    if (Array.isArray(sc.enum) && !sc.enum.some((e) => JSON.stringify(e) === JSON.stringify(v))) bad(`must be one of ${sc.enum.map((e) => JSON.stringify(e)).join(", ")}`);
    if ("const" in sc && JSON.stringify(sc.const) !== JSON.stringify(v)) bad(`must equal ${JSON.stringify(sc.const)}`);
    if (typeof v === "number") {
      if (typeof sc.minimum === "number" && v < sc.minimum) bad(`must be at least ${sc.minimum}`);
      if (typeof sc.maximum === "number" && v > sc.maximum) bad(`must be at most ${sc.maximum}`);
      if (typeof sc.exclusiveMinimum === "number" && v <= sc.exclusiveMinimum) bad(`must be greater than ${sc.exclusiveMinimum}`);
      if (typeof sc.exclusiveMaximum === "number" && v >= sc.exclusiveMaximum) bad(`must be less than ${sc.exclusiveMaximum}`);
    }
    if (typeof v === "string") {
      const len = [...v].length;
      if (typeof sc.minLength === "number" && len < sc.minLength) bad(`must be at least ${sc.minLength} characters`);
      if (typeof sc.maxLength === "number" && len > sc.maxLength) bad(`must be at most ${sc.maxLength} characters`);
      if (typeof sc.pattern === "string") {
        try {
          if (!new RegExp(sc.pattern, "u").test(v)) bad(`must match ${sc.pattern}`);
        } catch {
          unchecked.add("pattern");
        }
      }
    }
    if (Array.isArray(v)) {
      if (typeof sc.minItems === "number" && v.length < sc.minItems) bad(`must have at least ${sc.minItems} items`);
      if (typeof sc.maxItems === "number" && v.length > sc.maxItems) bad(`must have at most ${sc.maxItems} items`);
      if (sc.uniqueItems === true && new Set(v.map((x) => JSON.stringify(x))).size !== v.length) bad("items must be unique");
      if (sc.items && typeof sc.items === "object") v.forEach((x, i) => walk(sc.items, x, `${path}/${i}`));
    }
    if (typeOf(v) === "object") {
      const o = v as Record<string, unknown>;
      const props = (sc.properties ?? {}) as Record<string, unknown>;
      for (const r of Array.isArray(sc.required) ? (sc.required as string[]) : []) if (!(r in o)) issues.push({ path: `${path}/${r}`, message: "is required" });
      for (const [k, x] of Object.entries(o)) {
        if (k in props) walk(props[k], x, `${path}/${k}`);
        else if (sc.additionalProperties === false) issues.push({ path: `${path}/${k}`, message: "is not an allowed property" });
        else if (typeof sc.additionalProperties === "object") walk(sc.additionalProperties, x, `${path}/${k}`);
      }
    }
  };
  walk(schema, value, "");
  return { issues, unchecked: [...unchecked] };
}
