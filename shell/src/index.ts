import process from "node:process";

import {
  defaultShellConfig,
  type SessionEvent,
  type SessionStateSnapshot,
  runEngineSession,
} from "./engineClient.js";

async function main(): Promise<void> {
  const config = parseArgs(process.argv.slice(2));
  const events = await runEngineSession(config);

  if (config.rawEvents) {
    console.log(JSON.stringify(events, null, 2));
    return;
  }

  console.log(renderTranscript(events));
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

  if (config.resumeSessionId !== "" && config.prompt !== "bootstrap hello from cc-shell") {
    throw new Error("--prompt cannot be used with --resume-session");
  }

  return config;
}

function renderTranscript(events: SessionEvent[]): string {
  const lines = ["Klaude Kode shell", `events: ${events.length}`];
  const initialState = firstState(events);
  if (initialState) {
    lines.push(`session: ${events[0]?.session_id ?? "unknown"}`);
    lines.push(`mode: ${initialState.mode}`);
    lines.push(`model: ${initialState.model}`);
  }

  for (const event of events) {
    switch (event.kind) {
      case "user_message_accepted":
        if (event.payload.message?.content) {
          lines.push(`user: ${event.payload.message.content}`);
        }
        break;
      case "assistant_message":
        if (event.payload.message?.content) {
          lines.push(`assistant: ${event.payload.message.content}`);
        }
        break;
      case "tool_call_requested":
        if (event.payload.tool) {
          lines.push(`tool:${event.payload.tool.name} requested`);
        }
        break;
      case "tool_call_progress":
        if (event.payload.tool?.progress_message) {
          lines.push(`tool:${event.payload.tool.name} ${event.payload.tool.progress_message}`);
        }
        break;
      case "tool_call_completed":
        if (event.payload.tool) {
          const output = event.payload.tool.output ? ` output=${event.payload.tool.output}` : "";
          lines.push(
            `tool:${event.payload.tool.name} ${event.payload.tool.result_summary}${output}`,
          );
        }
        break;
      case "permission_requested":
        if (event.payload.permission?.prompt) {
          lines.push(`permission: ${event.payload.permission.prompt}`);
        }
        break;
      case "permission_resolved":
        if (event.payload.permission?.resolution) {
          lines.push(`permission: ${event.payload.permission.resolution}`);
        }
        break;
      case "warning":
        if (event.payload.warning) {
          lines.push(`warning: ${event.payload.warning}`);
        }
        break;
      case "failure":
        if (event.payload.failure) {
          lines.push(
            `failure:${event.payload.failure.category} ${event.payload.failure.message}`,
          );
        }
        break;
      case "session_closed":
        lines.push(`status: closed (${event.payload.reason || "no_reason"})`);
        break;
      default:
        break;
    }
  }

  const finalState = lastState(events);
  if (finalState) {
    lines.push(`terminal_outcome: ${finalState.terminal_outcome || "none"}`);
  }

  return lines.join("\n");
}

function firstState(events: SessionEvent[]): SessionStateSnapshot | null {
  for (const event of events) {
    if (event.payload.state) {
      return event.payload.state;
    }
  }
  return null;
}

function lastState(events: SessionEvent[]): SessionStateSnapshot | null {
  for (let index = events.length - 1; index >= 0; index -= 1) {
    const state = events[index]?.payload.state;
    if (state) {
      return state;
    }
  }
  return null;
}

function printHelp(): void {
  const help = [
    "Usage: npm run dev -- [options]",
    "",
    "Options:",
    "  --prompt <text>            Prompt to send to cc-engine",
    "  --session-id <id>          Session identifier",
    "  --resume-session <id>      Load a persisted session instead of sending a prompt",
    "  --cwd <path>               Session working directory",
    "  --profile-id <id>          Active auth profile id",
    "  --model <id>               Active model id",
    "  --state-root <path>        Engine state root",
    "  --raw-events               Print parsed event JSON instead of a transcript",
  ];
  console.log(help.join("\n"));
}

main().catch((error: unknown) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(message);
  process.exit(1);
});
