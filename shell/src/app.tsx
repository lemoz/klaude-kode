import React, { useEffect } from "react";
import { Box, Static, Text, useApp } from "ink";
import type { TranscriptTurn } from "./presentation.js";
import type { PermissionEventPayload } from "./engineClient.js";

const SHELL_THEME = {
  brand: "cyanBright",
  accent: "yellowBright",
  shell: "cyan",
  transcript: "blue",
  activity: "magenta",
  warning: "yellow",
  danger: "red",
  chrome: "gray",
} as const;

export interface InteractiveShellHeader {
  sessionId: string;
  mode: string;
  profileId: string;
  provider: string;
  authState: string;
  model: string;
}

export interface InteractiveShellFooter {
  surface: string;
  sessionStatus: string;
  terminalOutcome: string;
  pendingPermission: string;
  modeCue: string;
}

export interface InteractivePromptState {
  mode: "ready" | "decision" | "read_only" | "closed";
  label: string;
  hint: string;
  editable: boolean;
}

export interface InteractiveShellProps {
  header: InteractiveShellHeader;
  footer: InteractiveShellFooter;
  hints: string[];
  turns: TranscriptTurn[];
  lines: string[];
  pendingPermission: PermissionEventPayload | null;
  promptState: InteractivePromptState;
  inputValue: string;
  closed: boolean;
}

export function InteractiveShell(props: InteractiveShellProps) {
  const { exit } = useApp();

  useEffect(() => {
    if (props.closed) {
      exit();
    }
  }, [exit, props.closed]);

  return (
    <Box flexDirection="column">
      <Box borderStyle="round" borderColor={SHELL_THEME.shell} paddingX={1}>
        <Box flexDirection="column">
          <Box>
            <Text backgroundColor={SHELL_THEME.shell} color="black"> KK </Text>
            <Text color={SHELL_THEME.brand}> Klaude Kode Terminal</Text>
          </Box>
          <Text dimColor>
            session={props.header.sessionId} mode={props.header.mode}
          </Text>
          <Text dimColor>
            profile={props.header.profileId} provider={props.header.provider} auth={props.header.authState} model={props.header.model}
          </Text>
        </Box>
      </Box>
      <Box marginTop={1}>
        <Box flexDirection="column">
          <Text dimColor>Klaude Kode commands live under /help. Use /exit to close the session.</Text>
          {props.hints.map((hint, index) => (
            <Text key={`${index}:${hint}`} dimColor>
              {hint}
            </Text>
          ))}
        </Box>
      </Box>
      <Box flexDirection="column" marginTop={1}>
        {props.turns.map((turn) => {
          const assistantText = turn.assistantFinal || turn.assistantStream;
          return (
            <Box
              key={turn.turnId}
              flexDirection="column"
              borderStyle="round"
              borderColor={turn.failure ? SHELL_THEME.danger : SHELL_THEME.transcript}
              paddingX={1}
              marginBottom={1}
            >
              <Text color={SHELL_THEME.transcript}>turn {turn.turnId}</Text>
              {turn.userMessage !== "" ? (
                <Text color={SHELL_THEME.accent}>you: {turn.userMessage}</Text>
              ) : null}
              {turn.toolCalls.map((tool) => {
                const lastProgress = tool.progressMessages[tool.progressMessages.length - 1] ?? "";
                const status = tool.resultSummary || lastProgress || "requested";
                const detail = tool.output !== "" ? `${status} output=${tool.output}` : status;
                return (
                  <Text key={tool.callId} color={tool.failed ? SHELL_THEME.danger : SHELL_THEME.activity}>
                    tool:{tool.name} {detail}
                  </Text>
                );
              })}
              {turn.warnings.map((warning, index) => (
                <Text key={`${turn.turnId}:warning:${index}`} color={SHELL_THEME.warning}>
                  warning: {warning}
                </Text>
              ))}
              {assistantText !== "" ? (
                <Text color={SHELL_THEME.brand}>
                  klaude: {assistantText}
                  {turn.assistantFinal === "" ? " ..." : ""}
                </Text>
              ) : null}
              {turn.failure ? (
                <Text color={SHELL_THEME.danger}>
                  failure: {turn.failure.category}/{turn.failure.code} {turn.failure.message}
                </Text>
              ) : null}
            </Box>
          );
        })}
      </Box>
      {props.pendingPermission ? (
        <Box
          borderStyle="round"
          borderColor={SHELL_THEME.warning}
          paddingX={1}
          flexDirection="column"
          marginTop={1}
        >
          <Text color={SHELL_THEME.accent}>Pending Permission</Text>
          <Text>{props.pendingPermission.prompt}</Text>
          <Text dimColor>
            scope={props.pendingPermission.scope} policy={props.pendingPermission.policy_source}
          </Text>
          <Text dimColor>
            request={props.pendingPermission.request_id} tool_call={props.pendingPermission.tool_call_id}
          </Text>
          <Text dimColor>approve with y/yes, deny with n/no</Text>
        </Box>
      ) : null}
      {props.lines.length > 0 ? (
        <Box borderStyle="round" borderColor={SHELL_THEME.activity} paddingX={1} flexDirection="column" marginTop={1}>
          <Text color={SHELL_THEME.activity}>Operations</Text>
          <Static items={props.lines}>
            {(line, index) => (
              <Text key={`${index}:${line}`} dimColor>
                {line}
              </Text>
            )}
          </Static>
        </Box>
      ) : null}
      <Box borderStyle="round" borderColor={SHELL_THEME.chrome} paddingX={1} marginTop={1}>
        <Text dimColor>
          surface={props.footer.surface} status={props.footer.sessionStatus} outcome={props.footer.terminalOutcome} pending={props.footer.pendingPermission} cue={props.footer.modeCue}
        </Text>
      </Box>
      <Box borderStyle="round" borderColor={props.promptState.mode === "decision" ? SHELL_THEME.warning : SHELL_THEME.shell} paddingX={1} marginTop={1}>
        <Text color={props.promptState.mode === "decision" ? SHELL_THEME.warning : SHELL_THEME.shell}>
          {props.promptState.label}
        </Text>
        <Text>{props.inputValue}</Text>
        {props.promptState.hint !== "" ? <Text dimColor>  {props.promptState.hint}</Text> : null}
      </Box>
    </Box>
  );
}
