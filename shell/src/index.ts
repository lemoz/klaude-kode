import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import process from "node:process";
import React from "react";
import { render } from "ink";

import {
  makeApprovePermissionCommand,
  defaultShellConfig,
  diffRuns,
  type EngineSession,
  makeCloseSessionCommand,
  makeDenyPermissionCommand,
  loginAnthropic,
  loginAnthropicOAuth,
  listModels,
  logoutProfile,
  listProfiles,
  loginOpenRouter,
  makeUpdateSessionSettingCommand,
  makeUserInputCommand,
  runBenchmarkEval,
  runReplayEval,
  showRun,
  summarizeRuns,
  startEngineSession,
  validateCandidate,
  type CandidateValidationResult,
  type EvalRun,
  type EvalRunDiff,
  type EvalRunSummary,
  type PermissionEventPayload,
  type ProfileStatus,
  exportReplayPack,
  listFrontier,
  type SessionEvent,
  type SessionStateSnapshot,
  type ShellConfig,
  type FrontierEntry,
} from "./engineClient.js";
import {
  InteractiveShell,
  type InteractiveShellFooter,
  type InteractiveShellHeader,
} from "./app.js";
import { ShellPresentationModel } from "./presentation.js";

async function main(): Promise<void> {
  const config = parseArgs(process.argv.slice(2));
  if (config.prompt !== "") {
    await runPromptMode(config);
    return;
  }
  await runInteractiveShell(config);
}

interface LineWriter {
  writeLine(line: string): void;
}

type ShellSurface =
  | "conversation"
  | "profiles"
  | "status"
  | "models"
  | "validation"
  | "runs"
  | "replay_eval"
  | "benchmark_eval"
  | "frontier"
  | "diff";

interface ShellUIController extends LineWriter {
  decisionVersion(): number;
  currentState(): SessionStateSnapshot | null;
  waitForDecision(afterVersion: number): Promise<void>;
  currentPrompt(): string;
  pendingPermission(): PermissionEventPayload | null;
  takePendingPermission(): PermissionEventPayload | undefined;
  setProfileStatuses(profiles: ProfileStatus[]): void;
  setActiveSurface(surface: ShellSurface): void;
}

async function runPromptMode(config: ShellConfig): Promise<void> {
  const renderer = createRenderer(config.rawEvents);
  const session = await startEngineSession(config, (event) => {
    renderer.handleEvent(event);
  });

  try {
    await session.sendCommand(makeUserInputCommand(config.prompt));
    await session.sendCommand(makeCloseSessionCommand("shell_prompt_complete"));
    await session.closeInput();
    await session.done;
  } finally {
    await session.closeInput().catch(() => undefined);
  }
}

async function runInteractiveShell(initialConfig: ShellConfig): Promise<void> {
  const config = { ...initialConfig };
  const model = new ShellPresentationModel(config.rawEvents);
  let profileStatuses: ProfileStatus[] = [];
  let session: EngineSession | null = null;
  let busy = true;
  let closed = false;
  let inputValue = "";
  let lines: string[] = [];
  let activeSurface: ShellSurface = "conversation";

  const appendLine = (line: string) => {
    lines = [...lines, line];
  };

  const ui: ShellUIController = {
    decisionVersion(): number {
      return model.decisionVersion();
    },
    currentState(): SessionStateSnapshot | null {
      return model.currentState();
    },
    waitForDecision(afterVersion: number): Promise<void> {
      return model.waitForDecision(afterVersion);
    },
    currentPrompt(): string {
      return model.currentPrompt();
    },
    pendingPermission(): PermissionEventPayload | null {
      return model.pendingPermission();
    },
    takePendingPermission(): PermissionEventPayload | undefined {
      return model.takePendingPermission();
    },
    setProfileStatuses(profiles: ProfileStatus[]): void {
      profileStatuses = profiles;
      rerender();
    },
    setActiveSurface(surface: ShellSurface): void {
      activeSurface = surface;
      rerender();
    },
    writeLine(line: string): void {
      appendLine(line);
      rerender();
    },
  };

  let rerender = () => undefined;
  const renderHeader = (): InteractiveShellHeader =>
    buildInteractiveHeader(config, model.currentState(), profileStatuses);
  const renderFooter = (): InteractiveShellFooter =>
    buildInteractiveFooter(model.currentState(), ui.pendingPermission(), closed, activeSurface);
  const renderShell = () =>
    React.createElement(InteractiveShell, {
      header: renderHeader(),
      footer: renderFooter(),
      turns: model.transcript(),
      lines,
      pendingPermission: ui.pendingPermission(),
      promptLabel: ui.currentPrompt(),
      inputValue,
      busy,
      closed,
    });
  const ink = render(renderShell());

  rerender = () => {
    ink.rerender(renderShell());
  };

  const stdin = process.stdin;
  const previousRawMode = "isRaw" in stdin ? (stdin as NodeJS.ReadStream & { isRaw?: boolean }).isRaw : undefined;
  const canSetRawMode = typeof (stdin as NodeJS.ReadStream).setRawMode === "function";

  const onInputData = (chunk: Buffer | string) => {
    const text = chunk.toString();
    void consumeInputChunk(text);
  };

  try {
    if (canSetRawMode && stdin.isTTY) {
      (stdin as NodeJS.ReadStream).setRawMode(true);
    }
    stdin.resume();
    stdin.setEncoding("utf8");
    stdin.on("data", onInputData);

    session = await startEngineSession(config, (event) => {
      const previousState = model.currentState();
      model.applyEvent(event);
      const eventLines = renderInteractiveEvent(
        config.rawEvents,
        event,
        previousState,
        model.currentState(),
      );
      if (eventLines.length > 0) {
        lines = [...lines, ...eventLines];
      }
      if (event.kind === "session_closed") {
        closed = true;
        busy = false;
      }
      rerender();
    });
    try {
      profileStatuses = await listProfiles(config);
    } catch {
      profileStatuses = [];
    }
    busy = false;
    rerender();

    void session.done.then(() => {
      closed = true;
      busy = false;
      rerender();
    }).catch((error: unknown) => {
      const message = error instanceof Error ? error.message : String(error);
      appendLine(`error: ${message}`);
      closed = true;
      busy = false;
      rerender();
    });

    await ink.waitUntilExit();
  } finally {
    stdin.off("data", onInputData);
    if (canSetRawMode && stdin.isTTY) {
      (stdin as NodeJS.ReadStream).setRawMode(previousRawMode ?? false);
    }
    await session?.closeInput().catch(() => undefined);
    ink.unmount();
  }

  async function closeShell(reason: string): Promise<void> {
    if (closed) {
      return;
    }
    busy = true;
    rerender();
    if (!session) {
      closed = true;
      busy = false;
      rerender();
      return;
    }
    await session.sendCommand(makeCloseSessionCommand(reason));
    await session.closeInput();
  }

  async function submitBufferedInput(): Promise<void> {
    const line = inputValue.trimEnd();
    inputValue = "";
    rerender();
    if (line === "") {
      return;
    }
    if (line.trim() === "/exit") {
      await closeShell("shell_exit");
      return;
    }
    if (!session) {
      return;
    }
    busy = true;
    rerender();
    try {
      await handleShellInputLine(session, ui, config, line);
    } catch (error) {
      const message = error instanceof Error ? error.message : String(error);
      appendLine(`error: ${message}`);
    } finally {
      if (!closed) {
        busy = false;
        rerender();
      }
    }
  }

  async function consumeInputChunk(text: string): Promise<void> {
    for (const character of text) {
      if (closed) {
        return;
      }
      if (character === "\u0003") {
        await closeShell("shell_exit");
        return;
      }
      if (busy) {
        continue;
      }
      if (character === "\r" || character === "\n") {
        await submitBufferedInput();
        continue;
      }
      if (character === "\u007f" || character === "\b") {
        inputValue = inputValue.slice(0, -1);
        rerender();
        continue;
      }
      if (character === "\u001b") {
        inputValue = "";
        rerender();
        continue;
      }
      if (character >= " " && character !== "\u007f") {
        inputValue += character;
        rerender();
      }
    }
  }
}

async function handleShellInputLine(
  session: EngineSession,
  ui: ShellUIController,
  config: ShellConfig,
  rawLine: string,
): Promise<void> {
  const line = rawLine.trim();
  if (line === "") {
    return;
  }
  const slashCommand = parseSlashCommand(line);
  if (slashCommand?.kind === "setting") {
    ui.setActiveSurface("conversation");
    const decisionVersion = ui.decisionVersion();
    await session.sendCommand(makeUpdateSessionSettingCommand(slashCommand.key, slashCommand.value));
    await ui.waitForDecision(decisionVersion);
    if (slashCommand.key === "profile_id") {
      config.profileId = slashCommand.value;
      const catalog = await listModels(config, slashCommand.value);
      config.model = catalog.default_model;
      ui.setProfileStatuses(await listProfiles(config));
    } else {
      config.model = slashCommand.value;
    }
    return;
  }
  if (slashCommand?.kind === "profiles") {
    ui.setActiveSurface("profiles");
    const profiles = await listProfiles(config);
    ui.setProfileStatuses(profiles);
    renderProfiles(ui, profiles, config.profileId);
    return;
  }
  if (slashCommand?.kind === "status") {
    ui.setActiveSurface("status");
    renderStatus(ui, config, ui.currentState());
    return;
  }
  if (slashCommand?.kind === "summarize_runs") {
    ui.setActiveSurface("runs");
    const summary = await summarizeRuns(config);
    renderRunSummary(ui, summary);
    return;
  }
  if (slashCommand?.kind === "show_run") {
    ui.setActiveSurface("runs");
    const run = await showRun(config, slashCommand.runID);
    renderEvalRun(ui, run);
    return;
  }
  if (slashCommand?.kind === "list_frontier") {
    ui.setActiveSurface("frontier");
    const entries = await listFrontier(config, slashCommand.limit);
    renderFrontier(ui, entries);
    return;
  }
  if (slashCommand?.kind === "diff_runs") {
    ui.setActiveSurface("diff");
    const diff = await diffRuns(config, slashCommand.leftRunID, slashCommand.rightRunID);
    renderRunDiff(ui, diff);
    return;
  }
  if (slashCommand?.kind === "validate_candidate") {
    ui.setActiveSurface("validation");
    const validation = await validateCandidate(config);
    renderCandidateValidation(ui, validation);
    return;
  }
  if (slashCommand?.kind === "run_benchmark") {
    ui.setActiveSurface("benchmark_eval");
    const benchmarkPath = path.resolve(config.cwd, slashCommand.benchmarkPath);
    const evalRun = await runBenchmarkEval(config, benchmarkPath);
    renderEvalRun(ui, evalRun);
    return;
  }
  if (slashCommand?.kind === "run_replay") {
    ui.setActiveSurface("replay_eval");
    const replayPath = path.resolve(config.cwd, slashCommand.replayPath);
    const evalRun = await runReplayEval(config, replayPath);
    renderReplayEval(ui, evalRun);
    return;
  }
  if (slashCommand?.kind === "export_replay") {
    ui.setActiveSurface("replay_eval");
    const targetPath = path.resolve(config.cwd, slashCommand.outputPath);
    const replayPack = await exportReplayPack(config);
    await mkdir(path.dirname(targetPath), { recursive: true });
    await writeFile(targetPath, `${JSON.stringify(replayPack, null, 2)}\n`, "utf8");
    ui.writeLine(`export: wrote replay pack to ${targetPath}`);
    return;
  }
  if (slashCommand?.kind === "models") {
    ui.setActiveSurface("models");
    const catalog = await listModels(
      config,
      slashCommand.profileId || config.profileId,
    );
    renderModelCatalog(
      ui,
      catalog,
      config.profileId,
      config.model,
    );
    return;
  }
  if (slashCommand?.kind === "logout") {
    ui.setActiveSurface("profiles");
    const profiles = await logoutProfile(config, slashCommand.profileId);
    ui.setProfileStatuses(profiles);
    ui.writeLine(`logout: cleared stored auth for ${slashCommand.profileId}`);
    renderProfiles(ui, profiles, config.profileId);
    return;
  }
  if (slashCommand?.kind === "login_openrouter") {
    ui.setActiveSurface("profiles");
    const profiles = await loginOpenRouter(config, {
      credential: slashCommand.credential,
      defaultModel: slashCommand.defaultModel,
      apiBase: slashCommand.apiBase,
    });
    ui.setProfileStatuses(profiles);
    ui.writeLine("login: saved openrouter-main and set it as default");
    renderProfiles(ui, profiles, "openrouter-main");
    const decisionVersion = ui.decisionVersion();
    await session.sendCommand(makeUpdateSessionSettingCommand("profile_id", "openrouter-main"));
    await ui.waitForDecision(decisionVersion);
    config.profileId = "openrouter-main";
    config.model = slashCommand.defaultModel || (await listModels(config, "openrouter-main")).default_model;
    return;
  }
  if (slashCommand?.kind === "login_anthropic") {
    ui.setActiveSurface("profiles");
    const profiles = await loginAnthropic(config, {
      credential: slashCommand.credential,
      defaultModel: slashCommand.defaultModel,
      apiBase: slashCommand.apiBase,
    });
    ui.setProfileStatuses(profiles);
    ui.writeLine("login: saved anthropic-api and set it as default");
    renderProfiles(ui, profiles, "anthropic-api");
    const decisionVersion = ui.decisionVersion();
    await session.sendCommand(makeUpdateSessionSettingCommand("profile_id", "anthropic-api"));
    await ui.waitForDecision(decisionVersion);
    config.profileId = "anthropic-api";
    config.model = slashCommand.defaultModel || (await listModels(config, "anthropic-api")).default_model;
    return;
  }
  if (slashCommand?.kind === "login_anthropic_oauth") {
    ui.setActiveSurface("profiles");
    ui.writeLine("login: opening browser for Anthropic OAuth");
    const profiles = await loginAnthropicOAuth(config, {
      defaultModel: slashCommand.defaultModel,
      apiBase: slashCommand.apiBase,
      accountScope: slashCommand.accountScope,
    });
    ui.setProfileStatuses(profiles);
    ui.writeLine("login: saved anthropic-main and set it as default");
    renderProfiles(ui, profiles, "anthropic-main");
    const decisionVersion = ui.decisionVersion();
    await session.sendCommand(makeUpdateSessionSettingCommand("profile_id", "anthropic-main"));
    await ui.waitForDecision(decisionVersion);
    config.profileId = "anthropic-main";
    config.model = slashCommand.defaultModel || (await listModels(config, "anthropic-main")).default_model;
    return;
  }

  const pendingPermission = ui.takePendingPermission();
  if (pendingPermission) {
    ui.setActiveSurface("conversation");
    const decisionVersion = ui.decisionVersion();
    if (looksApproved(line)) {
      await session.sendCommand(
        makeApprovePermissionCommand(pendingPermission.request_id),
      );
    } else {
      await session.sendCommand(
        makeDenyPermissionCommand(pendingPermission.request_id),
      );
    }
    await ui.waitForDecision(decisionVersion);
    return;
  }

  const decisionVersion = ui.decisionVersion();
  ui.setActiveSurface("conversation");
  await session.sendCommand(makeUserInputCommand(line, { askForPermission: true }));
  await ui.waitForDecision(decisionVersion);
}

function parseArgs(args: string[]) {
  const config = defaultShellConfig();

  for (let index = 0; index < args.length; index += 1) {
    const arg = args[index];

    if (arg === "--help" || arg === "-h") {
      printHelp();
      process.exit(0);
    }
    if (arg === "--raw-events") {
      config.rawEvents = true;
      continue;
    }

    const nextValue = args[index + 1];
    const takeValue = (flag: string): string => {
      if (arg === flag) {
        if (nextValue === undefined) {
          throw new Error(`${flag} requires a value`);
        }
        index += 1;
        return nextValue;
      }
      if (arg.startsWith(`${flag}=`)) {
        return arg.slice(flag.length + 1);
      }
      return "";
    };

    const prompt = takeValue("--prompt");
    if (prompt !== "") {
      config.prompt = prompt;
      continue;
    }

    const sessionId = takeValue("--session-id");
    if (sessionId !== "") {
      config.sessionId = sessionId;
      continue;
    }

    const resumeSessionId = takeValue("--resume-session");
    if (resumeSessionId !== "") {
      config.resumeSessionId = resumeSessionId;
      continue;
    }

    const cwd = takeValue("--cwd");
    if (cwd !== "") {
      config.cwd = cwd;
      continue;
    }

    const profileId = takeValue("--profile-id");
    if (profileId !== "") {
      config.profileId = profileId;
      continue;
    }

    const model = takeValue("--model");
    if (model !== "") {
      config.model = model;
      continue;
    }

    const stateRoot = takeValue("--state-root");
    if (stateRoot !== "") {
      config.stateRoot = stateRoot;
      continue;
    }

    throw new Error(`unsupported argument: ${arg}`);
  }

  if (config.resumeSessionId !== "" && config.prompt !== "") {
    throw new Error("--prompt cannot be used with --resume-session");
  }

  return config;
}

function createRenderer(rawEvents: boolean) {
  let headerPrinted = false;
  const model = new ShellPresentationModel(rawEvents);
  const streamedTurns = new Set<string>();

  const print = (line: string) => {
    console.log(line);
  };

  return {
    handleEvent(event: SessionEvent): void {
      const previousState = model.currentState();
      model.applyEvent(event);
      if (rawEvents) {
        print(JSON.stringify(event));
        return;
      }

      const state = model.currentState();
      if (!headerPrinted && state) {
        print(`session: ${event.session_id}`);
        print(`mode: ${state.mode}`);
        print(`model: ${state.model}`);
        headerPrinted = true;
      } else if (
        event.kind === "session_state" &&
        state &&
        previousState &&
        state.status === "active" &&
        (state.model !== previousState.model ||
          state.profile_id !== previousState.profile_id)
      ) {
        print(`session: model=${state.model} profile=${state.profile_id}`);
      }

      if (event.kind === "assistant_delta") {
        if (event.payload.turn_id !== "") {
          streamedTurns.add(event.payload.turn_id);
        }
        const rendered = renderEvent(event);
        if (rendered !== "") {
          print(rendered);
        }
        return;
      }

      if (
        event.kind === "assistant_message" &&
        event.payload.turn_id !== "" &&
        streamedTurns.has(event.payload.turn_id)
      ) {
        streamedTurns.delete(event.payload.turn_id);
      } else {
        const rendered = renderEvent(event);
        if (rendered !== "") {
          print(rendered);
        }
      }

    },
    decisionVersion(): number {
      return model.decisionVersion();
    },
    currentState(): SessionStateSnapshot | null {
      return model.currentState();
    },
    waitForDecision(afterVersion: number): Promise<void> {
      return model.waitForDecision(afterVersion);
    },
    currentPrompt(): string {
      return model.currentPrompt();
    },
    pendingPermission(): PermissionEventPayload | null {
      return model.pendingPermission();
    },
    takePendingPermission(): PermissionEventPayload | undefined {
      return model.takePendingPermission();
    },
    showPrompt(prompt: string): void {
      if (!rawEvents && process.stdout.isTTY) {
        process.stdout.write(prompt);
      }
    },
    writeLine(line: string): void {
      print(line);
    },
  };
}

function renderInteractiveEvent(
  rawEvents: boolean,
  event: SessionEvent,
  _previousState: SessionStateSnapshot | null,
  _currentState: SessionStateSnapshot | null,
): string[] {
  if (rawEvents) {
    return [JSON.stringify(event)];
  }

  const lines: string[] = [];
  if (
    event.kind === "user_message_accepted" ||
    event.kind === "assistant_delta" ||
    event.kind === "assistant_message" ||
    event.kind === "tool_call_requested" ||
    event.kind === "tool_call_progress" ||
    event.kind === "tool_call_completed" ||
    event.kind === "permission_requested"
  ) {
    return lines;
  }
  const rendered = renderEvent(event);
  if (rendered !== "") {
    lines.push(rendered);
  }

  return lines;
}

function buildInteractiveHeader(
  config: ShellConfig,
  state: SessionStateSnapshot | null,
  profiles: ProfileStatus[],
): InteractiveShellHeader {
  const sessionId = config.resumeSessionId || config.sessionId;
  const mode = state?.mode ?? "interactive";
  const profileId = state?.profile_id || config.profileId || "default";
  const model = state?.model || config.model || "default";
  const activeProfile = profiles.find((entry) => entry.profile.id === profileId);

  return {
    sessionId,
    mode,
    profileId,
    provider: activeProfile?.profile.provider ?? "unknown",
    authState: activeProfile?.auth.state ?? "unknown",
    model,
  };
}

function buildInteractiveFooter(
  state: SessionStateSnapshot | null,
  pendingPermission: PermissionEventPayload | null,
  closed: boolean,
  surface: ShellSurface,
): InteractiveShellFooter {
  return {
    surface,
    sessionStatus: closed ? "closed" : state?.status ?? "starting",
    terminalOutcome: state?.terminal_outcome || "none",
    pendingPermission: pendingPermission ? "awaiting_decision" : "none",
    modeCue: surface.includes("eval") || surface === "runs" || surface === "frontier" || surface === "diff"
      ? "artifact_view"
      : "conversation_view",
  };
}

function renderEvent(event: SessionEvent): string {
  switch (event.kind) {
    case "user_message_accepted":
      return event.payload.message?.content
        ? `user: ${event.payload.message.content}`
        : "";
    case "assistant_delta":
      return event.payload.message?.content
        ? `assistant(stream): ${event.payload.message.content}`
        : "";
    case "assistant_message":
      return event.payload.message?.content
        ? `assistant: ${event.payload.message.content}`
        : "";
    case "tool_call_requested":
      return event.payload.tool ? `tool:${event.payload.tool.name} requested` : "";
    case "tool_call_progress":
      return event.payload.tool?.progress_message
        ? `tool:${event.payload.tool.name} ${event.payload.tool.progress_message}`
        : "";
    case "tool_call_completed":
      if (!event.payload.tool) {
        return "";
      }
      return event.payload.tool.output
        ? `tool:${event.payload.tool.name} ${event.payload.tool.result_summary} output=${event.payload.tool.output}`
        : `tool:${event.payload.tool.name} ${event.payload.tool.result_summary}`;
    case "permission_requested":
      return event.payload.permission?.prompt
        ? `permission: ${event.payload.permission.prompt}`
        : "";
    case "permission_resolved":
      return event.payload.permission?.resolution
        ? `permission: ${event.payload.permission.resolution}`
        : "";
    case "warning":
      return event.payload.warning ? `warning: ${event.payload.warning}` : "";
    case "failure":
      return event.payload.failure
        ? `failure:${event.payload.failure.category}/${event.payload.failure.code} retryable=${event.payload.failure.retryable} ${event.payload.failure.message}`
        : "";
    case "session_closed":
      return `status: closed (${event.payload.reason || "no_reason"})`;
    case "session_state":
      return renderTerminalOutcome(event.payload.state);
    default:
      return "";
  }
}

function renderTerminalOutcome(state: SessionStateSnapshot | null): string {
  if (!state || state.status !== "closed") {
    return "";
  }
  return `terminal_outcome: ${state.terminal_outcome || "none"}`;
}

function looksApproved(line: string): boolean {
  const normalized = line.trim().toLowerCase();
  return normalized === "y" || normalized === "yes";
}

function parseSlashCommand(
  line: string,
):
  | { kind: "setting"; key: "model" | "profile_id"; value: string }
  | { kind: "profiles" }
  | { kind: "status" }
  | { kind: "summarize_runs" }
  | { kind: "show_run"; runID: string }
  | { kind: "list_frontier"; limit?: number }
  | { kind: "diff_runs"; leftRunID: string; rightRunID: string }
  | { kind: "validate_candidate" }
  | { kind: "run_benchmark"; benchmarkPath: string }
  | { kind: "run_replay"; replayPath: string }
  | { kind: "export_replay"; outputPath: string }
  | { kind: "models"; profileId?: string }
  | { kind: "logout"; profileId: string }
  | { kind: "login_openrouter"; credential: string; defaultModel?: string; apiBase?: string }
  | { kind: "login_anthropic"; credential: string; defaultModel?: string; apiBase?: string }
  | { kind: "login_anthropic_oauth"; defaultModel?: string; apiBase?: string; accountScope?: string }
  | null {
  const trimmed = line.trim();
  if (!trimmed.startsWith("/")) {
    return null;
  }

  if (trimmed === "/profiles") {
    return { kind: "profiles" };
  }
  if (trimmed === "/status") {
    return { kind: "status" };
  }
  if (trimmed === "/summarize-runs") {
    return { kind: "summarize_runs" };
  }
  if (trimmed.startsWith("/show-run ")) {
    const runID = trimmed.slice("/show-run ".length).trim();
    if (runID === "") {
      throw new Error("usage: /show-run <id>");
    }
    return { kind: "show_run", runID };
  }
  if (trimmed === "/list-frontier") {
    return { kind: "list_frontier" };
  }
  if (trimmed.startsWith("/list-frontier ")) {
    const rawLimit = trimmed.slice("/list-frontier ".length).trim();
    if (rawLimit === "") {
      throw new Error("usage: /list-frontier [limit]");
    }
    const limit = Number.parseInt(rawLimit, 10);
    if (!Number.isFinite(limit) || limit <= 0) {
      throw new Error("usage: /list-frontier [limit]");
    }
    return { kind: "list_frontier", limit };
  }
  if (trimmed.startsWith("/diff-runs ")) {
    const parts = trimmed.slice("/diff-runs ".length).trim().split(/\s+/);
    if (parts.length !== 2 || parts[0] === "" || parts[1] === "") {
      throw new Error("usage: /diff-runs <left-run-id> <right-run-id>");
    }
    return { kind: "diff_runs", leftRunID: parts[0], rightRunID: parts[1] };
  }
  if (trimmed === "/validate-candidate") {
    return { kind: "validate_candidate" };
  }
  if (trimmed.startsWith("/run-benchmark ")) {
    const benchmarkPath = trimmed.slice("/run-benchmark ".length).trim();
    if (benchmarkPath === "") {
      throw new Error("usage: /run-benchmark <path>");
    }
    return { kind: "run_benchmark", benchmarkPath };
  }
  if (trimmed.startsWith("/run-replay ")) {
    const replayPath = trimmed.slice("/run-replay ".length).trim();
    if (replayPath === "") {
      throw new Error("usage: /run-replay <path>");
    }
    return { kind: "run_replay", replayPath };
  }
  if (trimmed.startsWith("/export-replay ")) {
    const outputPath = trimmed.slice("/export-replay ".length).trim();
    if (outputPath === "") {
      throw new Error("usage: /export-replay <path>");
    }
    return { kind: "export_replay", outputPath };
  }
  if (trimmed === "/models") {
    return { kind: "models" };
  }
  if (trimmed.startsWith("/models ")) {
    const profileId = trimmed.slice("/models ".length).trim();
    if (profileId === "") {
      throw new Error("usage: /models [profile-id]");
    }
    return { kind: "models", profileId };
  }
  if (trimmed === "/logout" || trimmed === "/logout anthropic") {
    return { kind: "logout", profileId: "anthropic-main" };
  }
  if (trimmed.startsWith("/logout ")) {
    const raw = trimmed.slice("/logout ".length).trim().toLowerCase();
    if (raw === "anthropic" || raw === "anthropic-main") {
      return { kind: "logout", profileId: "anthropic-main" };
    }
    if (raw === "openrouter" || raw === "openrouter-main") {
      return { kind: "logout", profileId: "openrouter-main" };
    }
    throw new Error("usage: /logout [anthropic|openrouter]");
  }
  if (trimmed.startsWith("/model ")) {
    const value = trimmed.slice("/model ".length).trim();
    if (value !== "") {
      return { kind: "setting", key: "model", value };
    }
  }
  if (trimmed.startsWith("/profile ")) {
    const value = trimmed.slice("/profile ".length).trim();
    if (value !== "") {
      return { kind: "setting", key: "profile_id", value };
    }
  }
  if (trimmed.startsWith("/login ")) {
    return parseLoginCommand(trimmed);
  }
  return null;
}

function parseLoginCommand(
  line: string,
):
  | { kind: "login_openrouter"; credential: string; defaultModel?: string; apiBase?: string }
  | { kind: "login_anthropic"; credential: string; defaultModel?: string; apiBase?: string }
  | { kind: "login_anthropic_oauth"; defaultModel?: string; apiBase?: string; accountScope?: string }
  | null {
  const parts = line.split(/\s+/).slice(1);
  if (parts.length < 2) {
    throw new Error("usage: /login anthropic oauth [model=<id>] [account_scope=claude|console] [api_base=<url>] | /login <openrouter|anthropic> <env-var|credential-ref> [model=<id>] [api_base=<url>]");
  }
  const provider = parts[0]?.toLowerCase();
  if (provider !== "openrouter" && provider !== "anthropic") {
    throw new Error(`unsupported login provider: ${parts[0]}`);
  }

  let defaultModel: string | undefined;
  let apiBase: string | undefined;
  let accountScope: string | undefined;
  const credential = parts[1] ?? "";
  const optionTokens = provider === "anthropic" && credential.toLowerCase() === "oauth"
    ? parts.slice(2)
    : parts.slice(2);

  if (provider === "anthropic" && credential.toLowerCase() === "oauth") {
    for (const token of optionTokens) {
      if (token.startsWith("model=")) {
        defaultModel = token.slice("model=".length);
        continue;
      }
      if (token.startsWith("api_base=")) {
        apiBase = token.slice("api_base=".length);
        continue;
      }
      if (token.startsWith("account_scope=")) {
        accountScope = token.slice("account_scope=".length);
        continue;
      }
      throw new Error(`unsupported login option: ${token}`);
    }
    return {
      kind: "login_anthropic_oauth",
      defaultModel,
      apiBase,
      accountScope,
    };
  }

  if (credential.trim() === "") {
    throw new Error("usage: /login <openrouter|anthropic> <env-var|credential-ref> [model=<id>] [api_base=<url>]");
  }

  for (const token of optionTokens) {
    if (token.startsWith("model=")) {
      defaultModel = token.slice("model=".length);
      continue;
    }
    if (token.startsWith("api_base=")) {
      apiBase = token.slice("api_base=".length);
      continue;
    }
    throw new Error(`unsupported login option: ${token}`);
  }

  if (provider === "anthropic") {
    return {
      kind: "login_anthropic",
      credential,
      defaultModel,
      apiBase,
    };
  }

  return {
    kind: "login_openrouter",
    credential,
    defaultModel,
    apiBase,
  };
}

function renderProfiles(
  ui: LineWriter,
  profiles: ProfileStatus[],
  activeProfileID: string,
): void {
  ui.writeLine("profiles:");
  for (const entry of profiles) {
    const currentMarker = entry.profile.id === activeProfileID ? " (current)" : "";
    ui.writeLine(
      `- ${entry.profile.id}${currentMarker} (${entry.profile.provider}/${entry.profile.kind}) default_model=${entry.profile.default_model} valid=${entry.validation.valid} auth=${entry.auth.state}`,
    );
    if (entry.auth.expires_at !== "") {
      ui.writeLine(`  expires_at: ${entry.auth.expires_at}`);
    }
    if (entry.auth.can_refresh) {
      ui.writeLine("  can_refresh: true");
    }
    if (entry.validation.message !== "") {
      ui.writeLine(`  validation: ${entry.validation.message}`);
    }
    if (entry.models.length > 0) {
      ui.writeLine(`  models: ${entry.models.join(", ")}`);
    }
    const capabilities = formatCapabilities(entry.capabilities);
    if (capabilities !== "") {
      ui.writeLine(`  capabilities: ${capabilities}`);
    }
  }
}

function renderStatus(
  ui: LineWriter,
  config: ShellConfig,
  state: SessionStateSnapshot | null,
): void {
  ui.writeLine("status:");
  ui.writeLine(`- session: ${config.resumeSessionId || config.sessionId}`);
  if (!state) {
    ui.writeLine("  state: unavailable");
    return;
  }
  ui.writeLine(`  mode: ${state.mode}`);
  ui.writeLine(`  status: ${state.status}`);
  ui.writeLine(`  profile: ${state.profile_id}`);
  ui.writeLine(`  model: ${state.model}`);
  ui.writeLine(`  turns: ${state.turn_count}`);
  ui.writeLine(`  events: ${state.event_count}`);
  ui.writeLine(`  terminal_outcome: ${state.terminal_outcome || "none"}`);
  if (state.closed_reason !== "") {
    ui.writeLine(`  closed_reason: ${state.closed_reason}`);
  }
}

function renderModelCatalog(
  ui: LineWriter,
  catalog: Awaited<ReturnType<typeof listModels>>,
  activeProfileID: string,
  activeModel: string,
): void {
  ui.writeLine("models:");
  ui.writeLine(`- profile: ${catalog.profile_id}`);
  ui.writeLine(`  default_model: ${catalog.default_model}`);
  if (catalog.models.length > 0) {
    const currentModel = activeProfileID === catalog.profile_id
      ? activeModel
      : "";
    const formattedModels = catalog.models.map((model) =>
      model === currentModel ? `${model} (current)` : model,
    );
    ui.writeLine(`  available: ${formattedModels.join(", ")}`);
  }
  const capabilities = formatCapabilities(catalog.capabilities);
  if (capabilities !== "") {
    ui.writeLine(`  capabilities: ${capabilities}`);
  }
}

function renderCandidateValidation(
  ui: LineWriter,
  validation: CandidateValidationResult,
): void {
  const issues = validation.issues ?? [];
  ui.writeLine("candidate:");
  ui.writeLine(`- valid: ${validation.valid}`);
  ui.writeLine(`  root: ${validation.candidate.root}`);
  ui.writeLine(`  engine_version: ${validation.candidate.engine_version}`);
  ui.writeLine(`  shell_version: ${validation.candidate.shell_version}`);
  ui.writeLine(`  default_profile: ${validation.candidate.default_profile_id}`);
  ui.writeLine(`  default_model: ${validation.candidate.default_model}`);
  if (issues.length > 0) {
    ui.writeLine(`  issues: ${issues.join(" | ")}`);
  }
}

function renderRunSummary(
  ui: LineWriter,
  summary: EvalRunSummary,
): void {
  ui.writeLine("runs:");
  ui.writeLine(`- artifact_root: ${summary.artifact_root}`);
  ui.writeLine(`  total_runs: ${summary.total_runs}`);
  ui.writeLine(`  completed: ${summary.completed}`);
  ui.writeLine(`  failed: ${summary.failed}`);
  ui.writeLine(`  average_score: ${summary.average_score.toFixed(2)}`);
  ui.writeLine(`  latest_run: ${summary.latest_run_id || "none"}`);
  ui.writeLine(`  latest_status: ${summary.latest_status || "none"}`);
  const failureCodes = Object.entries(summary.failure_codes ?? {});
  if (failureCodes.length > 0) {
    const rendered = failureCodes
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([code, count]) => `${code}=${count}`)
      .join(", ");
    ui.writeLine(`  failure_codes: ${rendered}`);
  }
}

function renderReplayEval(
  ui: LineWriter,
  evalRun: EvalRun,
): void {
  renderEvalRun(ui, evalRun);
}

function renderEvalRun(
  ui: LineWriter,
  evalRun: EvalRun,
): void {
  const caseResults = evalRun.case_results ?? [];
  ui.writeLine("run:");
  ui.writeLine(`- run: ${evalRun.id}`);
  ui.writeLine(`  kind: ${evalRun.kind}`);
  ui.writeLine(`  status: ${evalRun.status}`);
  ui.writeLine(`  score: ${evalRun.score.toFixed(2)}`);
  ui.writeLine(`  candidate_root: ${evalRun.candidate.root}`);
  if (evalRun.replay_path !== "") {
    ui.writeLine(`  replay_path: ${evalRun.replay_path}`);
  }
  if (evalRun.benchmark) {
    ui.writeLine(`  benchmark: ${evalRun.benchmark.name}`);
    ui.writeLine(`  benchmark_path: ${evalRun.benchmark.path}`);
    ui.writeLine(`  cases: ${evalRun.benchmark.case_count}`);
  }
  if (caseResults.length > 0) {
    ui.writeLine(`  case_results: ${caseResults.length}`);
  }
  if (evalRun.failure) {
    ui.writeLine(`  failure_code: ${evalRun.failure.code}`);
    ui.writeLine(`  failure_message: ${evalRun.failure.message}`);
    ui.writeLine(`  retryable: ${evalRun.failure.retryable}`);
  }
}

function renderFrontier(
  ui: LineWriter,
  entries: FrontierEntry[],
): void {
  ui.writeLine("frontier:");
  ui.writeLine(`- entries: ${entries.length}`);
  for (const entry of entries) {
    ui.writeLine(
      `  - ${entry.run_id} kind=${entry.kind} status=${entry.status} score=${entry.score.toFixed(2)}`,
    );
    if (entry.benchmark !== "") {
      ui.writeLine(`    benchmark: ${entry.benchmark}`);
    }
    if (entry.failure_code !== "") {
      ui.writeLine(`    failure_code: ${entry.failure_code}`);
    }
  }
}

function renderRunDiff(
  ui: LineWriter,
  diff: EvalRunDiff,
): void {
  ui.writeLine("diff:");
  ui.writeLine(`- left_run: ${diff.left_run_id}`);
  ui.writeLine(`  right_run: ${diff.right_run_id}`);
  ui.writeLine(`  left_status: ${diff.left_status}`);
  ui.writeLine(`  right_status: ${diff.right_status}`);
  ui.writeLine(`  left_score: ${diff.left_score.toFixed(2)}`);
  ui.writeLine(`  right_score: ${diff.right_score.toFixed(2)}`);
  ui.writeLine(`  score_delta: ${diff.score_delta.toFixed(2)}`);
  if (diff.left_failure_code !== "" || diff.right_failure_code !== "") {
    ui.writeLine(`  left_failure_code: ${diff.left_failure_code || "none"}`);
    ui.writeLine(`  right_failure_code: ${diff.right_failure_code || "none"}`);
  }
  if (diff.case_diffs.length > 0) {
    ui.writeLine(`  case_diffs: ${diff.case_diffs.length}`);
  }
}

function formatCapabilities(capabilities: ProfileStatus["capabilities"]): string {
  const enabled: string[] = [];
  if (capabilities.streaming) {
    enabled.push("streaming");
  }
  if (capabilities.tool_calling) {
    enabled.push("tool_calling");
  }
  if (capabilities.structured_outputs) {
    enabled.push("structured_outputs");
  }
  if (capabilities.token_counting) {
    enabled.push("token_counting");
  }
  if (capabilities.prompt_caching) {
    enabled.push("prompt_caching");
  }
  if (capabilities.reasoning_controls) {
    enabled.push("reasoning_controls");
  }
  if (capabilities.deferred_tool_search) {
    enabled.push("deferred_tool_search");
  }
  if (capabilities.image_input) {
    enabled.push("image_input");
  }
  if (capabilities.document_input) {
    enabled.push("document_input");
  }
  return enabled.join(", ");
}


function printHelp(): void {
  const help = [
    "Usage: npm run dev -- [options]",
    "",
    "Options:",
    "  --prompt <text>            Send one prompt, stream results, and exit",
    "  --session-id <id>          Session identifier",
    "  --resume-session <id>      Resume a persisted session",
    "  --cwd <path>               Session working directory",
    "  --profile-id <id>          Active auth profile id",
    "  --model <id>               Active model id",
    "  --state-root <path>        Engine state root",
    "  --raw-events               Print canonical event JSON lines",
    "",
    "Interactive commands:",
    "  /profiles",
    "  /status",
    "  /summarize-runs",
    "  /show-run <id>",
    "  /list-frontier [limit]",
    "  /diff-runs <left-run-id> <right-run-id>",
    "  /validate-candidate",
    "  /run-benchmark <path>",
    "  /run-replay <path>",
    "  /export-replay <path>",
    "  /models [profile-id]",
    "  /logout [anthropic|openrouter]",
    "  /profile <id>",
    "  /model <id>",
    "  /login anthropic oauth [model=<id>] [account_scope=claude|console] [api_base=<url>]",
    "  /login anthropic <env-var|credential-ref> [model=<id>] [api_base=<url>]",
    "  /login openrouter <env-var|credential-ref> [model=<id>] [api_base=<url>]",
  ];
  console.log(help.join("\n"));
}

main().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(message);
  process.exit(1);
});
