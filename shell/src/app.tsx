import React, { useEffect, useRef } from "react";
import { Box, Text, useApp, useStdin } from "ink";
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
  authDetail: string;
  model: string;
  defaultModel: string;
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

export interface ArtifactSection {
  title: string;
  lines: string[];
}

export interface ArtifactView {
  title: string;
  summary: string[];
  sections: ArtifactSection[];
}

export interface InteractiveShellProps {
  header: InteractiveShellHeader;
  footer: InteractiveShellFooter;
  hints: string[];
  artifactView: ArtifactView | null;
  turns: TranscriptTurn[];
  lines: string[];
  pendingPermission: PermissionEventPayload | null;
  promptState: InteractivePromptState;
  inputValue: string;
  closed: boolean;
  onData: (input: string) => void;
}

type OperationTone = "info" | "warning" | "failure";

interface RenderedOperationLine {
  tone: OperationTone;
  text: string;
}

function formatTranscriptWarning(warning: string): string {
  if (warning.includes("capability fallback")) {
    return `capability fallback: ${warning}`;
  }
  return `warning: ${warning}`;
}

function formatTranscriptFailure(failure: NonNullable<TranscriptTurn["failure"]>): string {
  if (failure.code === "capability_mismatch") {
    return `capability rejected: ${failure.message}`;
  }
  return `failure: ${failure.category}/${failure.code} ${failure.message}`;
}

function classifyOperationLine(line: string): RenderedOperationLine {
  const trimmed = line.trim();
  if (trimmed.startsWith("warning:")) {
    const warningText = trimmed.slice("warning:".length).trim();
    if (warningText.includes("capability fallback")) {
      return {
        tone: "warning",
        text: `capability fallback: ${warningText}`,
      };
    }
    return {
      tone: "warning",
      text: trimmed,
    };
  }
  if (trimmed.startsWith("failure:")) {
    const detail = trimmed.replace(/^failure:[^\s]+\s+retryable=\w+\s+/, "");
    if (trimmed.includes("/capability_mismatch")) {
      return {
        tone: "failure",
        text: `capability rejected: ${detail}`,
      };
    }
    return {
      tone: "failure",
      text: trimmed,
    };
  }
  return {
    tone: "info",
    text: trimmed,
  };
}

export function InteractiveShell(props: InteractiveShellProps) {
  const { exit } = useApp();
  const { stdin, setRawMode, isRawModeSupported } = useStdin();
  const visibleOperationLines = props.lines
    .filter((line) => line.trim() !== "")
    .map(classifyOperationLine);
  const onDataRef = useRef(props.onData);

  onDataRef.current = props.onData;

  useEffect(() => {
    if (props.closed) {
      exit();
    }
  }, [exit, props.closed]);

  useEffect(() => {
    if (props.closed) {
      return;
    }
    if (isRawModeSupported) {
      setRawMode(true);
    }

    const handleData = (chunk: Buffer | string) => {
      onDataRef.current(chunk.toString());
    };

    stdin.resume();
    stdin.on("data", handleData);

    return () => {
      stdin.off("data", handleData);
      if (isRawModeSupported) {
        setRawMode(false);
      }
    };
  }, [stdin, setRawMode, isRawModeSupported, props.closed]);

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
          <Text dimColor>
            profile_default={props.header.defaultModel} {props.header.authDetail}
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
      {props.artifactView ? (
        <Box borderStyle="round" borderColor={SHELL_THEME.activity} paddingX={1} flexDirection="column" marginTop={1}>
          <Text color={SHELL_THEME.activity}>{props.artifactView.title}</Text>
          {props.artifactView.summary.map((line, index) => (
            <Text key={`${props.artifactView?.title}:summary:${index}`}>{line}</Text>
          ))}
          {props.artifactView.sections.map((section, sectionIndex) => (
            <Box key={`${props.artifactView?.title}:section:${sectionIndex}`} flexDirection="column" marginTop={1}>
              <Text color={SHELL_THEME.brand}>{section.title}</Text>
              {section.lines.map((line, lineIndex) => (
                <Text key={`${section.title}:${lineIndex}`} dimColor>
                  {line}
                </Text>
              ))}
            </Box>
          ))}
        </Box>
      ) : null}
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
                  {formatTranscriptWarning(warning)}
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
                  {formatTranscriptFailure(turn.failure)}
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
      {visibleOperationLines.length > 0 ? (
        <Box borderStyle="round" borderColor={SHELL_THEME.activity} paddingX={1} flexDirection="column" marginTop={1}>
          <Text color={SHELL_THEME.activity}>Operations</Text>
          {visibleOperationLines.map((line, index) => (
            <Text
              key={`${index}:${line.text}`}
              color={
                line.tone === "failure"
                  ? SHELL_THEME.danger
                  : line.tone === "warning"
                    ? SHELL_THEME.warning
                    : undefined
              }
              bold={line.tone !== "info"}
              dimColor={line.tone === "info"}
            >
              {line.text}
            </Text>
          ))}
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
