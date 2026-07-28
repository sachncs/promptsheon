import{i as e,n as t,r as n,t as r,v as i,y as a}from"./index-BWv0qt4w.js";function o(e,t=`neutral`){return`<span class="status-pill ${t} !px-2 !py-1"><span class="status-dot"></span>${n(e)}</span>`}function s(e){return{delete:`danger`,update:`warn`,create:`good`,activate:`good`,rollback:`danger`}[e]||`neutral`}function c(t){let r=t.details||{},i=(t.resource||``).split(`:`).pop(),a=r.name||i||t.resource,c=(t.user_id||`api`)===`api`?`System`:t.user_id;return`<a href="#/audit?action=${encodeURIComponent(t.action||``)}" class="flex items-start gap-3 rounded-lg px-2 py-2 transition hover:bg-paper/60">
    <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-paper text-[#5c5e63]"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-pulse"/></svg></span>
    <div class="min-w-0 flex-1"><p class="text-[.74rem] leading-5"><span class="font-bold">${n(c)}</span> · <span class="font-semibold">${n(t.action)}</span> · <span class="font-bold">${n(a)}</span></p><p class="mt-0.5 text-[.63rem] text-muted">${n(e(t.timestamp))} <span class="mx-1 text-[#c1c2bd]">·</span> <span class="mono">${n(t.resource)}</span></p></div>
    ${o(t.action,s(t.action))}
  </a>`}function l(t){let r={critical:`danger`,high:`danger`,medium:`warn`,low:`neutral`}[t.severity]||`neutral`;return`<a href="#/guardrails" class="flex items-start gap-3 rounded-lg bg-rose-50 px-2 py-2 transition hover:bg-rose-100">
    <span class="grid h-8 w-8 shrink-0 place-items-center rounded-lg bg-rose-100 text-rose-700"><svg class="h-4 w-4 fill-none stroke-current stroke-[1.7]"><use href="#icon-warning"/></svg></span>
    <div class="min-w-0 flex-1"><p class="text-[.74rem] font-bold">${n(t.rule_name||t.message||`Alert`)}</p><p class="mt-0.5 text-[.62rem] text-muted">${n(t.message||``)}</p><p class="mt-1 text-[.62rem] text-muted">Triggered ${n(e(t.triggered_at))}</p></div>
    ${o(t.severity,r)}
  </a>`}async function u(e){if(!e)return;e.innerHTML=`<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="notif-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Inbox</div><h2 id="notif-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Notifications</h2><p class="mt-1 text-[.7rem] text-muted">Active alerts and the most recent activity from the audit trail.</p></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <div class="px-5 py-5 sm:px-6 space-y-5 max-h-[70vh] overflow-y-auto">
        <section>
          <div class="flex items-center justify-between"><div class="eyebrow">Active alerts</div><span id="notif-alerts-count" class="text-[.62rem] text-muted">—</span></div>
          <div id="notif-alerts" class="mt-3 space-y-2"><div class="skeleton h-12 w-full"></div></div>
        </section>
        <section>
          <div class="flex items-center justify-between"><div class="eyebrow">Recent activity</div><a href="#/audit" class="text-[.62rem] text-muted hover:text-ink">Open audit →</a></div>
          <div id="notif-audit" class="mt-3 space-y-1"><div class="skeleton h-10 w-full"></div></div>
        </section>
      </div>
    </section>
  </div>`,e.querySelector(`[data-close-modal]`)?.addEventListener(`click`,()=>e.replaceChildren());let{loadSettings:o}=await r(async()=>{let{loadSettings:e}=await import(`./settings-DtbVhwEU.js`).then(e=>e.i);return{loadSettings:e}},[]);if(!o().apiKey)return;let[s,u]=await Promise.all([i().catch(e=>({ok:!1,error:String(e?.message||e)})),a({limit:8}).catch(e=>({ok:!1,error:String(e?.message||e)}))]),d=e.querySelector(`#notif-alerts`),f=e.querySelector(`#notif-alerts-count`);if(s.ok&&Array.isArray(s.data)&&s.data.length){let e=s.data.filter(e=>e.status===`active`||e.status===`pending`);e.length?(d.innerHTML=e.slice(0,4).map(l).join(``),f.textContent=`${e.length} firing`):(d.innerHTML=`<p class="rounded-lg border border-dashed border-line bg-paper p-3 text-center text-[.7rem] text-muted">No active alerts. All clear.</p>`,f.textContent=`0 firing`)}else d.innerHTML=`<p class="text-[.68rem] text-muted">${n(t(s))||`Alerts unavailable.`}</p>`,f.textContent=`—`;let p=e.querySelector(`#notif-audit`);u.ok&&Array.isArray(u.data)&&u.data.length?p.innerHTML=u.data.slice(0,6).map(c).join(``):p.innerHTML=`<p class="text-[.68rem] text-muted">${n(t(u))||`Audit log unavailable.`}</p>`}export{u as openNotificationsModal};