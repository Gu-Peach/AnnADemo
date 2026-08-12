export interface Note {
  id: string;
  order: number;
  content: string;
  createdAt: string;
}

export interface AnnaRuntime {
  storage: {
    get(input: { key: string }): Promise<{ value?: unknown; exists?: boolean }>;
    set(input: { key: string; value: unknown }): Promise<unknown>;
  };
  tools: {
    invoke(input: { tool_id: string; method?: string; args: unknown; timeoutMs?: number }): Promise<{ result?: unknown } | unknown>;
  };
  window?: {
    set_title?(input: { title: string }): Promise<unknown>;
  };
}

export interface SummarizeResult {
  summary: string;
  model?: string;
  usage?: unknown;
  stopReason?: string;
}
