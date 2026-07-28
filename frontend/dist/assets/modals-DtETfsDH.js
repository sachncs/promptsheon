import{S as e,f as t,l as n,n as r,r as i,s as a,u as o}from"./index-BWv0qt4w.js";function s(e,t,n){return`<div class="modal-backdrop" role="presentation">
    <section class="modal-card" role="dialog" aria-modal="true" aria-labelledby="op-modal-title">
      <div class="flex items-start justify-between border-b border-line/70 px-5 py-5 sm:px-6">
        <div><div class="eyebrow">Operations</div><h2 id="op-modal-title" class="mt-2 text-[1.1rem] font-bold tracking-[-.04em]">${i(e)}</h2>${t?`<p class="mt-1 text-[.7rem] text-muted">${i(t)}</p>`:``}</div>
        <button class="icon-button !h-8 !w-8 !bg-paper" data-close-modal aria-label="Close dialog"><svg class="h-4 w-4 fill-none stroke-current stroke-2"><use href="#icon-close"/></svg></button>
      </div>
      <form class="space-y-4 px-5 py-5 sm:px-6">
        ${n}
        <p class="op-modal-error hidden rounded-lg bg-rose-50 px-3 py-2 text-[.68rem] text-rose-800"></p>
        <div class="flex flex-col-reverse gap-2 pt-1 sm:flex-row sm:justify-end">
          <button type="button" class="quiet-button" data-close-modal>Cancel</button>
          <button type="submit" class="primary-button">Save</button>
        </div>
      </form>
    </section>
  </div>`}function c(i,s){i.querySelector(`[data-close-modal]`)?.addEventListener(`click`,()=>i.replaceChildren()),i.querySelector(`form`)?.addEventListener(`submit`,async c=>{c.preventDefault();let l=new FormData(c.target),u=c.target.closest(`form`).dataset.op,d=i.querySelector(`.op-modal-error`);d.classList.add(`hidden`);let f=c.target.querySelector(`button[type=submit]`);f.disabled=!0;let p=null,m=null;try{u===`rule`?(p={name:(l.get(`name`)||``).toString().trim(),type:(l.get(`type`)||`threshold`).toString(),severity:(l.get(`severity`)||`medium`).toString(),threshold:Number(l.get(`threshold`)||0),duration_minutes:Number(l.get(`duration_minutes`)||0),window_minutes:Number(l.get(`window_minutes`)||0),enabled:l.get(`enabled`)===`on`},m=await a(p)):u===`group`?(p={name:(l.get(`name`)||``).toString().trim(),channels:(l.get(`channels`)||`log`).toString().split(`,`).map(e=>e.trim()).filter(Boolean)},m=await n(p)):u===`webhook`?(p={url:(l.get(`url`)||``).toString().trim(),events:(l.get(`events`)||``).toString().split(`,`).map(e=>e.trim()).filter(Boolean),secret:(l.get(`secret`)||``).toString()},m=await t(p)):u===`vault`?(p={provider_name:(l.get(`provider_name`)||``).toString().trim(),key_name:(l.get(`key_name`)||``).toString().trim(),key:(l.get(`key`)||``).toString()},m=await e(p)):u===`user`&&(p={email:(l.get(`email`)||``).toString().trim(),name:(l.get(`name`)||``).toString().trim(),role:(l.get(`role`)||`reader`).toString()},m=await o(p))}catch(e){d.textContent=String(e?.message||e),d.classList.remove(`hidden`),f.disabled=!1;return}if(f.disabled=!1,!m||!m.ok){d.textContent=r(m||{error:`no response`}),d.classList.remove(`hidden`);return}i.replaceChildren(),typeof s==`function`&&s()})}async function l(e,t){e&&(e.innerHTML=s(`New alert rule`,`Threshold + duration + window define when the rule fires.`,`
    <input type="hidden" name="op" value="rule" />
    <div><label class="eyebrow mb-2 block" for="ar-name">Name</label><input id="ar-name" name="name" class="field" required data-autofocus /></div>
    <div class="grid gap-3 sm:grid-cols-3">
      <div><label class="eyebrow mb-2 block" for="ar-type">Type</label><select id="ar-type" name="type" class="field"><option value="threshold">threshold</option><option value="latency_spike">latency_spike</option><option value="error_rate">error_rate</option></select></div>
      <div><label class="eyebrow mb-2 block" for="ar-sev">Severity</label><select id="ar-sev" name="severity" class="field"><option value="low">low</option><option value="medium">medium</option><option value="high">high</option><option value="critical">critical</option></select></div>
      <div><label class="eyebrow mb-2 block" for="ar-threshold">Threshold</label><input id="ar-threshold" name="threshold" type="number" step="any" class="field mono" value="0" /></div>
      <div><label class="eyebrow mb-2 block" for="ar-dur">Duration (min)</label><input id="ar-dur" name="duration_minutes" type="number" class="field mono" value="10" /></div>
      <div><label class="eyebrow mb-2 block" for="ar-win">Window (min)</label><input id="ar-win" name="window_minutes" type="number" class="field mono" value="5" /></div>
    </div>
    <label class="flex items-center gap-2 text-[.74rem]"><input type="checkbox" name="enabled" checked />Enabled</label>`),c(e,t))}async function u(e,t){e&&(e.innerHTML=s(`New notification group`,`Channels route alert events to log / webhook / etc.`,`
    <input type="hidden" name="op" value="group" />
    <div><label class="eyebrow mb-2 block" for="ng-name">Name</label><input id="ng-name" name="name" class="field" required data-autofocus /></div>
    <div><label class="eyebrow mb-2 block" for="ng-channels">Channels (comma)</label><input id="ng-channels" name="channels" class="field mono" placeholder="log, webhook" /></div>`),c(e,t))}async function d(e,t){e&&(e.innerHTML=s(`New webhook`,`Subscribers receive HMAC-signed payloads.`,`
    <input type="hidden" name="op" value="webhook" />
    <div><label class="eyebrow mb-2 block" for="wh-url">URL</label><input id="wh-url" name="url" type="url" class="field mono" required data-autofocus placeholder="https://example.com/hook" /></div>
    <div><label class="eyebrow mb-2 block" for="wh-events">Events (comma)</label><input id="wh-events" name="events" class="field mono" placeholder="release.activated, alert.fired" /></div>
    <div><label class="eyebrow mb-2 block" for="wh-secret">Signing secret</label><input id="wh-secret" name="secret" class="field mono" placeholder="(optional)" /></div>`),c(e,t))}async function f(e,t){e&&(e.innerHTML=s(`Save vault key`,`Master key stays on the daemon; this key is round-tripped only for delete.`,`
    <input type="hidden" name="op" value="vault" />
    <div class="grid gap-3 sm:grid-cols-2">
      <div><label class="eyebrow mb-2 block" for="vk-provider">Provider</label><input id="vk-provider" name="provider_name" class="field mono" required data-autofocus placeholder="openai" /></div>
      <div><label class="eyebrow mb-2 block" for="vk-name">Key name</label><input id="vk-name" name="key_name" class="field mono" required placeholder="prod-write" /></div>
    </div>
    <div><label class="eyebrow mb-2 block" for="vk-key">Key material</label><input id="vk-key" name="key" type="password" class="field mono" required placeholder="paste once, the dashboard will not display it again" /></div>`),c(e,t))}async function p(e,t){e&&(e.innerHTML=s(`Create user`,`Admin can mint a new admin key after creation.`,`
    <input type="hidden" name="op" value="user" />
    <div class="grid gap-3 sm:grid-cols-2">
      <div><label class="eyebrow mb-2 block" for="ur-name">Name</label><input id="ur-name" name="name" class="field" required data-autofocus /></div>
      <div><label class="eyebrow mb-2 block" for="ur-email">Email</label><input id="ur-email" name="email" type="email" class="field" required /></div>
    </div>
    <div><label class="eyebrow mb-2 block" for="ur-role">Role</label><select id="ur-role" name="role" class="field"><option value="admin">admin</option><option value="writer">writer</option><option value="reader">reader</option></select></div>`),c(e,t))}export{u as openNewGroupModal,l as openNewRuleModal,p as openNewUserModal,d as openNewWebhookModal,f as openVaultKeyModal};