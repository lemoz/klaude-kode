import React, { useEffect } from "react";
import { Box, Static, Text, useApp } from "ink";

export interface InteractiveShellHeader {
  sessionId: string;
  mode: string;
  profileId: string;
  provider: string;
  authState: string;
  model: string;
}

export interface InteractiveShellProps {
  header: InteractiveShellHeader;
  lines: string[];
  promptLabel: string;
  inputValue: string;
  busy: boolean;
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
      <Box borderStyle="round" borderColor="cyan" paddingX={1}>
        <Box flexDirection="column">
          <Text color="cyanBright">Klaude Kode</Text>
          <Text dimColor>
            session={props.header.sessionId} mode={props.header.mode}
          </Text>
          <Text dimColor>
            profile={props.header.profileId} provider={props.header.provider} auth={props.header.authState} model={props.header.model}
          </Text>
        </Box>
      </Box>
      <Box marginTop={1}>
        <Text dimColor>Type /help for commands. Type /exit to close the session.</Text>
      </Box>
      <Box flexDirection="column" marginTop={1}>
        <Static items={props.lines}>
          {(line, index) => <Text key={`${index}:${line}`}>{line}</Text>}
        </Static>
      </Box>
      <Box marginTop={1}>
        <Text color="green">{props.promptLabel}</Text>
        <Text>{props.inputValue}</Text>
        {props.busy ? <Text dimColor> ...</Text> : null}
      </Box>
    </Box>
  );
}
