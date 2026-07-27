import{d as e,n as t,r as n}from"./index-BWv0qt4w.js";async function r(e,t,r){if(!e)return;let o=(r?.reduce((e,t)=>Math.max(e,t.version||0),0)||0)+1,s=({error:e=null,created:r=null}={})=>`<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="new-version-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Versions / New</div><h2 id="new-version-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">New version for ${n(t.name)}</h2><p class="mt-1 text-[.7rem] text-muted">Manifest artifacts are sha256 hashes; paste them in or compute via the helper.</p></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form id="new-version-form" class="space-y-4 px-5 py-5 sm:px-6">
        <div><label class="eyebrow mb-2 block" for="nv-version">Version number</label><input id="nv-version" name="version" class="field mono" required type="number" min="1" value="${n(o)}" data-autofocus /></div>
        <div class="grid gap-3 sm:grid-cols-2">
          ${i(`nv-prompt`,`Prompt hash`,`prompt`)}
          ${i(`nv-model`,`Model policy hash`,`model_policy`)}
          ${i(`nv-runtime`,`Runtime policy hash`,`runtime_policy`)}
          ${i(`nv-context`,`Context contract hash`,`context_contract`)}
          ${i(`nv-memory`,`Memory hash`,`memory`)}
        </div>
        ${e?`<p class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${n(e)}</p>`:``}
        ${r?`<p class="rounded-lg bg-lime/15 px-3 py-2 text-[.68rem] text-[#52632d]">Created <span class="mono">${n(r.id)}</span>.</p>`:``}
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="quiet-button" data-close-modal>Cancel</button>
          <button type="submit" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create version</button>
        </div>
      </form>
    </section>
  </div>`;e.innerHTML=s(),a(e,s,t)}function i(e,t,r){return`<div><label class="eyebrow mb-2 block" for="${e}">${n(t)}</label><div class="flex gap-1"><input id="${e}" name="${r}" class="field mono" required placeholder="64 hex chars" /><button type="button" data-fill-hash="${r}" class="quiet-button !h-9 !px-2 !text-[.66rem]">Compute</button></div></div>`}function a(n,r,i){n.querySelector(`[data-close-modal]`)?.addEventListener(`click`,()=>n.replaceChildren()),n.querySelectorAll(`[data-fill-hash]`).forEach(e=>{e.addEventListener(`click`,async()=>{let t=e.dataset.fillHash,r=n.querySelector(`#nv-${t===`prompt`?`prompt`:t===`model_policy`?`model`:t===`runtime_policy`?`runtime`:t===`context_contract`?`context`:`memory`}`),i=window.prompt(`Paste the source text for the ${t} artifact to hash:`);if(!i)return;let a=await o(i);r&&(r.value=a)})}),n.querySelector(`#new-version-form`)?.addEventListener(`submit`,async o=>{o.preventDefault();let s=new FormData(o.target),c={version:Number(s.get(`version`)),manifest:{prompt:{kind:`prompt`,hash:(s.get(`prompt`)||``).toString()},model_policy:{kind:`model_policy`,hash:(s.get(`model_policy`)||``).toString()},runtime_policy:{kind:`runtime_policy`,hash:(s.get(`runtime_policy`)||``).toString()},context_contract:{kind:`context_contract`,hash:(s.get(`context_contract`)||``).toString()},memory:{kind:`memory`,hash:(s.get(`memory`)||``).toString()}}},l=await e(i.id,c);if(!l.ok){n.innerHTML=r({error:t(l)}),a(n,r,i);return}window.location.reload()})}async function o(e){let t=new TextEncoder().encode(e),n=await crypto.subtle.digest(`SHA-256`,t);return Array.from(new Uint8Array(n)).map(e=>e.toString(16).padStart(2,`0`)).join(``)}export{r as openNewVersionModal};