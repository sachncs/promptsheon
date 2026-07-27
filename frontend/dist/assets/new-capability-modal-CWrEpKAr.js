import{_ as e,b as t,c as n,d as r,n as i,r as a}from"./index-BWv0qt4w.js";function o(e,t){return`<div class="rounded-lg bg-paper p-3"><span class="eyebrow">${a(e)}</span><span class="mono mt-2 block text-[.78rem] font-bold">${a(t)}</span></div>`}async function s(e,n){if(!e)return;let r=e=>{let t=!e.projects||e.projects.length===0,n=(e.projects||[]).map(e=>`<option value="${a(e.id)}"${e.selected?` selected`:``}>${a(e.name)}</option>`).join(``);return`<div class="modal-backdrop" role="presentation">
      <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="new-capability-title">
        <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
          <div><div class="eyebrow">Catalog / New</div><h2 id="new-capability-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">Create a capability</h2><p class="mt-1 text-[.7rem] text-muted">Define the outcome first. Implementation can evolve behind it.</p></div>
          <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
        </div>
        <form id="new-capability-form" class="space-y-5 px-5 py-5 sm:px-6">
          <div><label class="eyebrow mb-2 block" for="nc-name">Capability name</label><input id="nc-name" name="name" class="field" required ${e.autofocus?`data-autofocus`:``} placeholder="e.g. Summarize customer feedback" /></div>
          <div><label class="eyebrow mb-2 block" for="nc-description">Business outcome</label><textarea id="nc-description" name="description" class="field min-h-24 resize-y" placeholder="What should this capability reliably help your team accomplish?"></textarea></div>
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <label class="eyebrow mb-2 block" for="nc-project">Project</label>
              <select id="nc-project" name="project_id" class="field" required>
                ${n||(t?`<option value="" disabled selected>No projects yet — create one first</option>`:`<option value="" disabled selected>Select a project</option>`)}
              </select>
            </div>
            <div><label class="eyebrow mb-2 block" for="nc-owner">Owner</label><select id="nc-owner" name="owner" class="field"><option value="">— unassigned —</option>${(e.users||[]).map(e=>`<option value="${a(e.id)}">${a(e.name||e.email||e.id)}</option>`).join(``)}</select></div>
          </div>
          <div><label class="eyebrow mb-2 block" for="nc-tags">Tags (comma separated)</label><input id="nc-tags" name="tags" class="field" placeholder="research, finance" /></div>
          <details class="rounded-xl border border-line bg-paper/40 p-3 text-[.74rem]">
            <summary class="cursor-pointer select-none font-bold text-ink">Advanced — create the first version and a pending release in one shot</summary>
            <div class="mt-3 grid gap-3 sm:grid-cols-3">
              <div><label class="eyebrow mb-2 block" for="nc-env">Target environment</label><select id="nc-env" name="environment" class="field"><option value="">— skip —</option><option value="dev">dev</option><option value="staging">staging</option><option value="prod">prod</option></select></div>
              <div><label class="eyebrow mb-2 block" for="nc-prompt">Prompt</label><input id="nc-prompt" name="prompt" class="field mono" placeholder="System prompt text" /></div>
              <div><label class="eyebrow mb-2 block" for="nc-model">Model</label><input id="nc-model" name="model" class="field mono" placeholder="gpt-4o-mini" /></div>
            </div>
            <p class="mt-3 text-[.66rem] text-muted">Hashes are computed client-side from the prompt and model strings.</p>
          </details>
          <p id="nc-error" class="hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
          ${e.error?`<p class="rounded-lg bg-amber-50 px-3 py-2 text-[.68rem] text-amber-800">${a(e.error)}</p>`:``}
          ${e.created?c(e.created):``}
          <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
            <button type="button" class="quiet-button" data-close-modal>Cancel</button>
            <button type="submit" class="primary-button"><svg class="h-3.5 w-3.5 fill-none stroke-current stroke-2"><use href="#icon-plus"/></svg>Create capability</button>
          </div>
        </form>
      </section>
    </div>`},i=(await t(50)).data||[];if(!n?.length){e.innerHTML=r({projects:n,users:i,autofocus:!0,error:"No projects in this workspace yet. Create one with `POST /api/v1/workspaces/{id}/projects` first, then reload."}),l(e,r,{projects:n,users:i});return}e.innerHTML=r({projects:n,users:i,autofocus:!0}),l(e,r,{projects:n,users:i})}function c(e){return`<div class="rounded-xl border border-line bg-paper p-3">
    <div class="flex items-center justify-between"><div><p class="text-[.7rem] font-bold">Created</p><p class="mt-1 text-[.62rem] text-muted mono">${a(e.id)}</p></div>${e.environment?`<span class="status-pill warn !px-2 !py-1">${a(e.environment)} pending</span>`:``}</div>
    <div class="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
      ${o(`Capability`,e.id)}
      ${e.versionId?o(`Version`,e.versionId):``}
      ${e.releaseId?o(`Release`,e.releaseId):``}
      ${e.environment?o(`Env`,e.environment):``}
    </div>
  </div>`}function l(t,a,o){let s=t.querySelector(`#new-capability-form`);s?.addEventListener(`submit`,async c=>{c.preventDefault();let f=new FormData(s),p=(f.get(`project_id`)||``).toString(),m=(f.get(`tags`)||``).toString().split(`,`).map(e=>e.trim()).filter(Boolean),h={name:(f.get(`name`)||``).toString().trim(),description:(f.get(`description`)||``).toString().trim()||void 0,owner:(f.get(`owner`)||``).toString()||void 0,tags:m.length?m:void 0},g=(f.get(`environment`)||``).toString();if(!p)return u(t,a,o,null,`Pick a project first.`);let _=s.querySelector(`button[type=submit]`);_&&(_.disabled=!0);let v=await n(p,h);if(!v.ok)return _&&(_.disabled=!1),u(t,a,o,v,i(v));let y={id:v.data.id};if(g){let n=(f.get(`prompt`)||``).toString().trim()||`You are a ${h.name.toLowerCase()} assistant.`,s=(f.get(`model`)||``).toString().trim()||`gpt-4o-mini`,c=await r(v.data.id,{version:1,manifest:{prompt:{kind:`prompt`,hash:await d(n)},model_policy:{kind:`model_policy`,hash:await d(JSON.stringify({model:s}))},runtime_policy:{kind:`runtime_policy`,hash:await d(JSON.stringify({max_tokens:600,timeout_sec:30}))},context_contract:{kind:`context_contract`,hash:await d(JSON.stringify({max_input_tokens:12e3}))},memory:{kind:`memory`,hash:await d(JSON.stringify({kind:`ephemeral`}))}}});if(!c.ok)return _&&(_.disabled=!1),u(t,a,o,c,`Created capability but version failed: ${i(c)}`);y.versionId=c.data.id;let l=await e(c.data.id,g);if(!l.ok)return _&&(_.disabled=!1),u(t,a,o,l,`Created capability+version but release failed: ${i(l)}`);y.releaseId=l.data.id,y.environment=g}_&&(_.disabled=!1),t.innerHTML=a({projects:o.projects,users:o.users,autofocus:!1,created:y}),l(t,a,o),window.location.reload()}),t.querySelector(`[data-close-modal]`)?.addEventListener(`click`,()=>t.replaceChildren())}function u(e,t,n,r,i){e.innerHTML=t({projects:n.projects,users:n.users,autofocus:!1,error:i}),l(e,t,n)}async function d(e){let t=new TextEncoder().encode(e),n=await crypto.subtle.digest(`SHA-256`,t);return Array.from(new Uint8Array(n)).map(e=>e.toString(16).padStart(2,`0`)).join(``)}export{s as openNewCapabilityModal};