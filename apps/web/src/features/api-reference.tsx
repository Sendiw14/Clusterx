import { CodeBlock, Notice } from "@clusterx/ui";
import { EXECUTED_IN_CURL_GUIDE, type ApiModel, type FieldRow, type Operation } from "@/lib/docs/openapi";

const cell = (s: string) => (s ? s : "");

function FieldTable({ rows, caption }: { rows: FieldRow[]; caption: string }) {
  if (!rows.length) return null;
  return (
    <div className="card tbl-wrap doc-table" role="region" tabIndex={0} aria-label={caption}>
      <table className="tbl">
        <caption className="sr-only">{caption}</caption>
        <thead><tr><th scope="col">Field</th><th scope="col">Type</th><th scope="col">Required</th><th scope="col">Description</th></tr></thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.name}>
              <td data-label="Field" data-primary><code>{r.name}</code></td>
              <td data-label="Type">{r.type}</td>
              <td data-label="Required">{r.required ? "Yes" : "No"}</td>
              <td data-label="Description">{cell(r.description)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function Op({ o }: { o: Operation }) {
  return (
    <section id={o.id} className="op" aria-labelledby={`${o.id}-h`}>
      <h4 id={`${o.id}-h`} className="op-h doc-h">
        <span className="verb" data-m={o.method}>{o.method}</span>
        <code className="path">{o.path}</code>
        <a className="anchor" href={`#${o.id}`} aria-label={`Link to operation: ${o.id}`}>#</a>
      </h4>
      <p className="op-sum">{o.summary}</p>
      <ul className="op-meta" aria-label={`Status of ${o.id}`}>
        <li><span className="k">Operation ID</span> <code>{o.id}</code></li>
        <li><span className="k">Authentication</span> {o.auth.length ? o.auth.map((a) => <code key={a}>{a}</code>) : "none"}</li>
        {o.served === true ? <li><span className="chip-s" data-s="ok">Served in this build</span></li> : null}
        {o.served === false ? <li><span className="chip-s" data-s="no">Not served yet: answers 404 not_found_route</span></li> : null}
        {EXECUTED_IN_CURL_GUIDE.has(o.id) ? <li><span className="chip-s" data-s="ok">Executed in the M4 curl guide</span></li> : null}
      </ul>
      {o.params.length ? (
        <div className="card tbl-wrap doc-table" role="region" tabIndex={0} aria-label={`Parameters of ${o.id}`}>
          <table className="tbl">
            <caption className="sr-only">Parameters of {o.id}</caption>
            <thead><tr><th scope="col">Parameter</th><th scope="col">In</th><th scope="col">Type</th><th scope="col">Required</th><th scope="col">Description</th></tr></thead>
            <tbody>
              {o.params.map((p) => (
                <tr key={p.in + p.name}>
                  <td data-label="Parameter" data-primary><code>{p.name}</code></td><td data-label="In">{p.in}</td><td data-label="Type">{p.type}</td><td data-label="Required">{p.required ? "Yes" : "No"}</td><td data-label="Description">{p.description}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
      {o.requestType ? <><h5 className="op-sub">Request body: <code>{o.requestType}</code></h5><FieldTable rows={o.requestRows} caption={`Request body fields of ${o.id}`} /></> : null}
      {o.requestExamples.slice(0, 2).map((e) => (
        <div className="doc-code" key={`rq-${e.label}`}><CodeBlock code={e.json} language={`JSON, request, ${e.label}`} label={`Example request for ${o.id}: ${e.label}`} /></div>
      ))}
      <h5 className="op-sub">Responses</h5>
      <div className="card tbl-wrap doc-table" role="region" tabIndex={0} aria-label={`Responses of ${o.id}`}>
        <table className="tbl">
          <caption className="sr-only">Responses of {o.id}</caption>
          <thead><tr><th scope="col">Status</th><th scope="col">Body</th><th scope="col">Meaning</th></tr></thead>
          <tbody>
            {o.responses.map((r) => (
              <tr key={r.status}>
                <td data-label="Status" data-primary><code>{r.status}</code></td>
                <td data-label="Body">{r.schema}</td>
                <td data-label="Meaning">{r.description}{r.headers.length ? <span className="muted"> Headers: {r.headers.join(", ")}.</span> : null}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {o.responseExamples.slice(0, 2).map((e) => (
        <div className="doc-code" key={`rs-${e.label}`}><CodeBlock code={e.json} language={`JSON, response ${e.status ?? ""}, ${e.label}`} label={`Example response for ${o.id}: ${e.label}`} /></div>
      ))}
    </section>
  );
}

/** Everything below the narrative: generated from the parsed OpenAPI document, never hand-copied. */
export function ApiReference({ api }: { api: ApiModel }) {
  const groups = api.tags.map((t) => ({ ...t, ops: api.operations.filter((o) => o.tag === t.name) })).filter((g) => g.ops.length);
  return (
    <>
      <h2 id="contract" className="doc-h">This build&apos;s contract<a className="anchor" href="#contract" aria-label="Link to section: This build's contract">#</a></h2>
      <p>Generated from <code>api/openapi.yaml</code>: <strong>{api.title}</strong>, version <code>{api.version}</code>, {api.operations.length} operations. {api.servers[0] ? <>Default server: <code>{api.servers[0]}</code>.</> : null}</p>
      {api.servedIds && api.servedIds.length === api.operations.length ? (
        <div className="doc-callout"><Notice variant="info" lead="Served in this build." role="none">The document states that every operation is routed, and the control plane registers each of them (a test compares the two). Routed is not the same as exercised: the labels below say which operations a recorded run or an end-to-end test executed.</Notice></div>
      ) : api.servedIds ? (
        <div className="doc-callout"><Notice variant="info" lead="Served in this build." role="none">The document states that these operations are routed: {api.servedIds.map((x, i) => <span key={x}>{i ? ", " : ""}<a href={`#${x}`}>{x}</a></span>)}. All other operations are described by the contract but answer <code>404 not_found_route</code> for now.</Notice></div>
      ) : (
        <div className="doc-callout"><Notice variant="warning" lead="Served status unknown." role="none">This version of the document does not say which operations are routed, so none is marked as served.</Notice></div>
      )}

      <h2 id="security-schemes" className="doc-h">Security schemes<a className="anchor" href="#security-schemes" aria-label="Link to section: Security schemes">#</a></h2>
      <div className="card tbl-wrap doc-table" role="region" tabIndex={0} aria-label="Security schemes">
        <table className="tbl">
          <thead><tr><th scope="col">Scheme</th><th scope="col">Description</th></tr></thead>
          <tbody>{api.securitySchemes.map((s) => <tr key={s.name}><td data-label="Scheme" data-primary><code>{s.name}</code></td><td data-label="Description">{s.description}</td></tr>)}</tbody>
        </table>
      </div>

      <h2 id="error-codes" className="doc-h">Error codes and statuses<a className="anchor" href="#error-codes" aria-label="Link to section: Error codes and statuses">#</a></h2>
      <p>Defined values of the <code>code</code> field:</p>
      <p className="code-list">{api.errorCodes.map((c) => <code key={c}>{c}</code>)}</p>
      <div className="card tbl-wrap doc-table" role="region" tabIndex={0} aria-label="Error responses">
        <table className="tbl">
          <thead><tr><th scope="col">Status</th><th scope="col">Example code</th><th scope="col">Meaning</th></tr></thead>
          <tbody>{api.errorResponses.map((e) => <tr key={e.name}><td data-label="Status" data-primary><code>{e.status || "n/a"}</code></td><td data-label="Example code"><code>{e.code}</code></td><td data-label="Meaning">{e.description}</td></tr>)}</tbody>
        </table>
      </div>

      <h2 id="operations" className="doc-h">Operations<a className="anchor" href="#operations" aria-label="Link to section: Operations">#</a></h2>
      {groups.map((g) => (
        <section key={g.name} aria-labelledby={`tag-${g.name}`}>
          <h3 id={`tag-${g.name}`} className="doc-h">{g.name[0]!.toUpperCase() + g.name.slice(1)}<a className="anchor" href={`#tag-${g.name}`} aria-label={`Link to section: ${g.name}`}>#</a></h3>
          <p className="muted">{g.description}</p>
          {g.ops.map((o) => <Op key={o.id} o={o} />)}
        </section>
      ))}

      <h2 id="schemas" className="doc-h">Schemas<a className="anchor" href="#schemas" aria-label="Link to section: Schemas">#</a></h2>
      {api.schemas.map((s) => (
        <section key={s.name} id={`schema-${s.name}`} className="schema" aria-labelledby={`schema-${s.name}-h`}>
          <h3 id={`schema-${s.name}-h`} className="schema-h"><code>{s.name}</code></h3>
          {s.description ? <p className="muted">{s.description}</p> : null}
          {s.enumValues.length ? <p className="code-list">{s.enumValues.map((v) => <code key={v}>{v}</code>)}</p> : null}
          <FieldTable rows={s.rows} caption={`Fields of ${s.name}`} />
        </section>
      ))}
    </>
  );
}
