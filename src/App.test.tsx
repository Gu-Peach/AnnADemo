import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { App } from './App';
import { SUMMARIZER_METHOD, SUMMARIZER_TOOL_ID } from './constants';
import type { AnnaRuntime, Note } from './types';

function makeAnna(initialNotes: Note[] = [], invokeResult: unknown = { success: true, data: { summary: 'Mock summary', model: 'mock' } }) {
  let stored = initialNotes;
  const anna: AnnaRuntime = {
    storage: {
      get: vi.fn(async () => ({ exists: stored.length > 0, value: stored })),
      set: vi.fn(async ({ value }) => {
        stored = value as Note[];
        return { ok: true };
      }),
    },
    tools: {
      invoke: vi.fn(async () => ({ result: invokeResult })),
    },
    window: { set_title: vi.fn(async () => ({})) },
  };
  return { anna, getStored: () => stored };
}

describe('App', () => {
  it('does not save empty input', async () => {
    const { anna } = makeAnna();
    render(<App anna={anna} />);
    await screen.findByText('No notes yet.');
    await userEvent.click(screen.getByRole('button', { name: /save note/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Please enter a note');
    expect(anna.storage.set).not.toHaveBeenCalled();
  });

  it('saves a note through anna.storage.set and clears input', async () => {
    const { anna, getStored } = makeAnna();
    render(<App anna={anna} />);
    const input = await screen.findByLabelText(/new note/i);
    await userEvent.type(input, 'Fix login bug');
    await userEvent.click(screen.getByRole('button', { name: /save note/i }));
    await screen.findByText('Fix login bug');
    expect(anna.storage.set).toHaveBeenCalledTimes(1);
    expect(getStored()[0]).toMatchObject({ order: 1, content: 'Fix login bug' });
    expect(input).toHaveValue('');
  });

  it('loads and deletes notes through storage', async () => {
    const note = { id: 'n1', order: 1, content: 'Workshop ideas', createdAt: new Date('2026-08-12T00:00:00Z').toISOString() };
    const { anna, getStored } = makeAnna([note]);
    render(<App anna={anna} />);
    await screen.findByText('Workshop ideas');
    await userEvent.click(screen.getByRole('button', { name: /delete note 1/i }));
    await waitFor(() => expect(getStored()).toEqual([]));
    expect(anna.storage.set).toHaveBeenCalledWith({ key: 'mini-notes:v1:notes', value: [] });
  });

  it('accepts harness storage.get responses without exists flag', async () => {
    const note = { id: 'n1', order: 1, content: 'Existing note', createdAt: new Date('2026-08-12T00:00:00Z').toISOString() };
    const { anna } = makeAnna([note]);
    vi.mocked(anna.storage.get).mockResolvedValueOnce({ value: [note] });
    render(<App anna={anna} />);
    await screen.findByText('Existing note');
  });

  it('summarizes by calling anna.tools.invoke with notes', async () => {
    const note = { id: 'n1', order: 1, content: 'Follow up with customer', createdAt: new Date().toISOString() };
    const { anna } = makeAnna([note]);
    render(<App anna={anna} />);
    await screen.findByText('Follow up with customer');
    await userEvent.click(screen.getByRole('button', { name: /summarize/i }));
    await screen.findByText('Mock summary');
    expect(anna.tools.invoke).toHaveBeenCalledWith({
      tool_id: SUMMARIZER_TOOL_ID,
      method: SUMMARIZER_METHOD,
      args: { notes: [note], max_words: 80 },
      timeoutMs: 90000,
    });
  });

  it('shows tool errors', async () => {
    const note = { id: 'n1', order: 1, content: 'Fix bug', createdAt: new Date().toISOString() };
    const { anna } = makeAnna([note], { success: false, error: '[-32603] harness started with --no-llm' });
    render(<App anna={anna} />);
    await screen.findByText('Fix bug');
    await userEvent.click(screen.getByRole('button', { name: /summarize/i }));
    expect(await screen.findByRole('alert')).toHaveTextContent('harness started with --no-llm');
  });
});
