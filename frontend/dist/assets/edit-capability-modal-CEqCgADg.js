import{C as e,n as t,r as n}from"./index-BWv0qt4w.js";async function r(e,t){if(!e)return;let r=Array.isArray(t.tags)?t.tags.join(`, `):``,a=({error:e=null}={})=>`<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="edit-cap-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Capability / Edit</div><h2 id="edit-cap-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">${n(t.name)}</h2></div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form id="edit-cap-form" class="space-y-4 px-5 py-5 sm:px-6">
        <div><label class="eyebrow mb-2 block" for="ec-name">Name</label><input id="ec-name" name="name" class="field" required data-autofocus value="${n(t.name)}" /></div>
        <div><label class="eyebrow mb-2 block" for="ec-desc">Description</label><textarea id="ec-desc" name="description" class="field min-h-20 resize-y">${n(t.description||``)}</textarea></div>
        <div class="grid gap-3 sm:grid-cols-2">
          <div><label class="eyebrow mb-2 block" for="ec-owner">Owner ID</label><input id="ec-owner" name="owner" class="field mono" value="${n(t.owner||``)}" /></div>
          <div><label class="eyebrow mb-2 block" for="ec-tags">Tags</label><input id="ec-tags" name="tags" class="field" value="${n(r)}" placeholder="comma separated" /></div>
        </div>
        ${e?`<p class="rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800">${n(e)}</p>`:``}
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="quiet-button" data-close-modal>Cancel</button>
          <button type="submit" class="primary-button">Save</button>
        </div>
      </form>
    </section>
  </div>`;e.innerHTML=a(),i(e,a,t)}function i(n,r,a){n.querySelector(`[data-close-modal]`)?.addEventListener(`click`,()=>n.replaceChildren()),n.querySelector(`#edit-cap-form`)?.addEventListener(`submit`,async o=>{o.preventDefault();let s=new FormData(o.target),c=(s.get(`tags`)||``).toString().split(`,`).map(e=>e.trim()).filter(Boolean),l={name:(s.get(`name`)||``).toString().trim()||void 0,description:(s.get(`description`)||``).toString().trim()||void 0,owner:(s.get(`owner`)||``).toString().trim()||void 0,tags:c.length?c:[]},u=await e(a.id,l);if(!u.ok){n.innerHTML=r({error:t(u)}),i(n,r,a);return}window.location.reload()})}export{r as openEditCapabilityModal};