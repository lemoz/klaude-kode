import { createInterface } from "node:readline";
import process from "node:process";

import {
  makeApprovePermissionCommand,
  defaultShellConfig,
  makeCloseSessionCommand,
  makeDenyPermissionCommand,
  listProfiles,
  loginOpenRouter,
  makeUpdateSessionSettingCommand,
  makeUserInputCommand,
  startEngineSession,
  type PermissionEventPayload,
  type ProfileStatus,
  type SessionEvent,
  type SessionStateSnapshot,
  type ShellConfig,
} from "./engineClient.js";

async function main(): Promise<void> {
  const config = parseArgs(process.argv.slice(2));
  const renderer = createRenderer(config.rawEvents);
  const session = await startEngineSession(config, (event) => {
    renderer.handleEvent(event);
  });

  try {
    if (config.prompt !== "") {
      await session.sendCommand(makeUserInputCommand(config.prompt));
      await session.sendCommand(makeCloseSessionCommand("shell_prompt_complete"));
      await session.closeInput();
      await session.done;
      return;
    }

    console.log("Klaude Kode shell");
    console.log("Type /exit to close the session.");
    await runInteractiveLoop(session, renderer, config);
    await session.done;
  } finally {
    await session.closeInput().catch(() => undefined);
  }
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

async function runInteractiveLoop(
  session: Awaited<ReturnType<typeof startEngineSession>>,
  ui: ReturnType<typeof createRenderer>,
  config: ShellConfig,
): Promise<void> {
  const rl = createInterface({
    input: process.stdin,
    output: process.stdout,
    terminal: process.stdin.isTTY && process.stdout.isTTY,
  });

  try {
    ui.showPrompt(ui.currentPrompt());
    let closedByUser = false;

    for await (const rawLine of rl) {
      try {
        const line = rawLine.trim();
        if (line === "") {
          ui.showPrompt(ui.currentPrompt());
          continue;
        }
        if (line === "/exit") {
          await session.sendCommand(makeCloseSessionCommand("shell_exit"));
          await session.closeInput();
          closedByUser = true;
          break;
        }
        const slashCommand = parseSlashCommand(line);
        if (slashCommand?.kind === "setting") {
          const decisionVersion = ui.decisionVersion();
          await session.sendCommand(makeUpdateSessionSettingCommand(slashCommand.key, slashCommand.value));
          await ui.waitForDecision(decisionVersion);
          ui.showPrompt(ui.currentPrompt());
          continue;
        }
        if (slashCommand?.kind === "profiles") {
          const profiles = await listProfiles(config);
          renderProfiles(ui, profiles);
          ui.showPrompt(ui.currentPrompt());
          continue;
        }
        if (slashCommand?.kind === "login_openrouter") {
          const profiles = await loginOpenRouter(config, {
            credential: slashCommand.credential,
            defaultModel: slashCommand.defaultModel,
            apiBase: slashCommand.apiBase,
          });
          ui.writeLine("login: saved openrouter-main and set it as default");
          renderProfiles(ui, profiles);
          const decisionVersion = ui.decisionVersion();
          await session.sendCommand(makeUpdateSessionSettingCommand("profile_id", "openrouter-main"));
          await ui.waitForDecision(decisionVersion);
          ui.showPrompt(ui.currentPrompt());
          continue;
        }
        const pendingPermission = ui.takePendingPermission();
        if (pendingPermission) {
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
          ui.showPrompt(ui.currentPrompt());
          continue;
        }
        const decisionVersion = ui.decisionVersion();
        await session.sendCommand(makeUserInputCommand(line, { askForPermission: true }));
        await ui.waitForDecision(decisionVersion);
        ui.showPrompt(ui.currentPrompt());
      } catch (error) {
        const message = error instanceof Error ? error.message : String(error);
        ui.writeLine(`error: ${message}`);
        ui.showPrompt(ui.currentPrompt());
      }
    }

    if (!closedByUser) {
      await session.sendCommand(makeCloseSessionCommand("shell_eof"));
      await session.closeInput();
    }
  } finally {
    rl.close();
  }
}

function createRenderer(rawEvents: boolean) {
  let headerPrinted = false;
  let currentState: SessionStateSnapshot | null = null;
  const pendingPermissions: PermissionEventPayload[] = [];
  let decisionSignals = 0;
  let notifyDecision: (() => void) | null = null;

  const print = (line: string) => {
    console.log(line);
  };

  const signalDecision = () => {
    decisionSignals += 1;
    const waiter = notifyDecision;
    notifyDecision = null;
    waiter?.();
  };

  return {
    handleEvent(event: SessionEvent): void {
      if (rawEvents) {
        print(JSON.stringify(event));
        return;
      }

      const state = event.payload.state;
      if (!headerPrinted && state) {
        print(`session: ${event.session_id}`);
        print(`mode: ${state.mode}`);
        print(`model: ${state.model}`);
        headerPrinted = true;
        currentState = state;
      } else if (
        event.kind === "session_state" &&
        state &&
        currentState &&
        state.status === "active" &&
        (state.model !== currentState.model ||
          state.profile_id !== currentState.profile_id)
      ) {
        print(`session: model=${state.model} profile=${state.profile_id}`);
        currentState = state;
      } else if (state) {
        currentState = state;
      }

      const rendered = renderEvent(event);
      if (rendered !== "") {
        print(rendered);
      }

      if (event.kind === "permission_requested" && event.payload.permission) {
        pendingPermissions.push(event.payload.permission);
      }
      if (
        event.kind === "permission_requested" ||
        event.kind === "session_state" ||
        event.kind === "assistant_message" ||
        event.kind === "failure" ||
        event.kind === "session_closed"
      ) {
        signalDecision();
      }
    },
    decisionVersion(): number {
      return decisionSignals;
    },
    waitForDecision(afterVersion: number): Promise<void> {
      if (decisionSignals > afterVersion) {
        return Promise.resolve();
      }
      return new Promise((resolve) => {
        notifyDecision = resolve;
      });
    },
    currentPrompt(): string {
      const pending = pendingPermissions[0];
      if (pending) {
        return `${pending.prompt} [y/N] `;
      }
      return "klaude> ";
    },
    takePendingPermission(): PermissionEventPayload | undefined {
      return pendingPermissions.shift();
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

function renderEvent(event: SessionEvent): string {
  switch (event.kind) {
    case "user_message_accepted":
      return event.payload.message?.content
        ? `user: ${event.payload.message.content}`
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
        ? `failure:${event.payload.failure.category} ${event.payload.failure.message}`
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
  | { kind: "login_openrouter"; credential: string; defaultModel?: string; apiBase?: string }
  | null {
  const trimmed = line.trim();
  if (!trimmed.startsWith("/")) {
    return null;
  }

  if (trimmed === "/profiles") {
    return { kind: "profiles" };
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
): { kind: "login_openrouter"; credential: string; defaultModel?: string; apiBase?: string } | null {
  const parts = line.split(/\s+/).slice(1);
  if (parts.length < 2) {
    throw new Error("usage: /login openrouter <env-var|credential-ref> [model=<id>] [api_base=<url>]");
  }
  const provider = parts[0]?.toLowerCase();
  if (provider !== "openrouter") {
    throw new Error(`unsupported login provider: ${parts[0]}`);
  }

  const credential = parts[1] ?? "";
  if (credential.trim() === "") {
    throw new Error("usage: /login openrouter <env-var|credential-ref> [model=<id>] [api_base=<url>]");
  }

  let defaultModel: string | undefined;
  let apiBase: string | undefined;
  for (const token of parts.slice(2)) {
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

  return {
    kind: "login_openrouter",
    credential,
    defaultModel,
    apiBase,
  };
}

function renderProfiles(
  ui: ReturnType<typeof createRenderer>,
  profiles: ProfileStatus[],
): void {
  ui.writeLine("profiles:");
  for (const entry of profiles) {
    ui.writeLine(
      `- ${entry.profile.id} (${entry.profile.provider}/${entry.profile.kind}) default_model=${entry.profile.default_model} valid=${entry.validation.valid}`,
    );
    if (entry.validation.message !== "") {
      ui.writeLine(`  validation: ${entry.validation.message}`);
    }
    if (entry.models.length > 0) {
      ui.writeLine(`  models: ${entry.models.join(", ")}`);
    }
  }
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
    "  /profile <id>",
    "  /model <id>",
    "  /login openrouter <env-var|credential-ref> [model=<id>] [api_base=<url>]",
  ];
  console.log(help.join("\n"));
}

main().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(message);
  process.exit(1);
});
