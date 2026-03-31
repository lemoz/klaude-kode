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

export interface AuthProfile {
  id: string;
  kind: string;
  provider: string;
  display_name: string;
  default_model: string;
  settings: Record<string, string>;
}

export interface ProfileValidationResult {
  valid: boolean;
  message: string;
}

export interface ProfileAuthStatus {
  state: string;
  expires_at: string;
  can_refresh: boolean;
}

export interface CapabilitySet {
  streaming: boolean;
  tool_calling: boolean;
  structured_outputs: boolean;
  token_counting: boolean;
  prompt_caching: boolean;
  reasoning_controls: boolean;
  deferred_tool_search: boolean;
  image_input: boolean;
  document_input: boolean;
}

export interface ProfileStatus {
  profile: AuthProfile;
  validation: ProfileValidationResult;
  models: string[];
  capabilities: CapabilitySet;
  auth: ProfileAuthStatus;
}

export interface ModelCatalog {
  profile_id: string;
  default_model: string;
  models: string[];
  capabilities: CapabilitySet;
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

export interface SessionCommandPayload {
  text?: string;
  reason?: string;
  request_id?: string;
  source?: string;
  setting_key?: string;
  setting_value?: string;
  metadata?: Record<string, string>;
}

export interface SessionCommand {
  kind: string;
  payload: SessionCommandPayload;
}

export interface EngineSession {
  sendCommand(command: SessionCommand): Promise<void>;
  closeInput(): Promise<void>;
  done: Promise<void>;
}

const sourceDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(sourceDir, "../..");

export async function startEngineSession(
  config: ShellConfig,
  onEvent: (event: SessionEvent) => void,
): Promise<EngineSession> {
  const args = buildEngineArgs(config);
  const child = spawn("go", args, {
    cwd: repoRoot,
    env: process.env,
    stdio: ["pipe", "pipe", "pipe"],
  });

  if (!child.stdin || !child.stdout || !child.stderr) {
    throw new Error("cc-engine stdio pipes are unavailable");
  }

  const lines = createInterface({ input: child.stdout });
  const readEvents = (async () => {
    for await (const line of lines) {
      const trimmed = line.trim();
      if (trimmed === "") {
        continue;
      }
      const event = JSON.parse(trimmed) as SessionEvent;
      onEvent(event);
    }
  })();

  let stderr = "";
  child.stderr.on("data", (chunk: Buffer | string) => {
    stderr += chunk.toString();
  });

  const done = (async () => {
    const exitCode = await new Promise<number>((resolve, reject) => {
      child.on("error", reject);
      child.on("close", (code) => resolve(code ?? 1));
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
  })();

  return {
    async sendCommand(command: SessionCommand): Promise<void> {
      if (child.stdin.destroyed) {
        throw new Error("cc-engine command stream is closed");
      }
      await writeLine(child.stdin, JSON.stringify(command));
    },
    async closeInput(): Promise<void> {
      if (child.stdin.destroyed || child.stdin.writableEnded) {
        return;
      }
      await new Promise<void>((resolve, reject) => {
        child.stdin.end((error?: Error | null) => {
          if (error) {
            reject(error);
            return;
          }
          resolve();
        });
      });
    },
    done,
  };
}

export function makeUserInputCommand(
  text: string,
  options?: { askForPermission?: boolean },
): SessionCommand {
  return {
    kind: "user_input",
    payload: {
      text,
      source: "interactive",
      metadata: options?.askForPermission ? { permission_mode: "ask" } : undefined,
    },
  };
}

export function makeCloseSessionCommand(reason: string): SessionCommand {
  return {
    kind: "close_session",
    payload: {
      reason,
    },
  };
}

export function makeApprovePermissionCommand(requestID: string): SessionCommand {
  return {
    kind: "approve_permission",
    payload: {
      request_id: requestID,
    },
  };
}

export function makeDenyPermissionCommand(requestID: string): SessionCommand {
  return {
    kind: "deny_permission",
    payload: {
      request_id: requestID,
    },
  };
}

export function makeUpdateSessionSettingCommand(
  key: string,
  value: string,
): SessionCommand {
  return {
    kind: "update_session_setting",
    payload: {
      setting_key: key,
      setting_value: value,
    },
  };
}

export async function listProfiles(config: Pick<ShellConfig, "stateRoot">): Promise<ProfileStatus[]> {
  const stdout = await runEngineAdminCommand([
    "-format=json",
    "-profiles",
    `-state-root=${config.stateRoot}`,
  ]);
  const parsed = JSON.parse(stdout) as { profiles?: ProfileStatus[] };
  return parsed.profiles ?? [];
}

export async function listModels(
  config: Pick<ShellConfig, "stateRoot">,
  profileID?: string,
): Promise<ModelCatalog> {
  const args = [
    "-format=json",
    "-models",
    `-state-root=${config.stateRoot}`,
  ];
  if (profileID && profileID.trim() !== "") {
    args.push(`-profile-id=${profileID.trim()}`);
  }
  const stdout = await runEngineAdminCommand(args);
  return JSON.parse(stdout) as ModelCatalog;
}

export async function logoutProfile(
  config: Pick<ShellConfig, "stateRoot">,
  profileID: string,
): Promise<ProfileStatus[]> {
  const stdout = await runEngineAdminCommand([
    "-format=json",
    `-logout-profile=${profileID}`,
    `-state-root=${config.stateRoot}`,
  ]);
  const parsed = JSON.parse(stdout) as { profiles?: ProfileStatus[] };
  return parsed.profiles ?? [];
}

export async function loginOpenRouter(
  config: Pick<ShellConfig, "stateRoot">,
  input: {
    credential: string;
    defaultModel?: string;
    apiBase?: string;
  },
): Promise<ProfileStatus[]> {
  const args = [
    "-format=json",
    "-upsert-profile",
    "-profile-id=openrouter-main",
    "-provider=openrouter",
    "-profile-kind=openrouter_api_key",
    "-display-name=OpenRouter Main",
    `-credential-ref=${normalizeCredentialRef(input.credential)}`,
    "-make-default",
    `-state-root=${config.stateRoot}`,
  ];

  if (input.defaultModel && input.defaultModel.trim() !== "") {
    args.push(`-default-model=${input.defaultModel.trim()}`);
  }
  if (input.apiBase && input.apiBase.trim() !== "") {
    args.push(`-api-base=${input.apiBase.trim()}`);
  }

  const stdout = await runEngineAdminCommand(args);
  const parsed = JSON.parse(stdout) as { profiles?: ProfileStatus[] };
  return parsed.profiles ?? [];
}

export async function loginAnthropic(
  config: Pick<ShellConfig, "stateRoot">,
  input: {
    credential: string;
    defaultModel?: string;
    apiBase?: string;
  },
): Promise<ProfileStatus[]> {
  const args = [
    "-format=json",
    "-upsert-profile",
    "-profile-id=anthropic-api",
    "-provider=anthropic",
    "-profile-kind=anthropic_api_key",
    "-display-name=Anthropic API",
    `-credential-ref=${normalizeCredentialRef(input.credential)}`,
    "-make-default",
    `-state-root=${config.stateRoot}`,
  ];

  if (input.defaultModel && input.defaultModel.trim() !== "") {
    args.push(`-default-model=${input.defaultModel.trim()}`);
  }
  if (input.apiBase && input.apiBase.trim() !== "") {
    args.push(`-api-base=${input.apiBase.trim()}`);
  }

  const stdout = await runEngineAdminCommand(args);
  const parsed = JSON.parse(stdout) as { profiles?: ProfileStatus[] };
  return parsed.profiles ?? [];
}

export async function loginAnthropicOAuth(
  config: Pick<ShellConfig, "stateRoot">,
  input: {
    defaultModel?: string;
    apiBase?: string;
    accountScope?: string;
  },
): Promise<ProfileStatus[]> {
  const args = [
    "-format=json",
    "-anthropic-oauth-login",
    "-profile-id=anthropic-main",
    "-display-name=Anthropic Main",
    "-make-default",
    `-state-root=${config.stateRoot}`,
  ];

  if (input.defaultModel && input.defaultModel.trim() !== "") {
    args.push(`-default-model=${input.defaultModel.trim()}`);
  }
  if (input.apiBase && input.apiBase.trim() !== "") {
    args.push(`-api-base=${input.apiBase.trim()}`);
  }
  if (input.accountScope && input.accountScope.trim() !== "") {
    args.push(`-account-scope=${input.accountScope.trim()}`);
  }

  const stdout = await runEngineAdminCommand(args, { inheritStderr: true });
  const parsed = JSON.parse(stdout) as { profiles?: ProfileStatus[] };
  return parsed.profiles ?? [];
}

function buildEngineArgs(config: ShellConfig): string[] {
  const args = [
    "run",
    "./cmd/cc-engine",
    "-transport=stdio",
    "-format=events",
    `-cwd=${config.cwd}`,
    `-profile-id=${config.profileId}`,
    `-model=${config.model}`,
    `-state-root=${config.stateRoot}`,
  ];

  if (config.resumeSessionId !== "") {
    args.push(`-resume-session=${config.resumeSessionId}`);
  } else {
    args.push(`-session-id=${config.sessionId}`);
  }

  return args;
}

async function runEngineAdminCommand(
  args: string[],
  options?: { inheritStderr?: boolean },
): Promise<string> {
  const child = spawn("go", ["run", "./cmd/cc-engine", ...args], {
    cwd: repoRoot,
    env: process.env,
    stdio: ["ignore", "pipe", "pipe"],
  });

  if (!child.stdout || !child.stderr) {
    throw new Error("cc-engine admin stdio pipes are unavailable");
  }

  const stdoutChunks: Buffer[] = [];
  const stderrChunks: Buffer[] = [];
  child.stdout.on("data", (chunk: Buffer | string) => {
    stdoutChunks.push(Buffer.from(chunk));
  });
  child.stderr.on("data", (chunk: Buffer | string) => {
    const buffer = Buffer.from(chunk);
    stderrChunks.push(buffer);
    if (options?.inheritStderr) {
      process.stderr.write(buffer);
    }
  });

  const exitCode = await new Promise<number>((resolve, reject) => {
    child.on("error", reject);
    child.on("close", (code) => resolve(code ?? 1));
  });

  const stdout = Buffer.concat(stdoutChunks).toString("utf8");
  const stderr = Buffer.concat(stderrChunks).toString("utf8").trim();
  if (exitCode !== 0) {
    throw new Error(
      stderr === ""
        ? `cc-engine admin command exited with code ${exitCode}`
        : `cc-engine admin command exited with code ${exitCode}: ${stderr}`,
    );
  }
  return stdout;
}

function writeLine(
  writable: NodeJS.WritableStream,
  line: string,
): Promise<void> {
  return new Promise((resolve, reject) => {
    writable.write(`${line}\n`, (error?: Error | null) => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
  });
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
    prompt: "",
    sessionId: "shell-bootstrap",
    resumeSessionId: "",
    cwd: repoRoot,
    profileId: "",
    model: "",
    stateRoot: defaultStateRoot(),
    rawEvents: false,
  };
}

function normalizeCredentialRef(value: string): string {
  const trimmed = value.trim();
  if (trimmed.includes("://")) {
    return trimmed;
  }
  if (trimmed.startsWith("env:")) {
    return `env://${trimmed.slice("env:".length)}`;
  }
  return `env://${trimmed}`;
}
