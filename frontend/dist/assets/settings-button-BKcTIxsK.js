import{n as e,r as t,t as n}from"./settings-DtbVhwEU.js";import{n as r,o as i,r as a}from"./index-BWv0qt4w.js";var o={apiBase:{label:`API base URL`,type:`text`,placeholder:`leave blank for /api via Vite proxy`},apiKey:{label:`API key (ps_…)`,type:`text`,placeholder:`leave blank for auth-off dev mode`}};function s(e,t){return`
    <div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="settings-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div>
            <div class="eyebrow">Connection</div>
            <h2 id="settings-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Promptsheon API</h2>
            <p class="mt-1 text-[.7rem] text-muted">Override the API URL or paste a key. Stored only in this browser.</p>
          </div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="settings-form" class="space-y-4 px-5 py-5 sm:px-6">
          ${Object.entries(o).map(([t,n])=>`
            <div>
              <label class="eyebrow mb-2 block" for="settings-${t}">${a(n.label)}</label>
              <input id="settings-${t}" name="${t}" type="${n.type}" class="field mono" ${t===`apiBase`?`data-autofocus`:``} value="${a(e[t]||``)}" placeholder="${a(n.placeholder||``)}" />
            </div>
          `).join(``)}
          <div class="rounded-xl border border-line bg-paper/50 p-3">
            <div class="flex items-center justify-between">
              <span class="eyebrow">Test connection</span>
              <button type="button" id="settings-test" class="quiet-button !h-7 !text-[.68rem]">Run</button>
            </div>
            <p id="settings-test-result" class="mt-2 text-[.68rem] text-muted">No connection test run yet.</p>
          </div>
          ${t?`<p class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${a(t)}</p>`:``}
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-clear-settings>Clear</button>
            <button type="submit" class="primary-button">Save and reload</button>
          </div>
        </form>
      </section>
    </div>
  `}function c(e,o,{onSaved:s,onCleared:c}={}){let l=e.querySelector(`#settings-form`);l.addEventListener(`submit`,e=>{e.preventDefault();let n=new FormData(l);t({apiBase:(n.get(`apiBase`)||``).toString().trim(),apiKey:(n.get(`apiKey`)||``).toString().trim()}),window.location.reload()}),l.querySelector(`[data-clear-settings]`)?.addEventListener(`click`,()=>{n(),window.location.reload()}),e.querySelector(`#settings-test`)?.addEventListener(`click`,async()=>{let t=e.querySelector(`#settings-test-result`);t.textContent=`Testing…`,t.className=`mt-2 text-[.68rem] text-muted`;let n=await i(`/health`,{retry:!1,timeout:4e3});if(n.ok){t.textContent=`Connected to daemon (v${a(n.data?.version||`?`)}, uptime ${a(n.data?.uptime||`?`)}).`,t.className=`mt-2 rounded-md bg-lime/15 px-2 py-1.5 text-[.68rem] text-[#3b641b]`;return}let s=await fetch(`/health`).then(e=>e.ok?e.json():null).catch(()=>null);if(s){t.textContent=`Direct connection failed (${r(n)}); Vite proxy works (v${a(s.version||`?`)}).`,t.className=`mt-2 rounded-md bg-amber-100 px-2 py-1.5 text-[.68rem] text-amber-800`;return}t.textContent=`${r(n)} — is the daemon running on ${a(o.apiBase||`/health`)}?`,t.className=`mt-2 rounded-md bg-rose-100 px-2 py-1.5 text-[.68rem] text-rose-800`})}async function l(t,{bootstrapError:n=null,onSaved:r}={}){let i=e();t.innerHTML=s(i,n),c(t,i,{onSaved:r})}export{l as openSettings};