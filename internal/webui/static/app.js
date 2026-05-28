const state = {
  view: "dashboard",
  snapshots: [],
  restoreId: null,
  jobPoll: null,
  config: null,
  configPath: "",
  settingsTab: "server",
  editRemoteName: null,
  providers: [],
};

const REDACTED = "••••••••";

const LABELS = {
  jobStatus: { idle: "空闲", running: "运行中", succeeded: "成功", failed: "失败" },
  jobOp: { backup: "备份", restore: "恢复", repo_verify: "校验", prune: "清理" },
  backupMode: { archive: "归档", directory: "目录", incremental: "增量" },
  controlType: { rcon: "RCON", console: "控制台", none: "离线模式" },
  snapStatus: { completed: "完成", failed: "失败", running: "运行中", pending: "等待" },
  select: {
    rcon: "RCON", console: "控制台", none: "离线（无需联动）",
    archive: "归档", directory: "目录", incremental: "增量",
    zstd: "Zstd", gzip: "Gzip", none: "不压缩",
    blake3: "BLAKE3", sha256: "SHA-256",
    debug: "Debug", info: "Info", warn: "Warn", error: "Error",
    text: "文本", json: "JSON",
  },
};

const SETTINGS_SECTIONS = [
  {
    id: "server", title: "服务器", desc: "Minecraft 服务器与世界路径，以及存档联动方式。",
    fields: [
      { path: "server.name", label: "名称", type: "text" },
      { path: "server.world_path", label: "世界路径", type: "text", hint: "存档目录的绝对路径" },
      { path: "server.control.type", label: "联动方式", type: "select", options: ["rcon", "console", "none"] },
      { path: "server.control.rcon.host", label: "RCON 地址", type: "text", showIf: (c) => c.server.control.type === "rcon" },
      { path: "server.control.rcon.port", label: "RCON 端口", type: "number", showIf: (c) => c.server.control.type === "rcon" },
      { path: "server.control.rcon.password", label: "RCON 密码", type: "password", placeholder: "留空保持不变", showIf: (c) => c.server.control.type === "rcon" },
      { path: "server.control.rcon.timeout", label: "连接超时", type: "text", hint: "如 10s", showIf: (c) => c.server.control.type === "rcon" },
      { path: "server.control.console.input_path", label: "输入管道", type: "text", showIf: (c) => c.server.control.type === "console" },
      { path: "server.control.console.output_path", label: "日志路径", type: "text", showIf: (c) => c.server.control.type === "console" },
    ],
  },
  {
    id: "backup", title: "备份", desc: "备份模式、压缩算法与 CDC 增量分块参数。",
    fields: [
      { path: "backup.mode", label: "备份模式", type: "select", options: ["archive", "directory", "incremental"] },
      { path: "backup.compression", label: "压缩方式", type: "select", options: ["zstd", "gzip", "none"] },
      { path: "backup.hash_method", label: "哈希算法", type: "select", options: ["blake3", "sha256"] },
      { path: "backup.staging_dir", label: "暂存目录", type: "text" },
      { path: "backup.lock_file", label: "锁文件", type: "text" },
      { path: "backup.safety_backup_local", label: "恢复前本地安全备份", type: "toggle", hint: "恢复前自动备份当前世界" },
      { path: "backup.exclude_patterns", label: "排除规则", type: "lines", hint: "每行一条 glob，如 session.lock" },
      { path: "backup.archive.include_paths", label: "归档包含路径", type: "lines", hint: "归档模式下额外包含的路径" },
      { path: "backup.cdc.enabled", label: "启用 CDC 分块", type: "toggle" },
      { path: "backup.cdc.min_size", label: "最小块大小（字节）", type: "number" },
      { path: "backup.cdc.avg_size", label: "平均块大小（字节）", type: "number" },
      { path: "backup.cdc.max_size", label: "最大块大小（字节）", type: "number" },
      { path: "backup.cdc.min_file_size", label: "启用 CDC 的最小文件", type: "number" },
    ],
  },
  {
    id: "repository", title: "本地仓库", desc: "SQLite 本地仓库路径与校验选项。",
    fields: [
      { path: "repository.local_path", label: "仓库路径", type: "text" },
      { path: "repository.cleanup_after_verified_upload", label: "上传校验后清理本地", type: "toggle" },
      { path: "repository.keep_local_manifests", label: "保留本地清单", type: "toggle" },
      { path: "repository.verify_after_backup", label: "备份后校验", type: "toggle" },
      { path: "repository.verify_after_upload", label: "上传后校验", type: "toggle" },
    ],
  },
  {
    id: "upload", title: "远程上传", desc: "备份完成后上传到云存储。",
    fields: [
      { path: "upload.enabled", label: "启用远程上传", type: "toggle" },
      { path: "rclone.remote", label: "Rclone 远程", type: "text", hint: "如 myremote:crypt" },
      { path: "rclone.remote_path", label: "远程目录", type: "text", hint: "云盘上的目标路径" },
    ],
  },
  {
    id: "transfer", title: "传输选项", desc: "Rclone 传输性能与超时设置。",
    fields: [
      { path: "rclone.bwlimit", label: "带宽限制", type: "text", hint: "如 10M，留空不限速" },
      { path: "rclone.transfers", label: "并发传输", type: "number" },
      { path: "rclone.checkers", label: "并发校验", type: "number" },
      { path: "rclone.timeout", label: "超时时间", type: "text", hint: "如 30m" },
      { path: "rclone.retries", label: "重试次数", type: "number" },
      { path: "rclone.extra_args", label: "额外参数", type: "lines", hint: "每行一个 rclone 参数" },
    ],
  },
  {
    id: "retention", title: "保留策略", desc: "自动清理时保留的快照数量，可在概览页预览和执行。",
    fields: [
      { path: "retention.daily", label: "每日保留", type: "number", hint: "最近 N 天的每日快照" },
      { path: "retention.weekly", label: "每周保留", type: "number", hint: "最近 N 周的每周快照" },
      { path: "retention.monthly", label: "每月保留", type: "number", hint: "最近 N 月的每月快照" },
    ],
  },
  {
    id: "schedule", title: "定时任务", desc: "Cron 定时备份，需另运行 snapcraft schedule run。",
    fields: [
      { path: "schedule.enabled", label: "启用定时备份", type: "toggle" },
      { path: "schedule.cron", label: "Cron 表达式", type: "text", hint: "如 0 4 * * *（每天 4:00）" },
    ],
  },
  {
    id: "notify", title: "通知", desc: "备份完成后的 Webhook 或邮件通知。",
    fields: [
      { path: "notify.webhook.enabled", label: "启用 Webhook", type: "toggle" },
      { path: "notify.webhook.url", label: "Webhook 地址", type: "text" },
      { path: "notify.email.enabled", label: "启用邮件", type: "toggle" },
      { path: "notify.email.smtp_host", label: "SMTP 服务器", type: "text" },
      { path: "notify.email.smtp_port", label: "SMTP 端口", type: "number" },
      { path: "notify.email.from", label: "发件人", type: "text" },
      { path: "notify.email.to", label: "收件人", type: "text" },
      { path: "notify.email.username", label: "用户名", type: "text" },
      { path: "notify.email.password", label: "密码", type: "password", placeholder: "留空保持不变" },
    ],
  },
  {
    id: "log", title: "日志", desc: "SnapCraft 运行日志级别与输出。",
    fields: [
      { path: "log.level", label: "日志级别", type: "select", options: ["debug", "info", "warn", "error"] },
      { path: "log.file", label: "日志文件", type: "text", hint: "留空输出到 stderr" },
      { path: "log.format", label: "输出格式", type: "select", options: ["text", "json"] },
    ],
  },
  {
    id: "webui", title: "WebUI", desc: "Web 控制台访问设置，修改地址后需重启。",
    fields: [
      { path: "webui.enabled", label: "启用 WebUI", type: "toggle" },
      { path: "webui.addr", label: "监听地址", type: "text", hint: "默认 127.0.0.1:7824" },
      { path: "webui.token", label: "访问令牌", type: "password", placeholder: "留空保持不变" },
      { path: "webui.cookie_name", label: "Cookie 名称", type: "text" },
    ],
  },
  { id: "rclone", title: "云存储远程", desc: "管理 Rclone 远程连接，供上传配置引用。" },
];

async function api(path, options = {}) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    credentials: "same-origin",
    ...options,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `请求失败 (${res.status})`);
  return data;
}

function $(id) { return document.getElementById(id); }
function show(el) { el.classList.remove("hidden"); }
function hide(el) { el.classList.add("hidden"); }

function toast(msg, isError = false) {
  const el = $("toast");
  el.textContent = msg;
  el.classList.toggle("error", isError);
  show(el);
  clearTimeout(state.toastTimer);
  state.toastTimer = setTimeout(() => hide(el), 3500);
}

function formatBytes(n) {
  if (!n) return "0 B";
  const u = ["B", "KB", "MB", "GB", "TB"];
  let i = 0, v = Number(n);
  while (v >= 1024 && i < u.length - 1) { v /= 1024; i++; }
  return `${v.toFixed(v >= 10 || i === 0 ? 0 : 1)} ${u[i]}`;
}

function formatTime(ts) {
  return ts ? new Date(ts).toLocaleString("zh-CN") : "—";
}

function getPath(obj, path) {
  return path.split(".").reduce((o, k) => (o == null ? undefined : o[k]), obj);
}

function setPath(obj, path, value) {
  const keys = path.split(".");
  let cur = obj;
  for (let i = 0; i < keys.length - 1; i++) {
    if (cur[keys[i]] == null) cur[keys[i]] = {};
    cur = cur[keys[i]];
  }
  cur[keys[keys.length - 1]] = value;
}

function setView(name) {
  state.view = name;
  document.querySelectorAll(".nav-item").forEach((b) => b.classList.toggle("active", b.dataset.view === name));
  document.querySelectorAll(".view").forEach((s) => s.classList.toggle("hidden", s.id !== `view-${name}`));
  if (name === "dashboard") loadDashboard();
  if (name === "snapshots") loadSnapshots();
  if (name === "settings") loadSettings();
}

async function bootstrap() {
  const session = await api("/api/session");
  if (session.authenticated) {
    hide($("login-view"));
    show($("main-view"));
    setView(state.view === "dashboard" ? "dashboard" : state.view);
    startJobPoll();
  } else {
    show($("login-view"));
    hide($("main-view"));
  }
}

$("login-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  try {
    await api("/api/login", { method: "POST", body: JSON.stringify({ token: $("token").value }) });
    hide($("login-error"));
    bootstrap();
  } catch (err) {
    $("login-error").textContent = err.message;
    show($("login-error"));
  }
});

$("logout").addEventListener("click", async () => {
  await api("/api/logout", { method: "POST" });
  stopJobPoll();
  bootstrap();
});

document.querySelectorAll(".nav-item").forEach((b) => b.addEventListener("click", () => setView(b.dataset.view)));

$("run-backup").addEventListener("click", async () => {
  try {
    await api("/api/backup/run", { method: "POST" });
    toast("备份已开始");
    setView("dashboard");
    await refreshJob();
  } catch (err) { toast(err.message, true); }
});

async function loadDashboard() {
  const data = await api("/api/status");
  const mode = LABELS.backupMode[data.backup.mode] || data.backup.mode;
  const upload = data.upload.enabled ? "已启用" : "未启用";
  const repo = data.repository.exists ? "正常" : "未初始化";

  $("status-grid").innerHTML = `
    <div class="stat-card"><div class="stat-label">服务器</div><div class="stat-value">${esc(data.server.name)}</div></div>
    <div class="stat-card"><div class="stat-label">备份模式</div><div class="stat-value">${mode}</div></div>
    <div class="stat-card"><div class="stat-label">快照</div><div class="stat-value">${data.repository.snapshots}</div></div>
    <div class="stat-card"><div class="stat-label">仓库</div><div class="stat-value ${data.repository.exists ? "ok" : "warn"}">${repo}</div></div>
    <div class="stat-card"><div class="stat-label">远程上传</div><div class="stat-value">${upload}</div></div>
    <div class="stat-card"><div class="stat-label">定时任务</div><div class="stat-value">${data.schedule.enabled ? data.schedule.cron : "未启用"}</div></div>
  `;
  renderJob(data.job);
}

function renderJob(job) {
  const badge = $("job-badge");
  const panel = $("job-panel");
  if (!job || job.status === "idle") {
    badge.textContent = "空闲";
    badge.className = "badge";
    panel.innerHTML = "<span>暂无运行中的任务</span>";
    return;
  }
  const op = LABELS.jobOp[job.operation] || job.operation;
  const st = LABELS.jobStatus[job.status] || job.status;
  badge.textContent = st;
  badge.className = `badge ${job.status === "running" ? "running" : job.status === "failed" ? "failed" : "done"}`;
  panel.innerHTML = `
    <strong>${esc(job.message || "处理中…")}</strong>
    <span>${op} · 开始 ${formatTime(job.started_at)}${job.completed_at ? ` · 完成 ${formatTime(job.completed_at)}` : ""}</span>
    ${job.snapshot_id ? `<span>快照 ${esc(job.snapshot_id)}</span>` : ""}
  `;
}

async function refreshJob() { renderJob(await api("/api/jobs/current")); }

function startJobPoll() {
  stopJobPoll();
  state.jobPoll = setInterval(() => refreshJob().catch(() => {}), 2000);
}

function stopJobPoll() {
  if (state.jobPoll) clearInterval(state.jobPoll);
  state.jobPoll = null;
}

async function loadSnapshots() {
  const data = await api("/api/snapshots");
  state.snapshots = data.snapshots || [];
  const list = $("snapshot-list");
  if (!state.snapshots.length) {
    list.innerHTML = `<div class="empty-state">暂无快照，点击顶栏「立即备份」创建第一个。</div>`;
    return;
  }
  list.innerHTML = state.snapshots.map((s) => {
    const mode = LABELS.backupMode[s.mode] || s.mode;
    const status = LABELS.snapStatus[s.status] || s.status;
    return `
    <div class="list-item">
      <div class="list-item-main">
        <div class="list-item-id">${esc(s.id)}</div>
        <div class="list-item-meta">${formatTime(s.started_at)} · ${mode} · ${status} · ${formatBytes(s.total_bytes)} · ${s.file_count || 0} 文件</div>
      </div>
      <button class="btn btn-ghost btn-sm restore-btn" data-id="${esc(s.id)}">恢复</button>
    </div>`;
  }).join("");
  list.querySelectorAll(".restore-btn").forEach((b) => b.addEventListener("click", () => openRestore(b.dataset.id)));
}

$("refresh-snapshots").addEventListener("click", loadSnapshots);

function openRestore(id) {
  state.restoreId = id;
  $("restore-title").textContent = id;
  $("restore-dialog").showModal();
}

$("restore-form").addEventListener("submit", async (e) => {
  if (e.submitter?.id !== "restore-submit") return;
  e.preventDefault();
  try {
    await api("/api/restore", {
      method: "POST",
      body: JSON.stringify({
        snapshot_id: state.restoreId,
        source: $("restore-source").value,
        force_online: $("restore-force-online").checked,
      }),
    });
    $("restore-dialog").close();
    toast("恢复已开始");
    setView("dashboard");
    await refreshJob();
  } catch (err) { toast(err.message, true); }
});

$("repo-init").addEventListener("click", async () => {
  try {
    const res = await api("/api/repo/init", { method: "POST" });
    const el = $("repo-message");
    el.textContent = res.message;
    show(el);
    toast("仓库已初始化");
    loadDashboard();
  } catch (err) { toast(err.message, true); }
});

$("repo-verify").addEventListener("click", async () => {
  try {
    await api("/api/repo/verify", { method: "POST", body: "{}" });
    toast("校验已开始");
    setView("dashboard");
    await refreshJob();
  } catch (err) { toast(err.message, true); }
});

$("prune-preview").addEventListener("click", async () => {
  try {
    const data = await api("/api/prune/preview");
    const el = $("prune-result");
    el.innerHTML = `
      <div class="prune-block">保留 ${(data.keep || []).length} 个\n${JSON.stringify(data.keep, null, 2)}</div>
      <div class="prune-block delete">删除 ${(data.delete || []).length} 个\n${JSON.stringify(data.delete, null, 2)}</div>
    `;
    show(el);
  } catch (err) { toast(err.message, true); }
});

$("prune-apply").addEventListener("click", async () => {
  if (!confirm("确定执行清理？过期快照将被永久删除。")) return;
  try {
    await api("/api/prune/apply", { method: "POST" });
    toast("清理已开始");
    await refreshJob();
  } catch (err) { toast(err.message, true); }
});

function esc(s) {
  return String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
}

function parseParams(text) {
  const p = {};
  text.split("\n").map((l) => l.trim()).filter(Boolean).forEach((line) => {
    const i = line.indexOf("=");
    if (i > 0) p[line.slice(0, i).trim()] = line.slice(i + 1).trim();
  });
  return p;
}

function paramsToText(obj) {
  return Object.entries(obj || {}).map(([k, v]) => `${k}=${v}`).join("\n");
}

async function loadProviders() {
  if (state.providers.length) return;
  try {
    const data = await api("/api/rclone/providers");
    state.providers = data.providers || [];
    $("remote-type").innerHTML = state.providers.map((p) =>
      `<option value="${esc(p.name)}">${esc(p.name)}</option>`
    ).join("");
  } catch (_) {
    $("remote-type").innerHTML = `<option value="webdav">webdav</option>`;
  }
}

async function loadRemotes() {
  await loadProviders();
  const data = await api("/api/rclone/remotes");
  const list = $("remote-list");
  const names = data.remotes || [];
  if (!names.length) {
    list.innerHTML = `<div class="empty-state" style="padding:24px">尚未添加远程连接</div>`;
    return;
  }
  const items = await Promise.all(names.map(async (name) => {
    const remote = await api(`/api/rclone/remotes/${encodeURIComponent(name)}`);
    return { name, remote };
  }));
  list.innerHTML = items.map(({ name, remote }) => `
    <div class="remote-item">
      <div>
        <div class="remote-name">${esc(name)}</div>
        <div class="remote-type">${esc(remote.type || "—")}</div>
      </div>
      <div class="remote-actions">
        <button class="btn btn-ghost btn-sm edit-remote" data-name="${esc(name)}">编辑</button>
        <button class="btn btn-ghost btn-sm btn-danger delete-remote" data-name="${esc(name)}">删除</button>
      </div>
    </div>
  `).join("");
  list.querySelectorAll(".delete-remote").forEach((b) => b.addEventListener("click", async () => {
    if (!confirm(`删除远程「${b.dataset.name}」？`)) return;
    await api(`/api/rclone/remotes/${encodeURIComponent(b.dataset.name)}`, { method: "DELETE" });
    toast("已删除");
    loadRemotes();
  }));
  list.querySelectorAll(".edit-remote").forEach((b) => b.addEventListener("click", async () => {
    const remote = await api(`/api/rclone/remotes/${encodeURIComponent(b.dataset.name)}`);
    state.editRemoteName = b.dataset.name;
    $("remote-edit-title").textContent = b.dataset.name;
    $("remote-edit-params").value = paramsToText(remote);
    $("remote-edit-dialog").showModal();
  }));
}

$("refresh-remotes").addEventListener("click", loadRemotes);

$("rclone-create-form").addEventListener("submit", async (e) => {
  e.preventDefault();
  try {
    await api("/api/rclone/remotes", {
      method: "POST",
      body: JSON.stringify({
        name: $("remote-name").value.trim(),
        type: $("remote-type").value,
        parameters: parseParams($("remote-params").value),
      }),
    });
    e.target.reset();
    toast("远程已添加");
    loadRemotes();
  } catch (err) { toast(err.message, true); }
});

$("remote-edit-form").addEventListener("submit", async (e) => {
  if (e.submitter?.id !== "remote-edit-submit") return;
  e.preventDefault();
  try {
    await api(`/api/rclone/remotes/${encodeURIComponent(state.editRemoteName)}`, {
      method: "PATCH",
      body: JSON.stringify({ parameters: parseParams($("remote-edit-params").value) }),
    });
    $("remote-edit-dialog").close();
    toast("已更新");
    loadRemotes();
  } catch (err) { toast(err.message, true); }
});

function renderSettingsNav() {
  $("settings-nav").innerHTML = SETTINGS_SECTIONS.map((s) =>
    `<button type="button" class="settings-nav-item ${s.id === state.settingsTab ? "active" : ""}" data-tab="${s.id}">${s.title}</button>`
  ).join("");
  $("settings-nav").querySelectorAll(".settings-nav-item").forEach((b) => {
    b.addEventListener("click", () => {
      state.settingsTab = b.dataset.tab;
      renderSettingsNav();
      renderSettingsContent();
    });
  });
}

function renderSettingsContent() {
  const section = SETTINGS_SECTIONS.find((s) => s.id === state.settingsTab);
  if (!section) return;

  $("settings-section-head").innerHTML = `
    <h2>${section.title}</h2>
    ${section.desc ? `<p>${section.desc}</p>` : ""}
  `;

  const isRclone = section.id === "rclone";
  $("config-form").classList.toggle("hidden", isRclone);
  $("rclone-panel").classList.toggle("hidden", !isRclone);

  if (isRclone) {
    loadRemotes();
    return;
  }

  if (!state.config || !section.fields) return;
  $("config-form").innerHTML = section.fields
    .filter((f) => !f.showIf || f.showIf(state.config))
    .map(renderField)
    .join("");

  $("config-form").querySelectorAll("[data-path]").forEach((el) => {
    el.addEventListener("change", onFieldChange);
    el.addEventListener("input", onFieldChange);
  });
}

function renderField(field) {
  const val = getPath(state.config, field.path);
  const id = `f-${field.path.replace(/\./g, "-")}`;

  if (field.type === "toggle") {
    return `
      <label class="switch-row" for="${id}">
        <div class="switch-info">
          <span class="switch-label">${field.label}</span>
          ${field.hint ? `<span class="switch-hint">${field.hint}</span>` : ""}
        </div>
        <input type="checkbox" class="switch-input" id="${id}" data-path="${field.path}" ${val ? "checked" : ""}>
        <span class="switch-track"></span>
      </label>`;
  }

  let control = "";
  if (field.type === "select") {
    control = `<select id="${id}" data-path="${field.path}">${field.options.map((o) =>
      `<option value="${o}" ${val === o ? "selected" : ""}>${LABELS.select[o] || o}</option>`
    ).join("")}</select>`;
  } else if (field.type === "lines") {
    control = `<textarea id="${id}" data-path="${field.path}" data-type="lines" rows="3">${Array.isArray(val) ? val.join("\n") : ""}</textarea>`;
  } else if (field.type === "password") {
    control = `<input type="password" id="${id}" data-path="${field.path}" data-type="password" placeholder="${field.placeholder || ""}" value="${val === REDACTED ? "" : esc(val || "")}">`;
  } else if (field.type === "number") {
    control = `<input type="number" id="${id}" data-path="${field.path}" data-type="number" value="${val ?? ""}">`;
  } else {
    control = `<input type="text" id="${id}" data-path="${field.path}" value="${esc(val ?? "")}">`;
  }

  return `
    <div class="field-row">
      <div class="field-row-label">
        ${field.label}
        ${field.hint ? `<span class="field-row-hint">${field.hint}</span>` : ""}
      </div>
      <div class="field-row-control">${control}</div>
    </div>`;
}

function onFieldChange(e) {
  const el = e.target;
  const path = el.dataset.path;
  let value;
  if (el.type === "checkbox") value = el.checked;
  else if (el.dataset.type === "lines") value = el.value.split("\n").map((l) => l.trim()).filter(Boolean);
  else if (el.dataset.type === "number") value = el.value === "" ? 0 : Number(el.value);
  else if (el.dataset.type === "password") value = el.value === "" ? REDACTED : el.value;
  else value = el.value;
  setPath(state.config, path, value);
  if (path === "server.control.type") renderSettingsContent();
}

async function loadSettings() {
  const data = await api("/api/config");
  state.config = data.config;
  state.configPath = data.path || "";
  $("config-path").textContent = state.configPath
    ? `配置文件 ${state.configPath}`
    : "未指定配置文件，保存不可用";
  renderSettingsNav();
  renderSettingsContent();
}

function collectConfig() {
  const cfg = JSON.parse(JSON.stringify(state.config));
  document.querySelectorAll("#config-form [data-path]").forEach((el) => {
    const path = el.dataset.path;
    let value;
    if (el.type === "checkbox") value = el.checked;
    else if (el.dataset.type === "lines") value = el.value.split("\n").map((l) => l.trim()).filter(Boolean);
    else if (el.dataset.type === "number") value = el.value === "" ? 0 : Number(el.value);
    else if (el.dataset.type === "password") value = el.value === "" ? REDACTED : el.value;
    else value = el.value;
    setPath(cfg, path, value);
  });
  return cfg;
}

$("config-save").addEventListener("click", async () => {
  try {
    const res = await api("/api/config", { method: "PUT", body: JSON.stringify(collectConfig()) });
    state.config = res.config;
    toast(res.message || "已保存");
    renderSettingsContent();
  } catch (err) { toast(err.message, true); }
});

$("config-validate").addEventListener("click", async () => {
  try {
    const res = await api("/api/config/validate", { method: "POST", body: JSON.stringify(collectConfig()) });
    toast(res.message || "校验通过");
  } catch (err) { toast(err.message, true); }
});

bootstrap().catch(() => {
  show($("login-view"));
  hide($("main-view"));
});
