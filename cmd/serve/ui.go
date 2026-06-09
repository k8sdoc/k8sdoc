package serve

const webUI = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Kubernetes AI Assistant</title>
<style>
:root{--bg:#0d1117;--sf:#161b22;--sf2:#1c2333;--bd:#30363d;--ac:#58a6ff;--gr:#3fb950;--am:#f0883e;--rd:#f85149;--mg:#bc8cff;--cy:#39d353;--txt:#e6edf3;--tx2:#8b949e;--ubg:#1f3251;--aibg:#1c2333;--cbg:#010409;--tbh:#21262d;}
*{box-sizing:border-box;margin:0;padding:0;}
body{font-family:'SF Mono','Fira Code',monospace;background:var(--bg);color:var(--txt);height:100vh;display:flex;flex-direction:column;overflow:hidden;}
header{background:var(--sf);border-bottom:1px solid var(--bd);padding:0 16px;height:50px;display:flex;align-items:center;gap:10px;flex-shrink:0;}
.logo{font-size:17px;font-weight:700;color:var(--ac);letter-spacing:-.5px;}.logo span{color:var(--gr);}
.hdr-sep{color:var(--bd);}
.ns-pill{background:var(--sf2);border:1px solid var(--bd);border-radius:4px;padding:2px 8px;font-size:11px;color:var(--tx2);cursor:pointer;display:flex;align-items:center;gap:4px;}
.ns-pill:hover{border-color:var(--ac);color:var(--txt);}
.hdr-r{margin-left:auto;display:flex;align-items:center;gap:10px;}
.sdot{width:7px;height:7px;border-radius:50%;background:var(--gr);animation:pulse 2s ease-in-out infinite;}
.sdot.busy{background:var(--am);animation:none;}
.slabel{font-size:11px;color:var(--tx2);}
.hbtn{background:none;border:1px solid var(--bd);color:var(--tx2);padding:3px 9px;border-radius:4px;font-size:11px;cursor:pointer;font-family:inherit;}
.hbtn:hover{border-color:var(--ac);color:var(--txt);}
@keyframes pulse{0%,100%{opacity:1}50%{opacity:.4}}
.layout{display:flex;flex:1;overflow:hidden;}
/* Sidebar */
.sidebar{width:210px;background:var(--sf);border-right:1px solid var(--bd);display:flex;flex-direction:column;overflow-y:auto;flex-shrink:0;}
.sb-sec{padding:10px 6px 2px;}
.sb-lbl{font-size:9px;color:var(--tx2);text-transform:uppercase;letter-spacing:.8px;padding:0 8px;margin-bottom:3px;}
.qbtn{display:flex;align-items:center;gap:7px;padding:6px 9px;border-radius:4px;font-size:11px;color:var(--tx2);cursor:pointer;border:none;background:none;width:100%;text-align:left;font-family:inherit;transition:all .1s;}
.qbtn:hover{background:var(--sf2);color:var(--txt);}
.qbtn .ic{font-size:12px;width:14px;text-align:center;flex-shrink:0;}
.sb-div{height:1px;background:var(--bd);margin:6px 10px;}
/* Chat */
.chat-main{flex:1;display:flex;flex-direction:column;overflow:hidden;}
.messages{flex:1;overflow-y:auto;padding:16px;display:flex;flex-direction:column;gap:14px;scroll-behavior:smooth;}
.messages::-webkit-scrollbar{width:5px;}.messages::-webkit-scrollbar-thumb{background:var(--bd);border-radius:3px;}
/* Messages */
.msg{display:flex;gap:10px;max-width:100%;}
.msg.user{flex-direction:row-reverse;}
.av{width:28px;height:28px;border-radius:5px;display:flex;align-items:center;justify-content:center;font-size:11px;font-weight:700;flex-shrink:0;}
.av.u{background:var(--ubg);color:var(--ac);}
.av.a{background:var(--gr);color:#000;}
.bbl{max-width:calc(100% - 48px);border-radius:7px;padding:10px 14px;font-size:12.5px;line-height:1.7;word-break:break-word;}
.bbl.u{background:var(--ubg);border:1px solid #2d4a7a;}
.bbl.a{background:var(--aibg);border:1px solid var(--bd);}
.bbl.err{background:#1f1218;border:1px solid var(--rd);color:var(--rd);}
.bbl strong{color:var(--ac);}
.bbl code{background:var(--cbg);border:1px solid var(--bd);padding:1px 4px;border-radius:3px;font-size:11px;color:var(--gr);}
.bbl pre{background:var(--cbg);border:1px solid var(--bd);border-radius:5px;padding:10px;overflow-x:auto;margin:6px 0;font-size:11px;line-height:1.5;color:var(--gr);}
.bbl ul,.bbl ol{padding-left:18px;margin:3px 0;}.bbl li{margin:1px 0;}
.bbl h3{color:var(--ac);margin:8px 0 3px;font-size:12px;}
.bbl p{margin:3px 0;}
.bbl a{color:var(--ac);text-decoration:none;}.bbl a:hover{text-decoration:underline;}
/* Tables */
.tbl-wrap{overflow-x:auto;margin:6px 0;border-radius:5px;border:1px solid var(--bd);}
.tbl-title{font-size:10px;color:var(--tx2);padding:6px 10px 4px;display:flex;align-items:center;gap:6px;background:var(--sf2);border-bottom:1px solid var(--bd);}
.tbl-cnt{background:var(--cbg);border:1px solid var(--bd);border-radius:10px;padding:0 6px;font-size:9px;}
table{border-collapse:collapse;width:100%;font-size:11px;}
th{background:var(--tbh);color:var(--tx2);text-align:left;padding:5px 10px;border-bottom:1px solid var(--bd);white-space:nowrap;font-weight:600;font-size:10px;text-transform:uppercase;letter-spacing:.4px;}
td{padding:4px 10px;border-bottom:1px solid var(--bd);color:var(--txt);white-space:nowrap;}
tr:last-child td{border-bottom:none;}
tr:hover td{background:rgba(255,255,255,.03);}
.ok{color:var(--gr);}
.s-running{color:var(--gr);}
.s-pending{color:var(--am);}
.s-failed,.s-error{color:var(--rd);}
.s-warning{color:var(--am);}
.s-info{color:var(--ac);}
.s-notready{color:var(--rd);}
/* Severity badges */
.sev-error{color:var(--rd);font-weight:600;}
.sev-warning{color:var(--am);}
.sev-info{color:var(--ac);}
.sev-ok{color:var(--gr);}
/* Log block */
.log-blk{background:var(--cbg);border:1px solid var(--bd);border-radius:5px;padding:10px;overflow:auto;max-height:380px;font-size:10.5px;line-height:1.6;color:var(--tx2);margin:6px 0;white-space:pre;font-family:inherit;}
.log-blk .le{color:var(--rd);}
.log-blk .lw{color:var(--am);}
.log-blk .li{color:var(--gr);}
/* Score badge */
.score-badge{display:inline-flex;align-items:center;gap:4px;padding:2px 8px;border-radius:10px;font-size:10px;font-weight:700;}
.score-good{background:#0d2b17;color:var(--gr);border:1px solid var(--gr);}
.score-warn{background:#2b1d0d;color:var(--am);border:1px solid var(--am);}
.score-bad{background:#2b0d0d;color:var(--rd);border:1px solid var(--rd);}
/* Typing / thinking */
.tcursor{display:inline-block;width:2px;height:13px;background:var(--ac);margin-left:2px;vertical-align:middle;animation:blink .8s step-end infinite;}
@keyframes blink{0%,100%{opacity:1}50%{opacity:0}}
.thinking{display:flex;align-items:center;gap:6px;color:var(--tx2);font-size:11px;}
.dots{display:flex;gap:3px;}
.dots span{width:5px;height:5px;border-radius:50%;background:var(--tx2);animation:bounce 1.2s ease-in-out infinite;}
.dots span:nth-child(2){animation-delay:.2s;}
.dots span:nth-child(3){animation-delay:.4s;}
@keyframes bounce{0%,80%,100%{transform:scale(.6);opacity:.4}40%{transform:scale(1);opacity:1}}
/* Welcome */
.welcome{display:flex;flex-direction:column;align-items:center;justify-content:center;height:100%;gap:20px;color:var(--tx2);text-align:center;padding:40px;}
.wlogo{font-size:44px;}
.welcome h2{font-size:18px;color:var(--txt);font-weight:600;}
.welcome p{font-size:12px;max-width:460px;line-height:1.7;}
.wgrid{display:grid;grid-template-columns:repeat(3,1fr);gap:8px;margin-top:6px;width:100%;max-width:600px;}
.wchip{background:var(--sf2);border:1px solid var(--bd);border-radius:5px;padding:9px 12px;font-size:11px;text-align:left;cursor:pointer;color:var(--tx2);transition:all .15s;font-family:inherit;}
.wchip:hover{border-color:var(--ac);color:var(--txt);background:var(--sf);}
.wchip .ci{font-size:14px;display:block;margin-bottom:3px;}
/* Input */
.inp-area{border-top:1px solid var(--bd);padding:12px 16px;background:var(--sf);display:flex;gap:8px;align-items:flex-end;flex-shrink:0;}
.inp-col{flex:1;display:flex;flex-direction:column;gap:5px;}
.ns-row{display:flex;align-items:center;gap:6px;font-size:10px;color:var(--tx2);}
.ns-inp{background:var(--sf2);border:1px solid var(--bd);border-radius:3px;color:var(--txt);font-family:inherit;font-size:10px;padding:2px 7px;width:120px;outline:none;}
.ns-inp:focus{border-color:var(--ac);}
.inp-wrap{background:var(--sf2);border:1px solid var(--bd);border-radius:7px;display:flex;align-items:flex-end;gap:6px;padding:0 6px 0 12px;transition:border-color .15s;}
.inp-wrap:focus-within{border-color:var(--ac);}
textarea{flex:1;background:none;border:none;outline:none;color:var(--txt);font-family:inherit;font-size:12px;padding:10px 0;resize:none;min-height:40px;max-height:150px;line-height:1.5;}
textarea::placeholder{color:var(--tx2);}
.inp-btns{display:flex;align-items:center;gap:4px;margin-bottom:4px;}
.ibtn{width:30px;height:30px;border-radius:5px;border:none;cursor:pointer;display:flex;align-items:center;justify-content:center;font-size:14px;font-family:inherit;transition:all .15s;}
.ibtn-send{background:var(--ac);color:#000;}
.ibtn-send:hover{background:#79b8ff;}
.ibtn-send:disabled{opacity:.4;cursor:not-allowed;}
.ibtn-upload{background:var(--sf);border:1px solid var(--bd);color:var(--tx2);}
.ibtn-upload:hover{border-color:var(--ac);color:var(--ac);}
/* File drop zone */
.dropzone{border:2px dashed var(--bd);border-radius:7px;padding:20px;text-align:center;color:var(--tx2);font-size:11px;cursor:pointer;transition:all .2s;margin:4px 0;}
.dropzone:hover,.dropzone.dragover{border-color:var(--ac);color:var(--txt);background:rgba(88,166,255,.05);}
.dropzone .dz-icon{font-size:28px;margin-bottom:6px;}
/* Modal */
.modal-bg{display:none;position:fixed;inset:0;background:rgba(0,0,0,.7);z-index:100;align-items:center;justify-content:center;}
.modal-bg.open{display:flex;}
.modal{background:var(--sf);border:1px solid var(--bd);border-radius:8px;width:560px;max-width:95vw;padding:20px;display:flex;flex-direction:column;gap:12px;}
.modal h3{font-size:14px;color:var(--txt);}
.modal-close{position:absolute;top:14px;right:14px;background:none;border:none;color:var(--tx2);cursor:pointer;font-size:18px;}
.mtextarea{background:var(--sf2);border:1px solid var(--bd);border-radius:5px;color:var(--txt);font-family:inherit;font-size:11px;padding:10px;resize:vertical;min-height:200px;width:100%;outline:none;line-height:1.6;}
.mtextarea:focus{border-color:var(--ac);}
.modal-btns{display:flex;gap:8px;justify-content:flex-end;}
.mbtn{padding:6px 14px;border-radius:5px;border:none;cursor:pointer;font-family:inherit;font-size:12px;}
.mbtn-primary{background:var(--ac);color:#000;}
.mbtn-primary:hover{background:#79b8ff;}
.mbtn-cancel{background:var(--sf2);border:1px solid var(--bd);color:var(--tx2);}
.mbtn-cancel:hover{color:var(--txt);}
.action-row{display:flex;flex-wrap:wrap;gap:6px;margin:6px 0 6px 38px;}
.act-btn{padding:5px 12px;border-radius:5px;border:none;cursor:pointer;font-family:inherit;font-size:11px;font-weight:600;transition:all .15s;}
.act-primary{background:var(--ac);color:#000;}.act-primary:hover{background:#79b8ff;}
.act-danger{background:var(--rd);color:#fff;}.act-danger:hover{opacity:.85;}
.act-ghost{background:var(--sf2);border:1px solid var(--bd);color:var(--tx2);}.act-ghost:hover{color:var(--txt);}
</style>
</head>
<body>
<header>
  <div class="logo">k8s<span>doc</span></div>
  <span class="hdr-sep">│</span>
  <div class="ns-pill" onclick="focusNs()">⎈ <span id="nsPillTxt">all namespaces</span></div>
  <div class="hdr-r">
    <div class="sdot" id="sdot"></div>
    <span class="slabel" id="slabel">Qwen ready</span>
    <button class="hbtn" onclick="sendQuick('security scan — check for privileged containers, missing limits, and security issues')">🔒 Security</button>
    <button class="hbtn" onclick="sendQuick('run full k8sdoc analysis and find all issues')">⚡ Full Scan</button>
    <button class="hbtn" onclick="openManifestModal()">📋 Lint YAML</button>
    <button class="hbtn" onclick="clearChat()">✕ Clear</button>
  </div>
</header>

<div class="layout">
<div class="sidebar">
  <div class="sb-sec">
    <div class="sb-lbl">Resources</div>
    <button class="qbtn" onclick="sendQuick('show all pods')"><span class="ic">◉</span>Pods</button>
    <button class="qbtn" onclick="sendQuick('list deployments')"><span class="ic">⬡</span>Deployments</button>
    <button class="qbtn" onclick="sendQuick('list services')"><span class="ic">⬢</span>Services</button>
    <button class="qbtn" onclick="sendQuick('list nodes')"><span class="ic">▣</span>Nodes</button>
    <button class="qbtn" onclick="sendQuick('list namespaces')"><span class="ic">⊟</span>Namespaces</button>
    <button class="qbtn" onclick="sendQuick('list ingresses')"><span class="ic">→</span>Ingresses</button>
    <button class="qbtn" onclick="sendQuick('list pvcs')"><span class="ic">⊞</span>PVCs</button>
    <button class="qbtn" onclick="sendQuick('list configmaps')"><span class="ic">⊟</span>ConfigMaps</button>
    <button class="qbtn" onclick="sendQuick('list secrets')"><span class="ic">🔑</span>Secrets</button>
    <button class="qbtn" onclick="sendQuick('list daemonsets')"><span class="ic">◈</span>DaemonSets</button>
    <button class="qbtn" onclick="sendQuick('list statefulsets')"><span class="ic">◫</span>StatefulSets</button>
    <button class="qbtn" onclick="sendQuick('list jobs')"><span class="ic">▷</span>Jobs</button>
  </div>
  <div class="sb-div"></div>
  <div class="sb-sec">
    <div class="sb-lbl">Diagnostics</div>
    <button class="qbtn" onclick="sendQuick('show warning events')"><span class="ic">⚠</span>Events</button>
    <button class="qbtn" onclick="sendQuick('diagnose all issues in the cluster')"><span class="ic">🔍</span>Diagnose</button>
    <button class="qbtn" onclick="sendQuick('cluster health summary')"><span class="ic">♥</span>Health</button>
    <button class="qbtn" onclick="sendQuick('show pods that are failing or pending or have errors')"><span class="ic">✕</span>Failed Pods</button>
    <button class="qbtn" onclick="sendQuick('find pods with crashloopbackoff or imagepullbackoff')"><span class="ic">↺</span>CrashLoops</button>
    <button class="qbtn" onclick="sendQuick('check network connectivity and dns status')"><span class="ic">⇄</span>Network</button>
  </div>
  <div class="sb-div"></div>
  <div class="sb-sec">
    <div class="sb-lbl">Security</div>
    <button class="qbtn" onclick="sendQuick('security scan — privileged containers, root access, missing limits')"><span class="ic">🔒</span>Sec Scan</button>
    <button class="qbtn" onclick="sendQuick('check for containers running as root or with no security context')"><span class="ic">👤</span>Root Check</button>
    <button class="qbtn" onclick="sendQuick('find containers with no resource limits')"><span class="ic">📊</span>Limits</button>
  </div>
  <div class="sb-div"></div>
  <div class="sb-sec">
    <div class="sb-lbl">Config Linting</div>
    <button class="qbtn" onclick="openManifestModal()"><span class="ic">📋</span>Lint YAML</button>
    <button class="qbtn" onclick="openManifestModal('deployment')"><span class="ic">📄</span>Lint Deploy</button>
    <button class="qbtn" onclick="openManifestModal('service')"><span class="ic">📄</span>Lint Service</button>
  </div>
  <div class="sb-div"></div>
  <div class="sb-sec">
    <div class="sb-lbl">kdoctor</div>
    <button class="qbtn" onclick="sendQuick('run full k8sdoc analysis and find all issues')"><span class="ic">⚡</span>Full Scan</button>
    <button class="qbtn" onclick="sendQuick('check network reachability between pods')"><span class="ic">⇄</span>NetReach</button>
    <button class="qbtn" onclick="sendQuick('check dns resolution status in cluster')"><span class="ic">⎔</span>DNS</button>
  </div>
  <div class="sb-div"></div>
  <div class="sb-sec">
    <div class="sb-lbl">K8s Knowledge</div>
    <button class="qbtn" onclick="sendQuick('explain how kubernetes networking works — services, DNS, CNI')"><span class="ic">🌐</span>Networking</button>
    <button class="qbtn" onclick="sendQuick('explain kubernetes storage — PV, PVC, StorageClass')"><span class="ic">💾</span>Storage</button>
    <button class="qbtn" onclick="sendQuick('explain RBAC in kubernetes — roles, rolebindings, serviceaccounts')"><span class="ic">🔑</span>RBAC</button>
    <button class="qbtn" onclick="sendQuick('what causes OOMKilled and how to fix it')"><span class="ic">💀</span>OOMKilled</button>
    <button class="qbtn" onclick="sendQuick('what causes CrashLoopBackOff and how to debug it')"><span class="ic">↺</span>CrashLoop</button>
    <button class="qbtn" onclick="sendQuick('explain HPA — horizontal pod autoscaler and how to configure it')"><span class="ic">⇕</span>HPA</button>
    <button class="qbtn" onclick="sendQuick('kubectl cheat sheet — most useful commands')"><span class="ic">📋</span>kubectl tips</button>
    <button class="qbtn" onclick="sendQuick('how to debug a pod that is not starting')"><span class="ic">🔍</span>Debug pods</button>
  </div>
</div>

<div class="chat-main">
  <div class="messages" id="messages">
    <div class="welcome" id="welcome">
      <div class="wlogo">⎈</div>
      <h2>k8sdoc — Kubernetes AI Assistant</h2>
      <p>Ask anything about your cluster, upload YAML manifests for linting, diagnose errors, check security, and get exact kubectl fixes — all powered by your local Qwen model.</p>
      <div class="wgrid">
        <button class="wchip" onclick="sendQuick('show all pods and their status')"><span class="ci">◉</span>"show all pods"</button>
        <button class="wchip" onclick="sendQuick('why is my pod crashing?')"><span class="ci">🔍</span>"why is pod crashing?"</button>
        <button class="wchip" onclick="sendQuick('show warning events in the cluster')"><span class="ci">⚠</span>"warning events"</button>
        <button class="wchip" onclick="openManifestModal()"><span class="ci">📋</span>Lint a YAML file</button>
        <button class="wchip" onclick="sendQuick('security scan — check for privileged containers and missing limits')"><span class="ci">🔒</span>Security scan</button>
        <button class="wchip" onclick="sendQuick('run full k8sdoc analysis and find all issues')"><span class="ci">⚡</span>Full diagnostics</button>
      </div>
    </div>
  </div>

  <div class="inp-area">
    <div class="inp-col">
      <div class="ns-row">
        <span>namespace:</span>
        <input class="ns-inp" id="nsInp" placeholder="all namespaces"
          onchange="updateNs(this.value)" onkeydown="if(event.key==='Enter')this.blur()">
      </div>
      <div class="inp-wrap">
        <textarea id="chatInp" rows="1"
          placeholder='Ask anything: "show pods", "why is nginx crashing?", "check my yaml", "security scan"...'
          onkeydown="handleKey(event)" oninput="autoR(this)"></textarea>
        <div class="inp-btns">
          <button class="ibtn ibtn-upload" title="Upload/paste YAML manifest" onclick="openManifestModal()">📋</button>
          <button class="ibtn ibtn-send" id="sendBtn" onclick="sendMsg()">↑</button>
        </div>
      </div>
    </div>
  </div>
</div>
</div>

<!-- Manifest lint modal -->
<div class="modal-bg" id="manifestModal" onclick="if(event.target===this)closeManifestModal()">
  <div class="modal" style="position:relative">
    <h3>📋 Lint Kubernetes YAML Manifest</h3>
    <button class="modal-close" onclick="closeManifestModal()">✕</button>
    <div class="dropzone" id="dropzone"
      ondragover="event.preventDefault();this.classList.add('dragover')"
      ondragleave="this.classList.remove('dragover')"
      ondrop="handleDrop(event)"
      onclick="document.getElementById('fileInput').click()">
      <div class="dz-icon">📂</div>
      <div>Drop YAML/JSON file here or click to browse</div>
      <div style="font-size:9px;margin-top:4px;color:var(--tx2)">Supports: Deployment, Service, Ingress, ConfigMap, Secret, PVC, HPA, NetworkPolicy</div>
    </div>
    <input type="file" id="fileInput" style="display:none" accept=".yaml,.yml,.json" onchange="handleFileSelect(event)">
    <div style="font-size:10px;color:var(--tx2);text-align:center;">— or paste YAML below —</div>
    <textarea class="mtextarea" id="manifestInput" placeholder="apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 1
  ..."></textarea>
    <div style="font-size:10px;color:var(--tx2)" id="filenameLabel"></div>
    <div class="modal-btns">
      <button class="mbtn mbtn-cancel" onclick="closeManifestModal()">Cancel</button>
      <button class="mbtn mbtn-primary" onclick="submitManifest()">⚡ Analyze Manifest</button>
    </div>
  </div>
</div>

<script>
const SID = 'sid-' + Date.now();
let ns = '';
let streaming = false;
let manifestFilename = 'manifest.yaml';

// ── Namespace ─────────────────────────────────────────────────────────────────
function updateNs(v) {
  ns = v.trim();
  document.getElementById('nsPillTxt').textContent = ns || 'all namespaces';
}
function focusNs() { document.getElementById('nsInp').focus(); }

// ── Input helpers ─────────────────────────────────────────────────────────────
function autoR(el) { el.style.height='auto'; el.style.height=Math.min(el.scrollHeight,150)+'px'; }
function handleKey(e) { if(e.key==='Enter'&&!e.shiftKey){e.preventDefault();sendMsg();} }
function sendQuick(msg) { document.getElementById('chatInp').value=msg; sendMsg(); }
function clearChat() {
  const m=document.getElementById('messages'); m.innerHTML='';
  const w=document.createElement('div'); w.className='welcome'; w.id='welcome';
  w.innerHTML='<div class="wlogo">⎈</div><h2>Chat cleared</h2><p>Ask me anything about your cluster.</p>';
  m.appendChild(w);
}

// ── Send message ──────────────────────────────────────────────────────────────
function sendMsg() {
  const inp=document.getElementById('chatInp');
  const msg=inp.value.trim();
  if(!msg||streaming) return;
  const w=document.getElementById('welcome'); if(w) w.remove();
  appendMsg('user', msg);
  inp.value=''; inp.style.height='auto';
  const tid=showThinking();
  streaming=true; setLoading(true);

  fetch('/api/v1/chat',{
    method:'POST',
    headers:{'Content-Type':'application/json'},
    body:JSON.stringify({sessionId:SID, message:msg, namespace:ns})
  }).then(async r=>{
    removeEl(tid);
    const aid=appendAI();
    await readSSE(r, aid);
    finishMsg(aid);
  }).catch(err=>{
    removeEl(tid);
    appendMsg('error','Connection error: '+err.message);
  }).finally(()=>{streaming=false;setLoading(false);});
}

// ── Manifest modal ────────────────────────────────────────────────────────────
function openManifestModal(hint) {
  document.getElementById('manifestModal').classList.add('open');
  if(hint) {
    const templates = {
      deployment: "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: my-app\nspec:\n  replicas: 1\n  selector:\n    matchLabels:\n      app: my-app\n  template:\n    metadata:\n      labels:\n        app: my-app\n    spec:\n      containers:\n      - name: my-app\n        image: nginx:latest\n",
      service: "apiVersion: v1\nkind: Service\nmetadata:\n  name: my-service\nspec:\n  selector:\n    app: my-app\n  ports:\n  - port: 80\n    targetPort: 8080\n"
    };
    if(templates[hint]) document.getElementById('manifestInput').value = templates[hint];
  }
}
function closeManifestModal() {
  document.getElementById('manifestModal').classList.remove('open');
  document.getElementById('manifestInput').value='';
  document.getElementById('filenameLabel').textContent='';
  manifestFilename='manifest.yaml';
}

function handleDrop(e) {
  e.preventDefault();
  document.getElementById('dropzone').classList.remove('dragover');
  const file = e.dataTransfer.files[0];
  if(file) loadFile(file);
}
function handleFileSelect(e) {
  const file = e.target.files[0];
  if(file) loadFile(file);
}
function loadFile(file) {
  manifestFilename = file.name;
  document.getElementById('filenameLabel').textContent = '📄 ' + file.name;
  const reader = new FileReader();
  reader.onload = e => { document.getElementById('manifestInput').value = e.target.result; };
  reader.readAsText(file);
}

function submitManifest() {
  const content = document.getElementById('manifestInput').value.trim();
  if(!content) { alert('Please paste or upload a YAML manifest.'); return; }
  closeManifestModal();

  const w=document.getElementById('welcome'); if(w) w.remove();
  appendMsg('user', '📋 Analyzing manifest: ' + manifestFilename);
  const tid=showThinking();
  streaming=true; setLoading(true);

  fetch('/api/v1/manifest',{
    method:'POST',
    headers:{'Content-Type':'application/json'},
    body:JSON.stringify({sessionId:SID, filename:manifestFilename, content:content})
  }).then(async r=>{
    removeEl(tid);
    const aid=appendAI();
    await readSSE(r, aid);
    finishMsg(aid);
  }).catch(err=>{
    removeEl(tid);
    appendMsg('error','Error: '+err.message);
  }).finally(()=>{streaming=false;setLoading(false);});
}

// ── SSE reader ────────────────────────────────────────────────────────────────
async function readSSE(resp, msgId) {
  const reader=resp.body.getReader();
  const dec=new TextDecoder();
  let buf='';
  while(true) {
    const {done,value}=await reader.read();
    if(done) break;
    buf+=dec.decode(value,{stream:true});
    const lines=buf.split('\n');
    buf=lines.pop();
    for(const line of lines) {
      if(!line.startsWith('data: ')) continue;
      const json=line.slice(6).trim();
      if(!json) continue;
      try { handleChunk(JSON.parse(json), msgId); } catch(e){}
    }
  }
}

// ── Chunk handler ─────────────────────────────────────────────────────────────
function handleChunk(chunk, msgId) {
  const el=document.getElementById(msgId);
  if(!el) return;
  const bbl=el.querySelector('.bbl');

  switch(chunk.type) {
    case 'token': {
      const tokContent = chunk.content || '';
      if(!tokContent) break; // skip empty keepalive tokens
      let txt=el.querySelector('.btxt');
      if(!txt){ txt=document.createElement('div'); txt.className='btxt'; bbl.appendChild(txt); }
      txt.dataset.raw=(txt.dataset.raw||'')+tokContent;
      renderMd(txt);
      scrollBot();
      break;
    }
    case 'data': {
      const d=document.createElement('div');
      if(chunk.dataType==='table'&&chunk.data&&chunk.data.headers) {
        d.innerHTML=renderTable(chunk.data);
      } else if(chunk.dataType==='yaml'||typeof chunk.data==='string') {
        d.innerHTML=renderLog(chunk.data);
      }
      bbl.prepend(d);
      scrollBot();
      break;
    }
    case 'error':
      bbl.classList.add('err');
      let etxt=el.querySelector('.btxt');
      if(!etxt){ etxt=document.createElement('div'); etxt.className='btxt'; bbl.appendChild(etxt); }
      etxt.textContent='⚠ '+chunk.content;
      break;
    case 'confirm': {
      // Render action buttons below the AI message
      if (chunk.actions && chunk.actions.length > 0) {
        const btnRow = document.createElement('div');
        btnRow.className = 'action-row';
        btnRow.id = 'ar-' + msgId;
        for (const act of chunk.actions) {
          const btn = document.createElement('button');
          btn.className = 'act-btn act-' + (act.style || 'primary');
          btn.textContent = act.label;
          btn.onclick = () => {
            document.getElementById('ar-' + msgId)?.remove();
            if (act.command !== '__skip__') {
              sendAction(act.command);
            }
          };
          btnRow.appendChild(btn);
        }
        const el = document.getElementById(msgId);
        if (el) el.after(btnRow);
      }
      break;
    }
    case 'done':
      finishMsg(msgId);
      break;
  }
}

// ── Message builders ──────────────────────────────────────────────────────────
function appendMsg(role, text) {
  const id='m'+Date.now();
  const m=document.getElementById('messages');
  const d=document.createElement('div');
  d.className='msg'+(role==='user'?' user':'');
  d.id=id;
  if(role==='user') {
    d.innerHTML='<div class="av u">U</div><div class="bbl u">'+esc(text)+'</div>';
  } else {
    d.innerHTML='<div class="av a">!</div><div class="bbl err">'+esc(text)+'</div>';
  }
  m.appendChild(d); scrollBot(); return id;
}

function appendAI() {
  const id='m'+Date.now();
  const m=document.getElementById('messages');
  const d=document.createElement('div');
  d.className='msg'; d.id=id;
  d.innerHTML='<div class="av a">K</div><div class="bbl a"><span class="tcursor"></span></div>';
  m.appendChild(d); scrollBot(); return id;
}

function showThinking() {
  const id='t'+Date.now();
  const m=document.getElementById('messages');
  const d=document.createElement('div');
  d.className='msg'; d.id=id;
  d.innerHTML='<div class="av a">K</div><div class="bbl a"><div class="thinking"><span id="'+id+'-lbl">Classifying query</span><div class="dots"><span></span><span></span><span></span></div></div></div>';
  m.appendChild(d); scrollBot();
  // Progress through thinking stages
  const labels = ["Classifying query", "Fetching cluster data", "Generating answer"];
  let i = 0;
  const timer = setInterval(() => {
    i = Math.min(i+1, labels.length-1);
    const lbl = document.getElementById(id+'-lbl');
    if(lbl) lbl.textContent = labels[i];
    else clearInterval(timer);
  }, 1200);
  d._timer = timer;
  return id;
}

function finishMsg(id) {
  const el=document.getElementById(id);
  if(!el) return;
  const c=el.querySelector('.tcursor'); if(c) c.remove();
}
function removeEl(id) { const e=document.getElementById(id); if(e){ if(e._timer) clearInterval(e._timer); e.remove(); } }

// ── Markdown renderer ─────────────────────────────────────────────────────────
function renderMd(el) {
  let t=el.dataset.raw||'';
  // Fenced code blocks
  t=t.replace(/\x60\x60\x60(\w*)\n?([\s\S]*?)\x60\x60\x60/g,(_,l,c)=>'<pre><code>'+esc(c.trim())+'</code></pre>');
  // Inline code
  t=t.replace(/\x60([^\x60]+)\x60/g,'<code>$1</code>');
  // Bold
  t=t.replace(/\*\*([^*]+)\*\*/g,'<strong>$1</strong>');
  // Italic
  t=t.replace(/(?<!\*)\*([^*]+)\*(?!\*)/g,'<em>$1</em>');
  // H3
  t=t.replace(/^###\s+(.+)$/gm,'<h3>$1</h3>');
  t=t.replace(/^##\s+(.+)$/gm,'<h3>$1</h3>');
  t=t.replace(/^#\s+(.+)$/gm,'<h3>$1</h3>');
  // Bullet lists
  t=t.replace(/^[-*]\s+(.+)$/gm,'<li>$1</li>');
  // Numbered lists
  t=t.replace(/^\d+\.\s+(.+)$/gm,'<li>$1</li>');
  // Wrap consecutive li in ul
  t=t.replace(/(<li>.*?<\/li>\n?)+/gs, m=>'<ul>'+m+'</ul>');
  // Paragraphs
  t=t.replace(/\n\n/g,'</p><p>');
  t=t.replace(/\n/g,'<br>');
  el.innerHTML=t;
}

// ── Table renderer ────────────────────────────────────────────────────────────
function renderTable(data) {
  let h='<div class="tbl-wrap"><div class="tbl-title">';
  if(data.title) h+=esc(data.title)+'<span class="tbl-cnt">'+(data.rows?data.rows.length:0)+'</span>';
  h+='</div><table><thead><tr>';
  for(const col of (data.headers||[])) h+='<th>'+esc(col)+'</th>';
  h+='</tr></thead><tbody>';
  for(const row of (data.rows||[])) {
    h+='<tr>';
    for(const col of (data.headers||[])) {
      const v=row[col]||'';
      const cls=cellClass(col,v);
      h+='<td class="'+cls+'">'+esc(v)+'</td>';
    }
    h+='</tr>';
  }
  return h+'</tbody></table></div>';
}

function cellClass(col,val) {
  const v=val.toLowerCase();
  if(col==='SEVERITY'||col==='severity') {
    if(v==='error') return 'sev-error';
    if(v==='warning') return 'sev-warning';
    if(v==='info') return 'sev-info';
    if(v==='ok') return 'sev-ok';
  }
  if(col==='STATUS'||col==='READY') {
    if(v==='running'||v.startsWith('true')||v==='active'||v==='bound') return 's-running';
    if(v==='pending') return 's-pending';
    if(v==='failed'||v==='error'||v==='notready') return 's-failed';
    if(v==='warning') return 's-warning';
  }
  if(col==='TYPE'&&v==='warning') return 's-warning';
  return '';
}

function renderLog(content) {
  if(typeof content!=='string') return '';
  const c=esc(content)
    .replace(/(error|Error|ERROR|FATAL|fatal|CRITICAL|critical)/g,'<span class="le">$1</span>')
    .replace(/(warn|WARN|Warning|WARNING)/g,'<span class="lw">$1</span>')
    .replace(/(info|INFO|started|Starting|ready|Ready)/g,'<span class="li">$1</span>');
  return '<div class="log-blk">'+c+'</div>';
}

// ── Helpers ───────────────────────────────────────────────────────────────────
function scrollBot() { const m=document.getElementById('messages'); m.scrollTop=m.scrollHeight; }
function setLoading(on) {
  document.getElementById('sendBtn').disabled=on;
  const d=document.getElementById('sdot'), l=document.getElementById('slabel');
  if(on){d.className='sdot busy';l.textContent='Generating...';}
  else{d.className='sdot';l.textContent='Qwen ready';}
}
function esc(s) {
  if(typeof s!=='string') return '';
  return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;')
          .replace(/"/g,'&quot;').replace(/'/g,'&#039;');
}

// ── Action button handler ────────────────────────────────────────────────────
function sendAction(command) {
  const w = document.getElementById('welcome'); if(w) w.remove();
  // Show a compact user message
  const displayCmd = command.replace('__exec__:', '');
  appendMsg('user', '▶ ' + displayCmd);
  const tid = showThinking();
  streaming = true; setLoading(true);

  fetch('/api/v1/chat', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({sessionId: SID, message: command, namespace: ns})
  }).then(async r => {
    removeEl(tid);
    const aid = appendAI();
    await readSSE(r, aid);
    finishMsg(aid);
  }).catch(err => {
    removeEl(tid);
    appendMsg('error', 'Error: ' + err.message);
  }).finally(() => { streaming = false; setLoading(false); });
}

document.getElementById('chatInp').focus();
</script>
</body>
</html>`
