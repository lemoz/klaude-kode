import { spawn } from "node:child_process";
import { createInterface } from "node:readline";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

export interface ShellConfig {
  prompt: string;
  sessionId: string;
  resumeSessionId: string;
  cwd: string;
  profileId: string;
  model: string;
  stateRoot: string;
  rawEvents: boolean;
}

export interface CanonicalMessage {
  role: string;
  content: string;
}

export interface ToolEventPayload {
  call_id: string;
  name: string;
  input: Record<string, unknown>;
  concurrency_class: string;
  progress_message: string;
  result_summary: string;
  output: string;
  failed: boolean;
}

export interface PermissionEventPayload {
  request_id: string;
  tool_call_id: string;
  policy_source: string;
  prompt: string;
  scope: string;
  resolution: string;
  actor: string;
}

export interface FailurePayload {
  category: string;
  code: string;
  message: string;
  retryable: boolean;
}

export interface SessionStateSnapshot {
  cwd: string;
  mode: string;
  status: string;
  profile_id: string;
  model: string;
  event_count: number;
  turn_count: number;
  last_sequence: number;
  closed_reason: string;
  terminal_outcome: string;
}

export interface SessionEventPayload {
  command_id: string;
  turn_id: string;
  source: string;
  message: CanonicalMessage | null;
  state: SessionStateSnapshot | null;
  tool: ToolEventPayload | null;
  permission: PermissionEventPayload | null;
  lifecycle_name: string;
  terminal_outcome: string;
  warning: string;
  failure: FailurePayload | null;
  reason: string;
}

export interface SessionEvent {
  schema_version: string;
  session_id: string;
  sequence: number;
  timestamp: string;
  kind: string;
  payload: SessionEventPayload;
}

const sourceDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(sourceDir, "../..");

export async function runEngineSession(
  config: ShellConfig,
): Promise<SessionEvent[]> {
  const args = buildEngineArgs(config);
  const child = spawn("go", args, {
    cwd: repoRoot,
    env: process.env,
    stdio: ["ignore", "pipe", "pipe"],
  });

  if (!child.stdout || !child.stderr) {
    throw new Error("cc-engine stdio pipes are unavailable");
  }

  const events: SessionEvent[] = [];
  const lines = createInterface({ input: child.stdout });

  const readEvents = (async () => {
    for await (const line of lines) {
      const trimmed = line.trim();
      if (trimmed === "") {
        continue;
      }
      const event = JSON.parse(trimmed) as SessionEvent;
      events.push(event);
    }
  })();

  let stderr = "";
  child.stderr.on("data", (chunk: Buffer | string) => {
    stderr += chunk.toString();
  });

  const exitCode = await new Promise<number>((resolve, reject) => {
    child.on("error", reject);
    child.on("close", resolve);
  });

  await readEvents;

  if (exitCode !== 0) {
    const details = stderr.trim();
    throw new Error(
      details === ""
        ? `cc-engine exited with code ${exitCode}`
        : `cc-engine exited with code ${exitCode}: ${details}`,
    );
  }

  return events;
}

function buildEngineArgs(config: ShellConfig): string[] {
  const args = ["run", "./cmd/cc-engine", "-format=events"];

  if (config.resumeSessionId !== "") {
    args.push(`-resume-session=${config.resumeSessionId}`);
  } else {
    args.push(`-prompt=${config.prompt}`);
    args.push(`-session-id=${config.sessionId}`);
  }

  args.push(`-cwd=${config.cwd}`);
  args.push(`-profile-id=${config.profileId}`);
  args.push(`-model=${config.model}`);
  args.push(`-state-root=${config.stateRoot}`);

  return args;
}

export function defaultStateRoot(): string {
  const home = process.env.HOME;
  if (!home) {
    return ".claude-next";
  }
  return path.join(home, ".claude-next");
}

export function defaultShellConfig(): ShellConfig {
  return {
    prompt: "bootstrap hello from cc-shell",
    sessionId: "shell-bootstrap",
    resumeSessionId: "",
    cwd: repoRoot,
    profileId: "shell-default",
    model: "shell-bootstrap-model",
    stateRoot: defaultStateRoot(),
    rawEvents: false,
  };
}
