import { createInterface } from "node:readline";
import process from "node:process";

import {
  defaultShellConfig,
  makeCloseSessionCommand,
  makeUserInputCommand,
  startEngineSession,
  type SessionEvent,
  type SessionStateSnapshot,
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
    await runInteractiveLoop(session, renderer.showPrompt);
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
  showPrompt: () => void,
): Promise<void> {
  const rl = createInterface({
    input: process.stdin,
    output: process.stdout,
    terminal: process.stdin.isTTY && process.stdout.isTTY,
  });

  try {
    showPrompt();
    let closedByUser = false;

    for await (const rawLine of rl) {
      const line = rawLine.trim();
      if (line === "") {
        showPrompt();
        continue;
      }
      if (line === "/exit") {
        await session.sendCommand(makeCloseSessionCommand("shell_exit"));
        await session.closeInput();
        closedByUser = true;
        break;
      }
      await session.sendCommand(makeUserInputCommand(line));
      showPrompt();
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

  const print = (line: string) => {
    console.log(line);
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
      }

      const rendered = renderEvent(event);
      if (rendered !== "") {
        print(rendered);
      }
    },
    showPrompt(): void {
      if (!rawEvents && process.stdout.isTTY) {
        process.stdout.write("klaude> ");
      }
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
  ];
  console.log(help.join("\n"));
}

main().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(message);
  process.exit(1);
});
