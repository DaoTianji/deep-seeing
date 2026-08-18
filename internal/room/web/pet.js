(() => {
  const orb = document.getElementById("pet-orb");
  const panel = document.getElementById("pet-panel");
  const closeBtn = document.getElementById("pet-close");
  const openRoomBtn = document.getElementById("pet-open-room");
  const clearBtn = document.getElementById("pet-clear");
  const resizeGrip = document.getElementById("pet-resize-grip");
  const form = document.getElementById("pet-form");
  const input = document.getElementById("pet-input");
  const sendBtn = document.getElementById("pet-send");
  const log = document.getElementById("pet-log");
  const empty = document.getElementById("pet-empty");
  const status = document.getElementById("pet-status");

  const isDesktop = Boolean(window.__TAURI_INTERNALS__ || window.__TAURI__);
  const state = { open: false, busy: false };

  if (isDesktop) {
    document.documentElement.classList.add("pet-desktop");
    document.body.classList.add("pet-desktop");
  }

  async function invoke(command, args = {}) {
    if (!isDesktop) return undefined;
    if (window.__TAURI__?.core?.invoke) {
      return window.__TAURI__.core.invoke(command, args);
    }
    return window.__TAURI_INTERNALS__?.invoke(command, args);
  }

  async function invokePetMode(open) {
    const command = open ? "set_pet_mode_panel" : "set_pet_mode_orb";
    await invoke(command);
  }

  async function setOpen(open) {
    state.open = open;
    orb.setAttribute("aria-expanded", open ? "true" : "false");
    orb.setAttribute("aria-label", open ? "收起终端" : "打开终端");
    try {
      if (open) {
        await invokePetMode(true);
        panel.hidden = false;
      } else {
        panel.hidden = true;
        await invokePetMode(false);
      }
    } catch {
      panel.hidden = !open;
      setStatus("窗口调整失败", "error");
    }
    if (open && !state.busy) {
      input.focus();
    }
  }

  function setStatus(text, kind = "ready") {
    status.textContent = text || "";
    document.body.classList.toggle("is-busy", kind === "busy");
    document.body.classList.toggle("has-error", kind === "error");
  }

  function appendLine(className, text) {
    empty.hidden = true;
    const line = document.createElement("div");
    line.className = className;
    line.textContent = text;
    log.appendChild(line);
    log.scrollTop = log.scrollHeight;
    return line;
  }

  function setBusy(busy) {
    state.busy = busy;
    input.disabled = busy;
    sendBtn.disabled = busy;
  }

  function resizeComposer() {
    input.style.height = "auto";
    input.style.height = `${Math.min(input.scrollHeight, 112)}px`;
  }

  function clearView() {
    log.querySelectorAll(":scope > :not(.pet-empty)").forEach((child) => child.remove());
    empty.hidden = false;
    setStatus("已清空当前视图");
    input.focus();
  }

  async function readNDJSON(stream, onEvent) {
    const reader = stream.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    while (true) {
      const { value, done } = await reader.read();
      buffer += decoder.decode(value || new Uint8Array(), { stream: !done });
      const lines = buffer.split("\n");
      buffer = lines.pop() || "";
      for (const line of lines) {
        if (!line.trim()) continue;
        let eventData;
        try {
          eventData = JSON.parse(line);
        } catch (error) {
          throw new Error(`流式解析失败：${error.message}`);
        }
        onEvent(eventData);
      }
      if (done) break;
    }
    if (buffer.trim()) {
      try {
        onEvent(JSON.parse(buffer));
      } catch (error) {
        throw new Error(`流式解析失败：${error.message}`);
      }
    }
  }

  function handleStreamEvent(eventData, reply, turn) {
    if (eventData.type === "delta") {
      turn.raw += String(eventData.data || "");
      reply.textContent = turn.raw;
      setStatus("正在回复", "busy");
      log.scrollTop = log.scrollHeight;
      return;
    }
    if (eventData.type === "tool") {
      setStatus("正在处理", "busy");
      return;
    }
    if (eventData.type === "error") {
      turn.streamError = eventData.data?.message || "对话失败";
      return;
    }
    if (eventData.type === "done" && eventData.data?.answer) {
      turn.raw = String(eventData.data.answer);
      reply.textContent = turn.raw;
    }
  }

  async function requireChatResponse(response) {
    if (response.ok) {
      if (!response.body) throw new Error("浏览器不支持流式回应");
      return response.body;
    }
    let detail = `HTTP ${response.status}`;
    try {
      const payload = await response.json();
      detail = payload?.error || detail;
    } catch {
      // The HTTP status remains the actionable fallback.
    }
    throw new Error(detail);
  }

  function renderTurnFailure(reply, turn, error) {
    const detail = String(error?.message || "对话失败");
    reply.textContent = turn.raw.trim()
      ? `${turn.raw}\n\n—\n回应中断：${detail}`
      : detail;
    reply.classList.add("pet-line-error");
    setStatus("回应中断", "error");
  }

  async function sendMessage(message) {
    if (state.busy) return;
    message = String(message || "").trim();
    if (!message) return;

    appendLine("pet-line-user", `> ${message}`);
    const reply = appendLine("pet-line-assistant", "");
    const turn = { raw: "", streamError: "" };

    setBusy(true);
    setStatus("正在理解", "busy");
    input.value = "";
    resizeComposer();

    try {
      const response = await fetch("/api/chat", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ message }),
      });
      const stream = await requireChatResponse(response);
      await readNDJSON(stream, (eventData) => handleStreamEvent(eventData, reply, turn));

      if (turn.streamError) throw new Error(turn.streamError || "对话失败");
      if (!turn.raw.trim()) {
        reply.textContent = "(这轮没有生成可见文字)";
        reply.className = "pet-line-meta";
      }
    } catch (error) {
      renderTurnFailure(reply, turn, error);
    } finally {
      setBusy(false);
      if (!document.body.classList.contains("has-error")) {
        setStatus("已连接");
      }
      if (state.open) input.focus();
    }
  }

  async function openFullRoom() {
    try {
      if (isDesktop) {
        await invoke("open_full_room");
      } else {
        window.open("/", "_blank", "noopener,noreferrer");
      }
      setStatus("已打开完整界面");
    } catch {
      setStatus("打开完整界面失败", "error");
    }
  }

  let pointerMoved = false;
  let pointerStart = null;
  orb.addEventListener("pointerdown", (event) => {
    pointerMoved = false;
    pointerStart = { x: event.clientX, y: event.clientY };
  });
  orb.addEventListener("pointermove", (event) => {
    if (!pointerStart) return;
    const dx = event.clientX - pointerStart.x;
    const dy = event.clientY - pointerStart.y;
    if (dx * dx + dy * dy > 16) pointerMoved = true;
  });
  orb.addEventListener("pointerup", () => {
    pointerStart = null;
  });
  orb.addEventListener("click", (event) => {
    if (pointerMoved) {
      event.preventDefault();
      return;
    }
    setOpen(!state.open);
  });
  closeBtn.addEventListener("click", () => setOpen(false));
  openRoomBtn.addEventListener("click", openFullRoom);
  clearBtn.addEventListener("click", clearView);

  resizeGrip.addEventListener("pointerdown", async (event) => {
    event.preventDefault();
    event.stopPropagation();
    try {
      await invoke("plugin:window|start_resize_dragging", {
        label: "main",
        value: "SouthEast",
      });
    } catch {
      setStatus("无法调整窗口大小", "error");
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && state.open) {
      event.preventDefault();
      setOpen(false);
    }
  });

  input.addEventListener("input", resizeComposer);
  input.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && !event.shiftKey && !event.isComposing) {
      event.preventDefault();
      form.requestSubmit();
    }
  });

  form.addEventListener("submit", (event) => {
    event.preventDefault();
    sendMessage(input.value);
  });

  if (isDesktop) {
    invokePetMode(false);
  }
  resizeComposer();
})();
