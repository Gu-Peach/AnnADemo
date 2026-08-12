import { SUMMARIZER_METHOD, SUMMARIZER_TOOL_ID } from '../constants';
import type { AnnaRuntime, Note, SummarizeResult } from '../types';

export class SummarizeService {
  constructor(private readonly anna: AnnaRuntime) {}

  async summarize(notes: Note[]): Promise<SummarizeResult> {
    const response = await this.anna.tools.invoke({
      tool_id: SUMMARIZER_TOOL_ID,
      method: SUMMARIZER_METHOD,
      args: {
        notes: notes.map((note) => ({
          id: note.id,
          order: note.order,
          content: note.content,
          createdAt: note.createdAt,
        })),
        max_words: 80,
      },
      timeoutMs: 90_000,
    });

    const invokeResult = extractInvokeResult(response);
    if (!invokeResult.success) {
      throw new Error(invokeResult.error || 'Summarize failed.');
    }
    const data = invokeResult.data as Partial<SummarizeResult> | undefined;
    if (!data || typeof data.summary !== 'string') {
      throw new Error('Summarizer did not return a summary.');
    }
    return {
      summary: data.summary,
      model: data.model,
      usage: data.usage,
      stopReason: data.stopReason,
    };
  }
}

function extractInvokeResult(response: unknown): { success: boolean; data?: unknown; error?: string } {
  const maybeWrapped = response as { result?: unknown };
  const payload = maybeWrapped && 'result' in maybeWrapped ? maybeWrapped.result : response;
  if (!payload || typeof payload !== 'object') {
    throw new Error('Invalid tools.invoke response.');
  }
  const result = payload as { success?: unknown; data?: unknown; error?: unknown };
  return {
    success: result.success === true,
    data: result.data,
    error: typeof result.error === 'string' ? result.error : undefined,
  };
}
