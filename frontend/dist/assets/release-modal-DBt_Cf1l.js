import{E as e,a as t,g as n,h as r,i,n as a,p as o,r as s,x as c}from"./index-BWv0qt4w.js";function l(e){return`<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="release-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Release review</div><h2 id="release-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Loading ${s(e||``)}…</h2></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <div class="px-6 py-6 space-y-3">
        <span class="skeleton block h-4 w-40"></span>
        <span class="skeleton block h-4 w-72"></span>
        <span class="skeleton block h-4 w-56"></span>
        <span class="skeleton block h-24 w-full"></span>
      </div>
    </section>
  </div>`}function u(e){return!e||!Object.keys(e).length?`<span class="text-muted">(empty manifest)</span>`:Object.entries(e).map(([e,t])=>{let n=typeof t==`string`?t:JSON.stringify(t);return`<div class="flex items-baseline gap-2"><span class="text-muted shrink-0">${s(e)}:</span> <span class="break-all">${s(n)}</span></div>`}).join(``)}function d(e){if(!e)return`<p class="mt-2 text-[.68rem] text-muted">Approval tally unavailable — quorum check will fail until the API responds.</p>`;let t=e.votes||[],n=e.updated_at;return`<p class="mt-2 text-[.68rem] text-muted">${t.length} vote${t.length===1?``:`s`} cast${n?` · updated ${s(i(n))}`:``}.</p>`}function f(e,t,n){let r=e||{},a=t?.name||r.capability_id||`Unknown capability`,o={active:`good`,approved:`good`,pending:`warn`,superseded:`neutral`,rolled_back:`danger`}[r.status]||`neutral`,c=r.status===`pending`,l=r.status===`active`||r.status===`superseded`;return`<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="release-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div>
          <div class="eyebrow">Release review · ${s(r.environment||`?`)}</div>
          <h2 id="release-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">${s(a)}</h2>
          <p class="mt-1 text-[.7rem] text-muted">v${s(r.capability_version||`?`)} · <span class="mono">${s(r.id||``)}</span> · ${g(r.status||`?`,o)}</p>
          ${d(n)}
        </div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <div class="space-y-5 px-5 py-5 sm:px-6">
        <div class="grid grid-cols-2 gap-3 sm:grid-cols-4">
          ${h(`Version`,r.capability_version??`—`)}
          ${h(`Environment`,r.environment??`—`)}
          ${h(`Created`,i(r.created_at))}
          ${h(`Status`,r.status??`—`)}
        </div>
        <div>
          <div class="eyebrow">Manifest fingerprints</div>
          <div class="mt-2 max-h-32 overflow-auto rounded-lg bg-paper p-3 text-[.65rem] mono text-[#50535a]">${u(r.manifest)}</div>
        </div>
        <div>
          <div class="eyebrow">Approval activity</div>
          <div class="mt-2 space-y-2">
            ${p(n)}
            ${m(`Required approver`,`?`,`Pending`,`warn`)}
          </div>
        </div>
        <form id="vote-form" class="space-y-3" data-release-id="${s(r.id||``)}">
          <label class="eyebrow block" for="vote-note">Decision note</label>
          <textarea id="vote-note" class="field min-h-20 resize-y" placeholder="Add context for the audit trail"></textarea>
          <p id="vote-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
          <div class="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
            ${l?`<button type="button" id="vote-rollback" data-rollback class="quiet-button">Rollback</button>`:``}
            <button type="button" class="quiet-button" data-close-modal>Close</button>
            ${c?`
              <button type="submit" name="decision" value="reject" class="quiet-button">Reject</button>
              <button type="submit" name="decision" value="approve" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-check"/></svg>Record approval</button>
            `:``}
          </div>
        </form>
      </div>
    </section>
  </div>`}function p(e){return!e||!Array.isArray(e.votes)||e.votes.length===0?`<p class="text-[.68rem] text-muted">No votes yet.</p>`:e.votes.map(e=>{let t=e.decision===`approve`?`good`:e.decision===`reject`?`danger`:`neutral`;return`<div class="flex items-center gap-3">
      <span class="grid h-8 w-8 place-items-center rounded-lg bg-paper text-[.62rem] font-bold">${s((e.identity||`?`)[0].toUpperCase())}</span>
      <span class="flex-1 text-[.72rem]"><span class="font-bold">${s(e.identity)}</span><span class="block text-[.63rem] text-muted">${s(e.reason||`no reason`)} · ${s(i(e.timestamp))}</span></span>
      ${g(e.decision,t)}
    </div>`}).join(``)}function m(e,t,n,r){return`<div class="flex items-center gap-3">
    <span class="grid h-8 w-8 place-items-center rounded-lg border border-dashed border-[#c9cac5] text-[.7rem] text-muted">${s(t)}</span>
    <span class="flex-1 text-[.72rem]"><span class="font-bold">${s(e)}</span><span class="block text-[.63rem] text-muted">${s(n)}</span></span>
    ${g(n,r)}
  </div>`}function h(e,t){return`<div class="rounded-lg bg-paper p-3"><span class="eyebrow">${s(e)}</span><span class="mono mt-2 block text-[.8rem] font-bold">${s(t)}</span></div>`}function g(e,t){return`<span class="status-pill ${t} !px-2 !py-1"><span class="status-dot"></span>${s(e)}</span>`}async function _(e,t){e.innerHTML=l(t);let[i,c,u]=await Promise.all([r(t),o(null).catch(()=>null),n(t)]),d=i.ok?i.data:null,p=null;if(d?.capability_id){let e=await o(d.capability_id);e.ok&&(p=e.data)}if(!i.ok){e.innerHTML=`<div class="modal-backdrop" role="presentation"><section class="modal-card" role="dialog" aria-modal="true"><div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6"><div><div class="eyebrow">Release</div><h2 class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Cannot load release</h2></div><button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button></div><div class="px-6 py-6 text-[.78rem] text-muted">${s(a(i))}</div></section></div>`;return}e.innerHTML=f(d,p,u.ok?u.data:null),v(e,d)}function v(n,r){let i=n.querySelector(`#vote-form`);i?.addEventListener(`submit`,async o=>{o.preventDefault();let s=o.submitter?.value||`approve`,c=(n.querySelector(`#vote-note`)?.value||``).trim(),l=n.querySelector(`#vote-error`),u=i.querySelectorAll(`button[type=submit], [data-rollback]`);u.forEach(e=>e.disabled=!0);let d=await e(r.id,s,c);if(!d.ok){u.forEach(e=>e.disabled=!1),l&&(l.textContent=a(d),l.classList.remove(`hidden`));return}s===`approve`&&await t(r.id),y(i,n)});let o=n.querySelector(`[data-rollback]`);o?.addEventListener(`click`,async()=>{let e=n.querySelector(`#vote-error`);o.disabled=!0;let t=await c(r.id);if(o.disabled=!1,!t.ok){e&&(e.textContent=a(t),e.classList.remove(`hidden`));return}y(i,n)}),n.querySelector(`[data-close-modal]`)?.addEventListener(`click`,()=>n.replaceChildren())}function y(e,t){t.replaceChildren(),window.location.reload()}async function b(e,t){e&&await _(e,t)}export{b as openReleaseModal};