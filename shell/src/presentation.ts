import type {
  FailurePayload,
  PermissionEventPayload,
  SessionEvent,
  SessionStateSnapshot,
  ToolEventPayload,
} from "./engineClient.js";

export interface TranscriptToolActivity {
  callId: string;
  name: string;
  input: Record<string, unknown>;
  concurrencyClass: string;
  progressMessages: string[];
  resultSummary: string;
  output: string;
  failed: boolean;
}

export interface TranscriptTurn {
  turnId: string;
  userMessage: string;
  assistantStream: string;
  assistantFinal: string;
  toolCalls: TranscriptToolActivity[];
  warnings: string[];
  failure: FailurePayload | null;
}

export interface ShellPresentationState {
  rawEvents: boolean;
  currentState: SessionStateSnapshot | null;
  pendingPermissions: PermissionEventPayload[];
  transcript: TranscriptTurn[];
}

export class ShellPresentationModel {
  private readonly state: ShellPresentationState;
  private readonly transcriptIndex = new Map<string, number>();
  private letDecisionSignals = 0;
  private notifyDecision: (() => void) | null = null;

  constructor(rawEvents: boolean) {
    this.state = {
      rawEvents,
      currentState: null,
      pendingPermissions: [],
      transcript: [],
    };
  }

  applyEvent(event: SessionEvent): void {
    if (event.payload.state) {
      this.state.currentState = event.payload.state;
    }

    switch (event.kind) {
      case "user_message_accepted":
        this.setTurnUserMessage(event.payload.turn_id, event.payload.message?.content ?? "");
        break;
      case "assistant_delta":
        this.appendAssistantStream(event.payload.turn_id, event.payload.message?.content ?? "");
        break;
      case "assistant_message":
        this.setAssistantFinal(event.payload.turn_id, event.payload.message?.content ?? "");
        break;
      case "tool_call_requested":
        if (event.payload.tool) {
          this.upsertToolActivity(event.payload.turn_id, event.payload.tool);
        }
        break;
      case "tool_call_progress":
      case "tool_call_completed":
        if (event.payload.tool) {
          this.mergeToolActivity(event.payload.turn_id, event.payload.tool);
        }
        break;
      case "warning":
        if (event.payload.warning !== "") {
          this.appendWarning(event.payload.turn_id, event.payload.warning);
        }
        break;
      case "failure":
        if (event.payload.failure) {
          this.setFailure(event.payload.turn_id, event.payload.failure);
        }
        break;
      case "permission_requested":
        if (event.payload.permission) {
          this.state.pendingPermissions.push(event.payload.permission);
        }
        break;
      case "permission_resolved":
        if (event.payload.permission) {
          this.resolvePendingPermission(event.payload.permission.request_id);
        }
        break;
      default:
        break;
    }

    if (
      event.kind === "permission_requested" ||
      event.kind === "session_state" ||
      event.kind === "assistant_message" ||
      event.kind === "failure" ||
      event.kind === "session_closed"
    ) {
      this.signalDecision();
    }
  }

  currentState(): SessionStateSnapshot | null {
    return this.state.currentState;
  }

  transcript(): TranscriptTurn[] {
    return this.state.transcript;
  }

  decisionVersion(): number {
    return this.letDecisionSignals;
  }

  waitForDecision(afterVersion: number): Promise<void> {
    if (this.letDecisionSignals > afterVersion) {
      return Promise.resolve();
    }
    return new Promise((resolve) => {
      this.notifyDecision = resolve;
    });
  }

  currentPrompt(): string {
    const pending = this.state.pendingPermissions[0];
    if (pending) {
      return `${pending.prompt} [y/N] `;
    }
    return "klaude> ";
  }

  takePendingPermission(): PermissionEventPayload | undefined {
    return this.state.pendingPermissions.shift();
  }

  private signalDecision(): void {
    this.letDecisionSignals += 1;
    const waiter = this.notifyDecision;
    this.notifyDecision = null;
    waiter?.();
  }

  private resolvePendingPermission(requestID: string): void {
    const index = this.state.pendingPermissions.findIndex((permission) =>
      permission.request_id === requestID
    );
    if (index >= 0) {
      this.state.pendingPermissions.splice(index, 1);
    }
  }

  private setTurnUserMessage(turnID: string, content: string): void {
    const turn = this.ensureTurn(turnID);
    turn.userMessage = content;
  }

  private appendAssistantStream(turnID: string, content: string): void {
    if (content === "") {
      return;
    }
    const turn = this.ensureTurn(turnID);
    turn.assistantStream += content;
  }

  private setAssistantFinal(turnID: string, content: string): void {
    const turn = this.ensureTurn(turnID);
    turn.assistantFinal = content;
  }

  private appendWarning(turnID: string, warning: string): void {
    const turn = this.ensureTurn(turnID);
    turn.warnings.push(warning);
  }

  private setFailure(turnID: string, failure: FailurePayload): void {
    const turn = this.ensureTurn(turnID);
    turn.failure = failure;
  }

  private upsertToolActivity(turnID: string, tool: ToolEventPayload): void {
    const turn = this.ensureTurn(turnID);
    if (turn.toolCalls.some((entry) => entry.callId === tool.call_id)) {
      return;
    }
    turn.toolCalls.push({
      callId: tool.call_id,
      name: tool.name,
      input: tool.input,
      concurrencyClass: tool.concurrency_class,
      progressMessages: tool.progress_message === "" ? [] : [tool.progress_message],
      resultSummary: tool.result_summary,
      output: tool.output,
      failed: tool.failed,
    });
  }

  private mergeToolActivity(turnID: string, tool: ToolEventPayload): void {
    const turn = this.ensureTurn(turnID);
    const existing = turn.toolCalls.find((entry) => entry.callId === tool.call_id);
    if (!existing) {
      this.upsertToolActivity(turnID, tool);
      return;
    }
    if (
      tool.progress_message !== "" &&
      existing.progressMessages[existing.progressMessages.length - 1] !== tool.progress_message
    ) {
      existing.progressMessages.push(tool.progress_message);
    }
    if (tool.result_summary !== "") {
      existing.resultSummary = tool.result_summary;
    }
    if (tool.output !== "") {
      existing.output = tool.output;
    }
    existing.failed = tool.failed;
  }

  private ensureTurn(turnID: string): TranscriptTurn {
    const normalizedTurnID = turnID === "" ? "system" : turnID;
    const existingIndex = this.transcriptIndex.get(normalizedTurnID);
    if (existingIndex !== undefined) {
      return this.state.transcript[existingIndex]!;
    }
    const turn: TranscriptTurn = {
      turnId: normalizedTurnID,
      userMessage: "",
      assistantStream: "",
      assistantFinal: "",
      toolCalls: [],
      warnings: [],
      failure: null,
    };
    this.state.transcript.push(turn);
    this.transcriptIndex.set(normalizedTurnID, this.state.transcript.length - 1);
    return turn;
  }
}
